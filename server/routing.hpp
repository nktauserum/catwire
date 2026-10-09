#pragma once

#include <cstdlib>
#include <cstring>
#include <vector>
#include <array>
#include <atomic>
#include <mutex>
#include <shared_mutex>

#include <sodium.h>

#include "../common/types.h"
#include "../common/macro.h"
#include "models/client.h"

#define MAX_CLIENTS 256

struct Session {
    std::atomic<bool> is_active = false;
    std::atomic<u64>  counter   = 0;

    std::shared_mutex mtx;

    u8 rx_key   [32];
    u8 tx_key   [32];
    u8 publicKey[32];

    u32 local_addr = 0;
    Address remote_addr;
};

class RoutingTable {
private:
    u8 privateKey[32];
public:
    std::array<Session, MAX_CLIENTS> table;
    std::array<int, MAX_CLIENTS> lookup;
    u32 num_clients = 0;

    u8 publicKey[32];

    int exists(u8* publicKey) {
        for (u32 i = 0; i < num_clients; ++i) {
            if(memcmp(table[i].publicKey, publicKey, 32) == 0) return i;
        }

        return -1;
    }

    int active(u32 idx) {
        int session_index = lookup[idx];
        if (session_index < 0) return -1;

        return table[session_index].is_active.load(std::memory_order_acquire) ? session_index : -1;
    }

    int Handshake(u8* client_pubkey, Address addr) {
        i32 sessionIndex = exists(client_pubkey);
        if (sessionIndex < 0) {
            puts("No such session index");
            return -1;
        }

        auto session = &table[sessionIndex];
        {
            std::unique_lock<std::shared_mutex> lock(session->mtx);

            if (crypto_kx_server_session_keys(
                session->rx_key, session->tx_key,
                publicKey, privateKey,
                client_pubkey
            ) < 0) return -1;

            session->remote_addr = addr;
        }

        session->is_active.store(1, std::memory_order_release);
        return sessionIndex;
    } 

    RoutingTable(u8* seed, std::vector<Client>& clients) {
        if (clients.size() > MAX_CLIENTS)
            panic("config: too many clients. Check MAX_CLIENTS constant.");

        lookup.fill(-1);

        for (num_clients = 0; num_clients < clients.size(); ++num_clients) {
            auto& client = clients[num_clients];
            Session& session = table[num_clients];

            int idx = ((client.local_addr >> 24) & 0xFF) - 2;
            if (idx < 0) panic("config: addresses ...0 and ...1 are reserved. Check client addresses.");
            
            memcpy(session.publicKey, client.publicKey, 32);
            session.local_addr = client.local_addr;
           
            if (lookup[idx] != -1) {
                std::cout << "[WARNING]: the client #" << num_clients+1 << " have dublicated address, overwriting..." << std::endl;
            }

            lookup[idx] = num_clients;
        }

        if (crypto_kx_seed_keypair(publicKey, privateKey, seed) != 0) {
            panic("config: main: check provided private keys again");
        }

    }
};
