#pragma once

#include <vector>
#include <thread>

#include "../../common/types.h"
#include "../models/client.h"

struct Config {
    u8                  seed[32] = {0};
    u16                 port;
    u32                 num_cores = std::thread::hardware_concurrency();
    std::vector<Client> clients;

    Config(const char* filename);
};

