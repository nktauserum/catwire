#pragma once

#include "../types.h"

enum PacketTypes : u8 {
   DATA,
   HANDSHAKE,
};

typedef struct {
    Address addr;
    char* payload;
    size_t size;
} UDPPacket;

typedef struct {
    u8  packetType;
    u64 peerIndex;
    u64 counter;
} __attribute__((packed)) Header;

typedef struct {
    Header  header;
    u8      payload[65535+16];
} __attribute__((packed)) Packet;

typedef struct {
    u64     idx;
    Address addr;
    u64     len;
    Packet  packet;
} IncomingBuffer;

typedef struct {
    long long unsigned int len; // strange :)
    u64                    idx;
    u8                     payload[65535];
} OutgoingBuffer;
