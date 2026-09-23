#include <thread>
#include <iostream>
#include <string>

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>
#include <boost/asio.hpp>
#include <sodium.h>
using boost::asio::ip::udp;

#include "../common/types.h"
#include "../common/models.h"
#include "config.hpp"

class Client {
private:
    boost::asio::io_context ctx;
    std::thread run_ctx;
    boost::asio::executor_work_guard<boost::asio::io_context::executor_type> guard;

    udp::socket   socket;
    udp::endpoint endpoint;
    
    SharedPool<IncomingBuffer> pool;

    std::atomic<u64> peerIndex = 0;
    std::atomic<u64> counter   = 0;
    u8 session_key[crypto_aead_aes256gcm_KEYBYTES] = {0};


    u8 publicKey[crypto_kx_PUBLICKEYBYTES] = {0};
    u8 privateKey[crypto_kx_SECRETKEYBYTES] = {0};
    
public:
    Client(const char* server_addr, u16 server_port, u8* key) : ctx{}, guard{boost::asio::make_work_guard(ctx)}, socket{udp::socket(ctx, udp::endpoint(udp::v4(), 0))} {
        udp::resolver resolver(ctx);
        udp::resolver::results_type endpoints = resolver.resolve(udp::v4(), server_addr, std::to_string(server_port));
        endpoint = *endpoints.begin();

        run_ctx = std::thread([this](){
            ctx.run();
        });

        if (sodium_init() < 0) 
            throw std::runtime_error("panic: failed to initialize libsodium");

        if (!crypto_aead_aes256gcm_is_available()) 
            throw std::runtime_error("panic: AES256-GCM is not supported by your hardware (CPU)");

        memcpy(&privateKey, key, 32);
        if (crypto_kx_keypair(publicKey, privateKey) != 0) {
            throw std::runtime_error("panic: check provided private key again");
        }
    }

    // TODO: add an eternal loop with availability check
    void Handshake() {
        u32 idx = pool.Acquire();
        IncomingBuffer* buf = &pool.data[idx];

        buf->len    = 32;
        buf->packet = Packet {
            .header  = Header {
                .packetType = HANDSHAKE,
                .peerIndex  = peerIndex.load(),
                .counter    = counter.fetch_add(1),
            },
            .payload = {0},
        };

        memcpy(&buf->packet.payload, publicKey, 32);

        socket.async_send_to(
            boost::asio::buffer(&buf->packet, buf->len+sizeof(Header)), 
            endpoint, 
        [this, idx](boost::system::error_code e, std::size_t sent_len)
        {
            std::cout << "Sent packet " << sent_len << " bytes" << std::endl;
            pool.Release(idx);
        });
    }

    void Incoming() {
        u32 idx = pool.Acquire();
        IncomingBuffer* buf = &pool.data[idx];

        socket.async_receive_from(
            boost::asio::buffer(&buf->packet, sizeof(buf->packet)),
            endpoint, // TODO: fix endpoint replacement
        [this, idx](boost::system::error_code e, std::size_t len) {
            if (e.value() != 0) {
                std::cout << "Incoming() failed: " << e.message() << std::endl;
            } else {
                IncomingBuffer* buf = &pool.data[idx];

                switch (buf->packet.header.packetType) {
                case DATA:
                    break;

                case HANDSHAKE: {
                    if (len != 32 + sizeof(Header)) goto cleanup;

                    u8 raw_secret [32]   = {0};
                    u8 hash_args  [32*3] = {0};

                    if (crypto_scalarmult(raw_secret, privateKey, buf->packet.payload) != 0) goto cleanup;

                    memcpy(hash_args,    raw_secret,          32);
                    sodium_memzero(raw_secret,                32);
                    memcpy(hash_args+32, buf->packet.payload, 32);
                    memcpy(hash_args+64, publicKey,           32);

                    int ret = crypto_generichash(
                        session_key, sizeof(session_key), 
                        hash_args,   sizeof(hash_args),
                        nullptr, 0
                    );

                    sodium_memzero(hash_args, 32*3);
                    if (ret < 0) goto cleanup; 

                    puts("The shared secret was computed!");

                    break;
                }

                default:
                    goto cleanup;
                    break;
                }
            }

        cleanup:
            pool.Release(idx);
            Incoming();
        });
    }

    inline void Wait() {
        return run_ctx.join();
    }
};

int main(void) {
    Config config = Config::load_from_file("config.ini");
    Client client(config.server_addr, config.server_port, config.privateKey);

    std::thread handshake([&client](){
        client.Handshake();
    });


    client.Incoming();
    client.Wait();

    handshake.join();

    return 0;
}
