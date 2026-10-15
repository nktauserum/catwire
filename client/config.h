#pragma once

#include "../common/types.h"

typedef struct {
    char seed[35];
    u32 server_addr;
    u16 server_port;
} Config;

int config_parse(Config*, const char*);
