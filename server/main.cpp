#include <sodium.h>
#include <iostream>

#include "../common/macro.h"

#include "utils/config.hpp"
#include "interface.hpp"

int main(void) {
    if (sodium_init() < 0) 
        panic("failed to initialize libsodium");

    if (!crypto_aead_aes256gcm_is_available()) 
        panic("AES256-GCM is not supported by your hardware (CPU)");

    Config config("config.ini");

    Tunnel    tun;
    Transport udp;

    Context ctx(config);

    tun.Init(ctx, config, &udp);
    udp.Init(ctx, config, &tun);

    std::cout << "[INFO]: Starting on port " << config.port << std::endl;

    tun.Join();

    return 0;
}
