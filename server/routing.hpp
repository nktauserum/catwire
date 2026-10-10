#pragma once

#include <cstdlib>
#include <cstring>
#include <vector>
#include <array>
#include <atomic>

#include "../common/types.h"
#include "../common/pool.h"

#include "models/client.h"

#define MAX_CLIENTS 64 // due to shared pool limit. maybe i will consider changing it

struct Session {
    std::atomic<u64> counter = 0;

    bool is_active = false;

    u8 rx_key   [32];
    u8 tx_key   [32];
    u8 publicKey[32];

    u32     local_addr;
    Address remote_addr;
};

class RoutingTable {
private:
    u8 privateKey[32];

    int exists(u8* publicKey);
public:
    u8 publicKey [32];

    SharedPool<Session> session_pool;
    std::array<std::atomic<int>, MAX_CLIENTS> lookup;
    u32 num_clients = 0;

    int Handshake(u8* client_pubkey, Address addr);
    RoutingTable(u8* seed, std::vector<Client>& clients);
};
