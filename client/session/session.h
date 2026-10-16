#pragma once

#include <stdlib.h>
#include <stdbool.h>

#include "../../common/types.h"

typedef struct {
    u32 available;

    u8 rx_key[32];
    u8 tx_key[32];

    u8 privateKey[32];
    u8 publicKey [32];

    Address remoteAddr;
    u32     peerIndex;
    u64     counter;
} Session;


bool session_available(Session*);
void session_set_available(Session*);
