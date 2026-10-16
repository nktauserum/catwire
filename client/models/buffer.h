#pragma once

#include "../../common/types.h"

typedef enum : int {
    Receive,
    Transmit,
} Origin;

typedef struct {
    size_t  len;
    Origin  origin;
    Address addr;
    u8      payload[2048];
} Buffer;
