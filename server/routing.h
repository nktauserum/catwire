#include <cstdlib>
#include <cstring>
#include <vector>

#include "../common/types.h"
#include "../common/models.h"

class RoutingTable {
public:
    std::vector<Session> table = {};

    inline i32 exists(u8* publicKey) {
        for (u64 i = 0; i < table.size(); ++i) {
            if(memcmp(table[i].publicKey, publicKey, 32) == 0) return static_cast<i32>(i);
        }

        return -1;
    }

    inline u64 addCounter(i32 idx) {
        return reinterpret_cast<std::atomic<u64>*>(&table[idx].counter)->fetch_add(1);
    }

    static RoutingTable init_from_vec(std::vector<Session> clients) {
        RoutingTable t;
        t.table = std::move(clients);
        return t;
    }
};
