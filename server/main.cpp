#include <sodium.h>
#include <iostream>

#include "../common/macro.h"

#include "utils/config.hpp"
#include "interface.hpp"

int main(void) {
    std::cout << "[INFO]: Starting..." << std::endl;

    if (sodium_init() < 0) {
        std::cout << "[ERROR]: Failed to initialize libsodium" << std::endl;
        return 1;
    }

    if (!crypto_aead_aes256gcm_is_available()) { 
        std::cout << "[ERROR]: AES256-GCM is not supported by your hardware (CPU)" << std::endl;
        return 1;
    }

    Config config("config.ini");

    Tunnel    tun;
    Transport udp;

    Context ctx(config);

    tun.Init(ctx, config, &udp);
    udp.Init(ctx, config, &tun);

    tun.Join();

    return 0;
}
