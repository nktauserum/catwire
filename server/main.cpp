#include <sodium.h>
#include <iostream>
#include <vector>
#include <thread>

#include "utils/config.hpp"
#include "worker.hpp"

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
    RoutingTable rtable(config.seed, config.clients);

    std::vector<std::unique_ptr<Worker>> workers;
    std::vector<std::thread> threads;
    workers.reserve(config.num_cores);
    threads.reserve(config.num_cores);

    for (u32 i = 0; i < config.num_cores; ++i) {
        workers.emplace_back(std::make_unique<Worker>(&rtable, config));

        auto w = workers.back().get();
        threads.emplace_back([w]() {
            w->Run();
        });
    }

    threads[0].join();

    return 0;
}
