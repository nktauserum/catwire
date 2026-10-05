#pragma once

#include <cstdlib>
#include <cstring>
#include <vector>
#include <atomic>
#include <mutex>
#include <shared_mutex>

#include <sodium.h>

#include "../common/types.h"
#include "../common/models.h"
#include "models/client.h"

#define MAX_CLIENTS 256

struct Session {
    std::shared_mutex mtx;
    std::atomic<bool> is_active = false;

    u8 shared_key[32] = {0};
    u8 publicKey[32] = {0};

    crypto_aead_aes256gcm_state crypto_ctx;
    u64 counter = 0;

    u32 local_addr = 0;
    Address remote_addr;

    __always_inline u64 add_counter() {
        return std::atomic_ref<u64>(counter).fetch_add(1, std::memory_order_relaxed);
    }
};

class RoutingTable {
private:
    u8 privateKey[32];

public:
    Session table[MAX_CLIENTS];
    int available[MAX_CLIENTS] = {0};
    int size = 0;

    u8 publicKey[32];


    int exists(u8* publicKey) {
        for (int i = 0; i < size; ++i) {
            int idx = available[i];
            if(memcmp(table[idx].publicKey, publicKey, 32) == 0) return idx;
        }

        return -1;
    }

    int Handshake(u8* client_pubkey, Address addr) {
        i32 sessionIndex = exists(client_pubkey);
        if (sessionIndex < 0) {
            puts("No such session index");
            return -1;
        }

        auto session = &table[sessionIndex];

        u8 raw_secret [32]                             = {0};
        u8 hash_args  [32*3]                           = {0};

        if (crypto_scalarmult(raw_secret, privateKey, client_pubkey) != 0) return -1;

        memcpy(hash_args,    raw_secret,          32);
        sodium_memzero(raw_secret, 32);
        memcpy(hash_args+32, publicKey, 32);
        memcpy(hash_args+64, client_pubkey,           32);
        
        {
            std::unique_lock<std::shared_mutex> lock(session->mtx);

            int ret = crypto_generichash(
                session->shared_key, 32, 
                hash_args,           sizeof(hash_args),
                nullptr,             0
            );
            if (ret < 0) return -1;

            ret = crypto_aead_aes256gcm_beforenm(&session->crypto_ctx, session->shared_key);
            if (ret < 0) return -1;

            session->remote_addr = addr;
        }

        sodium_memzero(hash_args, 32*3);

        session->is_active.store(1, std::memory_order_release);
        
        return sessionIndex;
    } 

    RoutingTable(u8* seed, std::vector<Client>& clients) {
        for (size = 0; size < clients.size(); ++size) {
            auto& client = clients[size];
            int idx = reinterpret_cast<u8*>(&client.local_addr)[3]-2;
            if (idx < 0) continue; // actually, we should panic: addresses ...0 and ...1 are reserved
            
            memcpy(table[idx].publicKey, client.publicKey, 32);
            table[idx].local_addr = client.local_addr;

            available[size] = idx;
        }
    }
};
