#include <string.h>
#include <stdio.h>
#include <thread>
#include <stdexcept>

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>
#include <sodium.h>

#include "types.h"
#include "memory.h"
#include "networking.h"

#define WORKERS_COUNT 8

struct Session {
private:
    Address remote_addr;
    u8 secret[crypto_scalarmult_BYTES];
    u8 publicKey[crypto_kx_PUBLICKEYBYTES];

public:
    Session(u8* s, u8* pubkey) {
        memcpy(&secret[0], s, crypto_kx_PUBLICKEYBYTES);
        memcpy(&publicKey[0], pubkey, crypto_scalarmult_BYTES);
    }
};

class Application : public Handler {
private:
    UDP* udp;

    u8 publicKey[crypto_kx_PUBLICKEYBYTES] = {0};
    u8 privateKey[crypto_kx_SECRETKEYBYTES] = {0};

    Channel<u32, WORKERS_COUNT> channel;
    SharedPool<IncomingBuffer> pool;

    u64 incoming_counter = 0;
public: 
    inline void worker() {
        auto queue = channel.add_worker();

        while (true) {
            u32 idx = *queue->read();
            queue->pop();

            IncomingBuffer* buf = &pool.data[idx];

            switch (buf->packet.header.packetType) {
            case DATA:
                break;

            case HANDSHAKE: {
                u8 secret[32] = {0};
                if (crypto_scalarmult(secret, privateKey, buf->packet.payload) != 0) 
                    goto cleanup;

                printf("The shared secret was computed!\n");
                fflush(stdout);

                break;
            }

            default:
                goto cleanup;
                break;
            }

    cleanup:
            pool.Release(idx);
        }
    }

    inline void incomingRecvCallback(UDPPacket packet) override {
        int idx = pool.Acquire();
        IncomingBuffer* buffer = &pool.data[idx];

        memcpy(&buffer->packet, packet.payload, packet.size); // but if the incoming packet was greater than 65535+17?
        buffer->addr = packet.addr;
        buffer->idx = incoming_counter++;

        channel.push(idx);

        printf("Incoming packet: payload %lu bytes, idx %lu, buf idx %d\n", packet.size, incoming_counter - 1, idx);
        fflush(stdout);
    }

    inline void incomingSendCallback(u32 idx) override {
        pool.Release(idx);
    }

    Application(UDP* udp, const char* key) : channel{Channel<u32, WORKERS_COUNT>()}, pool{SharedPool<IncomingBuffer>()}, udp{udp} {
        if (sodium_init() < 0) 
            throw std::runtime_error("panic: failed to initialize libsodium");

        if (!crypto_aead_aes256gcm_is_available()) 
            throw std::runtime_error("panic: AES256-GCM is not supported by your hardware (CPU)");

        char privateKeyBytes[32];
        size_t n = boost::beast::detail::base64::decode(&privateKeyBytes, key, strlen(key)).first;

        if (crypto_kx_keypair(publicKey, privateKey) != 0) {
            throw std::runtime_error("panic: check provided private key again");
        }
    };
};

int main(void) {
    UDP udp_listener; 
    bool ok = udp_listener.Setup();
    if (!ok) 
        return 1;

    Application app{&udp_listener, "zb1NPTbALjQmO/aWqF2YUnRJC1igyulIsk6zQK5nhEE="};

    std::thread workers[WORKERS_COUNT];
    for (int i = 0; i < WORKERS_COUNT; ++i) {
        workers[i] = std::thread([&app](){
            app.worker();
        });
    }

    std::thread incoming([&app, &udp_listener](){
        udp_listener.Listen(&app);
    });


    incoming.join();

    return 0;
}
