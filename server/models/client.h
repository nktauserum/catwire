#pragma once 

#include "../../common/types.h"

struct Client {
    u8  publicKey[32] = {0};
    u32 local_addr = 0;
};
