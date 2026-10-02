#include <cstdlib>
#include <cstring>
#include <vector>
#include <atomic>
#include <shared_mutex>

#include <sodium.h>

#include "../common/types.h"
#include "../common/models.h"

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
public:
    Session table[MAX_CLIENTS];
    int available[MAX_CLIENTS] = {0};
    int size = 0;

    int exists(u8* publicKey) {
        for (int i = 0; i < size; ++i) {
            int idx = available[i];
            if(memcmp(table[idx].publicKey, publicKey, 32) == 0) return idx;
        }

        return -1;
    }

    RoutingTable(std::vector<Client>& clients) {
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
