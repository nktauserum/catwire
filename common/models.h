#pragma once

#include "types.h"

enum PacketTypes : u8 {
   DATA,
   HANDSHAKE,
};

typedef struct {
    u8  packetType;
    u32 peerIndex;
    u64 counter;
    union { // union here will be important, trust me
        u8 aegis256_nonce[32];
    };
} __attribute__((packed)) Header;

typedef struct {
    Header  header;
    u8      payload[sizeof(Header)+1500+16]; // - sizeof(Header), maybe?
} __attribute__((packed)) Packet;

typedef struct {
    u64                    idx;
    Address                addr;
    long long unsigned int len;
    Packet                 packet;
} IncomingBuffer;

typedef struct {
    long long unsigned int len; // strange :)
    u64                    idx;
    u8                     payload[1500];
} OutgoingBuffer;
