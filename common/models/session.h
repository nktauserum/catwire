#pragma once

#include <atomic>
#include "../types.h"

struct Session {
    u8 shared_key[32] = {0};
    u8 publicKey[32] = {0};
    u32 local_addr;
    Address remote_addr;
};

bool compareSessionsByAddr(const Session& a, const Session& b) {
    return a.local_addr < b.local_addr;
}
