#include <string.h>
#include <stdio.h>
#include <thread>
#include <shared_mutex>
#include <mutex>

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>
#include <sodium.h>

#include "../common/types.h"

#include "incoming.hpp"
#include "outgoing.hpp"
#include "config.hpp"
#include "routing.hpp"

#define WORKERS_COUNT 8

class Application {
private:
    RoutingTable routingTable;

    u8 publicKey [crypto_kx_PUBLICKEYBYTES] = {0};
    u8 privateKey[crypto_kx_SECRETKEYBYTES] = {0};

    Channel<u32> incomingChannel;
    Channel<u32> outgoingChannel;

    SharedPool<IncomingBuffer> incomingPool;
    SharedPool<OutgoingBuffer> outgoingPool;

    Incoming     incoming;
    Outgoing     outgoing;

public: 
    void incomingWorker() {
        auto queue = incomingChannel.add_worker();

        while (true) {
            u32 idx = *queue->read();
            queue->pop();

            IncomingBuffer* buf = &incomingPool.data[idx];

            switch (buf->packet.header.packetType) {
            case DATA: {
                auto peer_idx = buf->packet.header.peerIndex;
                if (peer_idx >= MAX_CLIENTS) goto cleanup;

                auto session = &routingTable.table[peer_idx];
                break;
            }

            case HANDSHAKE: {
                if (buf->len != 32) goto cleanup;
                i32 sessionIndex = routingTable.exists(buf->packet.payload);
                if (sessionIndex < 0) {
                    puts("No such session index");
                    goto cleanup;
                }

                auto session = &routingTable.table[sessionIndex];

                u8 raw_secret [32]                             = {0};
                u8 hash_args  [32*3]                           = {0};

                if (crypto_scalarmult(raw_secret, privateKey, buf->packet.payload) != 0) goto cleanup;

                memcpy(hash_args,    raw_secret,          32);
                sodium_memzero(raw_secret, 32);
                memcpy(hash_args+32, buf->packet.payload, 32);
                memcpy(hash_args+64, publicKey,           32);
                
                {
                    std::unique_lock<std::shared_mutex> lock(session->mtx);

                    int ret = crypto_generichash(
                        session->shared_key, 32, 
                        hash_args,           sizeof(hash_args),
                        nullptr,             0
                    );

                    if (ret < 0) goto cleanup;
                }

                sodium_memzero(hash_args, 32*3);

                printf("The shared secret was computed!\n");
                fflush(stdout);

                // send the server's private key as a response
                u32 out_idx = incomingPool.Acquire();
                IncomingBuffer* out_buf = &incomingPool.data[out_idx];

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

                if (!incoming.send(out_buf)) {
                    puts("Incoming::Send() failed");
                    goto cleanup;
                }

                session->is_active.store(1);

                break;
            }

            default:
                goto cleanup;
                break;
            }

    cleanup:
            incomingPool.Release(idx);
        }
    }

    void outgoingWorker() {
        auto queue = outgoingChannel.add_worker(); 

        while (true) {
            u32 idx = *queue->read();
            queue->pop();

            OutgoingBuffer* buf = &outgoingPool.data[idx];

            // process
        }
    }

    void listen_incoming() {
        incoming.listen(&incomingPool, &incomingChannel);
    }

    void listen_outgoing() {
        outgoing.listen(&outgoingPool, &outgoingChannel);
    }

    Application(Incoming& incoming, Outgoing& outgoing, Config* config) : 
        incomingChannel{Channel<u32>(WORKERS_COUNT)}, 
        incomingPool{SharedPool<IncomingBuffer>()},
        incoming{std::move(incoming)},
        outgoingChannel{Channel<u32>(WORKERS_COUNT)},
        outgoingPool{SharedPool<OutgoingBuffer>()},
        outgoing{std::move(outgoing)},
        routingTable{RoutingTable(config->clients)}
    {
        if (sodium_init() < 0) 
            throw panic("panic: failed to initialize libsodium");

        if (!crypto_aead_aes256gcm_is_available()) 
            throw panic("panic: AES256-GCM is not supported by your hardware (CPU)");

        if (crypto_kx_seed_keypair(publicKey, privateKey, config->seed) != 0) {
            throw panic("panic: check provided private key again");
        }
    };
};

int main(void) {
    Config config = Config::load_from_file("config.ini");

    Incoming incoming;
    if (!incoming.init(config.port)) return 1;

    Outgoing outgoing;
    if (!outgoing.init("cw1")) return 1;

    Application app{incoming, outgoing, &config};

    std::thread workers[WORKERS_COUNT*2];
    for (int i = 0; i < WORKERS_COUNT; ++i) {
        workers[i] = std::thread([&app](){
            app.incomingWorker();
        });
    }
    for (int i = WORKERS_COUNT; i < WORKERS_COUNT*2; ++i) {
        workers[i] = std::thread([&app](){
            app.outgoingWorker();
        });
    }

    std::thread i([&app](){
        app.listen_incoming();
    });

    app.listen_outgoing();
    
    return 0;
}
