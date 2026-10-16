#pragma once

#include "../../common/types.h"

typedef enum : int {
    Receive,
    Transmit,
} Origin;

typedef struct {
    u64     idx;
    size_t  len;
    Origin  origin;
    Address addr;
    u8      payload[2048];
} Buffer;
