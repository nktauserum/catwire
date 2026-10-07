#pragma once

#include "../../common/types.h"

typedef struct {
    u8  packetType;
    u64 peerIndex;
    u64 counter;
} __attribute__((packed)) Header;

typedef struct {
    Header  header;
    u8      payload[1420+16];
} __attribute__((packed)) Packet;

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
