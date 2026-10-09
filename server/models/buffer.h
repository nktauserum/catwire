#pragma once

#include "../../common/types.h"
#include "../../common/models.h"

typedef struct {
    u64     idx;
    u64     payload_len;
    struct {
        struct msghdr hdr;
        struct iovec  vec;
    };                 
    Address addr; 
    Packet  packet;
} TransportBuffer;

typedef struct {
    u64 idx;
    u64 counter;
    u64 len;
    u8  payload[1420];
} TunnelBuffer;
