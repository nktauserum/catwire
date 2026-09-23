#pragma once

#include <atomic>
#include "../types.h"

struct Session {
    u8 shared_key[32] = {0};
    u8 publicKey[32] = {0};
    Address remote_addr;

    std::atomic<u32> available;
};
