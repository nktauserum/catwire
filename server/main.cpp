#include <string.h>
#include <stdio.h>
#include <thread>

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>
#include <sodium.h>

#include "../common/types.h"

#include "transport.h"
#include "config.hpp"
#include "routing.hpp"

#define WORKERS_COUNT 8

class Application {
private:
    UDP* udp;
    RoutingTable routingTable;

    u8 publicKey[crypto_kx_PUBLICKEYBYTES] = {0};
    u8 privateKey[crypto_kx_SECRETKEYBYTES] = {0};

    Channel<u32> channel;
    SharedPool<IncomingBuffer> pool;
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
                if (buf->len != 32) goto cleanup;
                i32 sessionIndex = routingTable.exists(buf->packet.payload);
                if (sessionIndex < 0) {
                    puts("No such session index");
                    goto cleanup;
                }

                u8 raw_secret [32]                             = {0};
                u8 hash_args  [32*3]                           = {0};
                u8 session_key[crypto_aead_aes256gcm_KEYBYTES] = {0};

                if (crypto_scalarmult(raw_secret, privateKey, buf->packet.payload) != 0) goto cleanup;

                memcpy(hash_args,    raw_secret,          32);
                sodium_memzero(raw_secret, 32);
                memcpy(hash_args+32, buf->packet.payload, 32);
                memcpy(hash_args+64, publicKey,           32);

                int ret = crypto_generichash(
                    session_key, sizeof(session_key), 
                    hash_args,   sizeof(hash_args),
                    nullptr, 0
                );

                sodium_memzero(hash_args, 32*3);
                if (ret < 0) goto cleanup; 
                printf("The shared secret was computed!\n");
                fflush(stdout);

                // send the server's private key as a response
                u32 out_idx = pool.Acquire();
                IncomingBuffer* out_buf = &pool.data[out_idx];

                out_buf->idx    = out_idx;
                out_buf->addr   = buf->addr;
                out_buf->len    = crypto_kx_PUBLICKEYBYTES + sizeof(Header);
                out_buf->packet = Packet {
                    .header  = Header {
                        .packetType = HANDSHAKE,
                        .peerIndex  = routingTable.table[sessionIndex].local_addr,
                        .counter    = routingTable.addCounter(sessionIndex),
                    },
                    .payload = {0},
                };
                memcpy(&out_buf->packet.payload, publicKey, crypto_kx_PUBLICKEYBYTES);

                // if (!udp->Send(out_buf)) {
                //     puts("UDP::Send() failed");
                //     goto cleanup;
                // }

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

    inline void listen_incoming() {
        udp->listen(&pool, &channel);
    }

    Application(UDP* udp, Config* config) : udp{udp}, channel{Channel<u32>(WORKERS_COUNT)}, pool{SharedPool<IncomingBuffer>()} {
        if (sodium_init() < 0) 
            throw panic("panic: failed to initialize libsodium");

        if (!crypto_aead_aes256gcm_is_available()) 
            throw panic("panic: AES256-GCM is not supported by your hardware (CPU)");

        if (crypto_kx_seed_keypair(publicKey, privateKey, config->seed) != 0) {
            throw panic("panic: check provided private key again");
        }

        routingTable = RoutingTable::init_from_vec(config->clients);
    };
};

int main(void) {
    auto config = Config::load_from_file("config.ini");

    UDP udp_listener;
    if (!udp_listener.init(config.port)) return 1;

    Application app{&udp_listener, &config};

    std::vector<std::thread> workers(WORKERS_COUNT);
    for (int i = 0; i < WORKERS_COUNT; ++i) {
        workers[i] = std::thread([&app](){
            app.worker();
        });
    }

    // std::thread incoming([&app, &udp_listener](){
        app.listen_incoming();
    // });


    // incoming.join();

    return 0;
}
