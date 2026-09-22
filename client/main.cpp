#include <thread>
#include <iostream>

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>
#include <boost/asio.hpp>
#include <sodium.h>
using boost::asio::ip::udp;

#include "../common/types.h"
#include "../common/models.h"

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

    u8 publicKey[crypto_kx_PUBLICKEYBYTES] = {0};
    u8 privateKey[crypto_kx_SECRETKEYBYTES] = {0};
    
public:
    Client(const char* server_addr, const char* server_port, const char* key) : ctx{}, guard{boost::asio::make_work_guard(ctx)}, socket{udp::socket(ctx, udp::endpoint(udp::v4(), 0))} {
        udp::resolver resolver(ctx);
        udp::resolver::results_type endpoints = resolver.resolve(udp::v4(), server_addr, server_port);
        endpoint = *endpoints.begin();
        // std::cout << endpoint << std::endl;

        run_ctx = std::thread([this](){
            ctx.run();
        });

        if (sodium_init() < 0) 
            throw std::runtime_error("panic: failed to initialize libsodium");

        if (!crypto_aead_aes256gcm_is_available()) 
            throw std::runtime_error("panic: AES256-GCM is not supported by your hardware (CPU)");

        char privateKeyBytes[32];
        boost::beast::detail::base64::decode(&privateKeyBytes, key, strlen(key));

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
            endpoint,
        [this, idx](boost::system::error_code e, std::size_t received_len) {
            std::cout << "Received packet " << received_len << " bytes" << std::endl;
            pool.Release(idx);
            Incoming();
        });
    }

    inline void Wait() {
        return run_ctx.join();
    }
};

int main(void) {
    Client client("127.0.0.1", "45230", "WIzlNXUEGlpWdLaxrEL/5xuQFvVFcjCIjwub87GWrac=");

    std::thread handshake([&client](){
        client.Handshake();
    });


    client.Incoming();

    client.Wait();
    handshake.join();

    return 0;
}
