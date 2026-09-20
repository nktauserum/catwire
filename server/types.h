#pragma once

#include <stdint.h>

typedef uint8_t u8;
typedef int8_t i8;
typedef uint16_t u16;
typedef int16_t i16;
typedef uint32_t u32;
typedef int32_t i32;
typedef uint64_t u64;
typedef int64_t i64;

typedef struct {
    u8  PacketType;
    u64 PeerIndex;
    u64 Counter;
} __attribute__((packed)) Header;

typedef struct {
    Header  Header;
    u8      Payload[65535];
} __attribute__((packed)) Packet;
