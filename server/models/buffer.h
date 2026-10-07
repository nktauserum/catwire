#pragma once

#include "../../common/types.h"
#include "../../common/models.h"

typedef struct {
    u64     idx;
    u64     len;
    union {
        struct {
            struct msghdr hdr;
            struct iovec  vec;
        };                      // send
        Address addr;           // receive
    };
    Packet  packet;
} TransportBuffer;

typedef struct {
    u64 len;
    u64 idx;
    u8  payload[1420];
} TunnelBuffer;
