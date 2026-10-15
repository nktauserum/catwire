#pragma once

#include "../common/types.h"

typedef struct {
    char seed[36];
    u32 server_addr;
    u16 server_port;
} Config;

int config_parse(Config*, const char*);
