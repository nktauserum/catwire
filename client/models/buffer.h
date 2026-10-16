#pragma once

#include "../../common/types.h"

typedef struct {
    u64     idx;
    size_t  len;
    Address addr;
    u8      payload[2048];
} Buffer;
