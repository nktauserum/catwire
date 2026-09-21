#pragma once

#include <stdint.h>
#include <netinet/in.h>

#define likely(x)    __builtin_expect(!!(x), 1)
#define unlikely(x)  __builtin_expect(!!(x), 0)

typedef uint8_t u8;
typedef int8_t i8;
typedef uint16_t u16;
typedef int16_t i16;
typedef uint32_t u32;
typedef int32_t i32;
typedef uint64_t u64;
typedef int64_t i64;

typedef struct sockaddr_in Address;

typedef struct {
    Address addr;
    const char* payload;
    size_t size;
} UDPPacket;

class Handler {
public:
    virtual ~Handler() = default;
    virtual void handleIncoming(UDPPacket) = 0; // now just for incoming
};

typedef struct {
    u8  packetType;
    u64 peerIndex;
    u64 counter;
} __attribute__((packed)) Header;

typedef struct {
    Header  header;
    u8      payload[65535];
} __attribute__((packed)) Packet;

typedef struct {
    u64     idx;
    Address incoming_addr;
    Packet  packet;
} IncomingBuffer;


