#include <iostream>
#include <string>

#include <thread>
#include <mutex>
#include <shared_mutex>

#ifdef __linux__
#include <fcntl.h>
#include <sys/ioctl.h>
#include <linux/if_tun.h>
#endif

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>
#include <boost/asio.hpp>
#include <sodium.h>

using boost::asio::ip::udp;

#include "../common/types.h"
#include "../common/models.h"
#include "../common/macro.h"
#include "../common/pool.h"
#include "config.hpp"

int open_tun(const char* ifname) {
#ifdef __linux__
	size_t ifname_len = strlen(ifname);
    if (ifname_len >= IFNAMSIZ) {
        return -1;
    }

    struct ifreq ifr = {0};
    ifr.ifr_flags = IFF_TUN | IFF_NO_PI;
    strncpy(ifr.ifr_name, ifname, ifname_len);

    int qfd = open("/dev/net/tun", O_RDWR | O_NONBLOCK);
    if (qfd == -1) {
        return -1;
    }

    int rc = ioctl(qfd, TUNSETIFF, &ifr);
    if (rc == -1) {
        close(qfd);
        return -1;
    }

    return qfd;

#else
#error "Only Linux is supported for now. Enter the Void: https://voidlinux.org/download"
#endif
}

class Application {
private:
    boost::asio::io_context ctx;
    std::thread run_ctx;
    boost::asio::executor_work_guard<boost::asio::io_context::executor_type> guard;

    udp::socket   socket;
    udp::endpoint endpoint;

    int tun_fd = -1;
    boost::asio::posix::stream_descriptor tun_stream;
    
    SharedPool<IncomingBuffer> incoming_pool;
    SharedPool<OutgoingBuffer> outgoing_pool;

    std::atomic<u64> peerIndex = 0;
    std::atomic<u64> counter   = 0;

    std::shared_mutex mtx;
    u8 session_key[crypto_aead_aes256gcm_KEYBYTES] = {0};
    crypto_aead_aes256gcm_state crypto_ctx;

    u8 publicKey[crypto_kx_PUBLICKEYBYTES] = {0};
    u8 privateKey[crypto_kx_SECRETKEYBYTES] = {0};
    
public:
    Application(const char* server_addr, u16 server_port, u8* key, int tun_fd) : ctx{}, guard{boost::asio::make_work_guard(ctx)}, socket{udp::socket(ctx, udp::endpoint(udp::v4(), 0))}, tun_stream{ctx, tun_fd} {
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

        if (crypto_kx_seed_keypair(publicKey, privateKey, key) != 0) {
            throw std::runtime_error("panic: check provided private key again");
        }
    }

    // TODO: add an eternal loop with availability check
    void Handshake() {
        u32 idx = incoming_pool.Acquire();
        IncomingBuffer* buf = &incoming_pool.data[idx];

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
            incoming_pool.Release(idx);
        });
    }

    void Incoming() {
        u32 idx = incoming_pool.Acquire();
        IncomingBuffer* buf = &incoming_pool.data[idx];

        socket.async_receive_from(
            boost::asio::buffer(&buf->packet, sizeof(buf->packet)),
            endpoint, // TODO: fix endpoint replacement
        [this, idx](boost::system::error_code e, std::size_t len) {
            if (e.value() != 0) {
                std::cout << "Incoming() failed: " << e.message() << std::endl;
            } else {
                std::cout << "Received " << len << " bytes from UDP" << std::endl; 
                IncomingBuffer* buf = &incoming_pool.data[idx];

                switch (buf->packet.header.packetType) {
                case DATA: {
                    u8 nonce[12] = {0};
                    u64 counter = __builtin_bswap64(buf->packet.header.counter);
                    memcpy(&nonce[4], &counter, sizeof(u64));

                    u32 out_idx = outgoing_pool.Acquire();
                    OutgoingBuffer* out_buf = &outgoing_pool.data[out_idx];

                    std::shared_lock<std::shared_mutex> lock(mtx);

                    print_hex(nonce, 12);
                    int ret = crypto_aead_aes256gcm_decrypt_afternm(
                        out_buf->payload, &out_buf->len, nullptr,
                        buf->packet.payload, len - sizeof(Header),
                        nullptr, 0,
                        nonce, &crypto_ctx
                    );
                    if (ret < 0) {
                        std::cout << "Error decrypting incoming message: code " << ret << std::endl; 
                        outgoing_pool.Release(out_idx);
                        goto cleanup;
                    }

                    tun_stream.async_write_some(
                        boost::asio::buffer(&out_buf->payload, out_buf->len),
                    [this, out_idx](boost::system::error_code e, std::size_t len){
                        outgoing_pool.Release(out_idx); 
                    });

                    break;
                }

                case HANDSHAKE: {
                    if (len != 32 + sizeof(Header)) goto cleanup;

                    u8 raw_secret [32]   = {0};
                    u8 hash_args  [32*3] = {0};

                    {
                        std::unique_lock<std::shared_mutex> lock(mtx);

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
                        if (ret < 0) goto cleanup; 

                        ret = crypto_aead_aes256gcm_beforenm(&crypto_ctx, session_key);
                        if (ret < 0) goto cleanup;
                    }

                    sodium_memzero(hash_args, 32*3);

                    puts("The shared secret was computed!");

                    int peer_idx = reinterpret_cast<u8*>(&buf->packet.header.peerIndex)[3]-2;
                    if (peer_idx < 0)
                        goto cleanup;

                    peerIndex.store(peer_idx);

                    Outgoing();

                    break;
                }

                default:
                    goto cleanup;
                    break;
                }
            }

        cleanup:
            incoming_pool.Release(idx);
            Incoming();
        });
    }

    void Outgoing() {
        u32 idx = outgoing_pool.Acquire();
        OutgoingBuffer* buf = &outgoing_pool.data[idx];

        tun_stream.async_read_some(
            boost::asio::buffer(buf->payload, sizeof(buf->payload)),
        [this, idx](const boost::system::error_code& e, std::size_t len) {
            if (e.value() != 0) 
                std::cout << "Outgoing() failed: " << e.message() << std::endl;
            else {
               std::cout << "Read " << len << " bytes from TUN" << std::endl;
                OutgoingBuffer* buf = &outgoing_pool.data[idx];

                u64 c = counter.fetch_add(1, std::memory_order_relaxed);
                u8 nonce[12] = {0};
                u64 bcounter = __builtin_bswap64(c);
                memcpy(&nonce[4], &bcounter, sizeof(u64));
 
                u32 out_idx = incoming_pool.Acquire();
                IncomingBuffer* out_buf = &incoming_pool.data[out_idx];

                {
                    std::shared_lock<std::shared_mutex> lock(mtx);
                    int res = crypto_aead_aes256gcm_encrypt_afternm(
                        out_buf->packet.payload, &out_buf->len,
                        buf->payload, len,
                        NULL, 0, NULL,
                        nonce, &crypto_ctx
                    );
                    if (res < 0) {
                        incoming_pool.Release(out_idx);
                        goto cleanup;
                    }
                }

                out_buf->packet.header = {
                    .packetType = DATA, 
                    .peerIndex = peerIndex.load(std::memory_order_relaxed),
                    .counter = c,
                };
                
                socket.async_send_to(
                    boost::asio::buffer(&out_buf->packet, out_buf->len+sizeof(Header)), 
                    endpoint, 
                [this, out_idx](boost::system::error_code e, std::size_t sent_len)
                {
                    std::cout << "Sent packet " << sent_len << " bytes" << std::endl;
                    incoming_pool.Release(out_idx);
                });
            }

        cleanup:
            outgoing_pool.Release(idx);
            Outgoing();
        }); 
    }

    inline void Wait() {
        return run_ctx.join();
    }
};

int main(void) {
    Config config = Config::load_from_file("config.ini");

    int tun_fd = open_tun("cw2"); // TODO: move name definition to config
    if (tun_fd < 0) {
        perror("TUN");
        panic("failed to initialize TUN interface");
    }

    Application client(config.server_addr, config.server_port, config.seed, tun_fd);

    client.Handshake();

    client.Incoming();
    client.Wait();

    return 0;
}
