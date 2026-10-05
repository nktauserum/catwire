#pragma once 

#include "../../common/types.h"

struct Client {
    u8  publicKey[64] = {0};
    u32 local_addr = 0;
};
