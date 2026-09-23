#include <cstdlib>
#include <iostream>
#include <sodium.h>

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>
#include "../common/types.h"

int main(void) {
    if (sodium_init() < 0) {
        std::cerr << "Failed to initialize sodium" << std::endl;
        return 1;
    }

    u8 seed[32];
    randombytes_buf(seed, sizeof(seed));

    u8 publicKey[32];
    u8 privateKey[32];

    crypto_kx_seed_keypair(publicKey, privateKey, seed);

    char publicKeyStr[45];
    char privateKeyStr[45];
    char seedStr[45];

    boost::beast::detail::base64::encode(publicKeyStr, publicKey, 32);
    publicKeyStr[44] = '\0'; 

    boost::beast::detail::base64::encode(privateKeyStr, privateKey, 32);
    privateKeyStr[44] = '\0';
    
    boost::beast::detail::base64::encode(seedStr, seed, 32);
    privateKeyStr[44] = '\0';


    std::cout << "Seed: " << seedStr << std::endl;
    std::cout << "Public key: " << publicKeyStr << std::endl;
    std::cout << "Private key: " << privateKeyStr << std::endl;

    return 0;
}

