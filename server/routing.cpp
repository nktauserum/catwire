#include "routing.hpp"

#include <sodium.h>
#include "../common/macro.h"

int RoutingTable::exists(u8* publicKey) {
    for (u32 i = 0; i < MAX_CLIENTS; ++i) {
        int entry = lookup[i].load(std::memory_order_relaxed);
        if (entry == -1) continue;

        auto session = &session_pool.data[entry];
        if (memcmp(session->publicKey, publicKey, 32) == 0) return i;
    }

    return -1;
}

int RoutingTable::Handshake(u8* client_pubkey, Address addr) {
    int lookup_idx = exists(client_pubkey);
    if (lookup_idx < 0) {
        puts("No such session index");
        return -1;
    }

    u32  old_idx     = lookup[lookup_idx].load(std::memory_order_relaxed);
    auto old_session = &session_pool.data[old_idx];

    u32  session_idx = session_pool.Acquire();
    auto session     = &session_pool.data[session_idx];

    memcpy(session->publicKey, client_pubkey, 32);
    session->local_addr = old_session->local_addr;
    session->counter.store(old_session->counter.load());

    if (crypto_kx_server_session_keys(
        session->rx_key, session->tx_key,
        publicKey, privateKey,
        client_pubkey
    ) < 0) return -1;

    session->remote_addr = addr;
    session->is_active   = true;

    lookup[lookup_idx].store(session_idx);
    session_pool.Release(old_idx);

    return 0;
} 

RoutingTable::RoutingTable(u8* seed, std::vector<Client>& clients) {
    if (clients.size() > MAX_CLIENTS)
        panic("config: too many clients. Check MAX_CLIENTS constant.");

    for (num_clients = 0; num_clients < clients.size(); ++num_clients) {
        auto& client = clients[num_clients];

        u32 session_idx =  session_pool.Acquire();
        auto session    = &session_pool.data[session_idx];

        int idx = ((client.local_addr >> 24) & 0xFF) - 2;
        if (idx < 0) panic("config: addresses ...0 and ...1 are reserved. Check client addresses.");
        
        session->local_addr = client.local_addr;
        memcpy(session->publicKey, client.publicKey, 32);
       
        if (lookup[idx].load(std::memory_order_relaxed) != -1) {
            std::cout << "[WARNING]: the client #" << num_clients+1 << " have dublicated address, overwriting..." << std::endl;
        }

        lookup[idx].store(session_idx);
    }

    if (crypto_kx_seed_keypair(publicKey, privateKey, seed) != 0) {
        panic("config: main: check provided private keys again");
    }

}

