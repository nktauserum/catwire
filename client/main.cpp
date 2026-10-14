#include <iostream>

#include <thread>
#include <memory>
#include <cstring>
#include <cstdio>

#ifdef __linux__
#include <fcntl.h>
#include <sys/ioctl.h>
#include <linux/if_tun.h>
#include <unistd.h>
#endif

#include <asio.hpp>
#include <sodium.h>

using asio::ip::udp;

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

struct Session {
    u8 rx_key[32];
    u8 tx_key[32];

    u32 peerIndex = 0;
    u64 counter   = 0;

    // Requires: all fields in Header are set and ready (!!). We cannot change them later - it'll break the decryption on the server side.
    int Encrypt(IncomingBuffer* dest, OutgoingBuffer* src) {
        return crypto_aead_aegis256_encrypt(
            dest->packet.payload, &dest->len,
            src->payload, src->len,
            reinterpret_cast<u8*>(&dest->packet.header), static_cast<u64>(sizeof(Header)), nullptr,
            dest->packet.header.aegis256_nonce, tx_key 
        );
    }

    // Requires: IncomingBuffer::len is set in the actual incoming packet size (with Header)
    int Decrypt(OutgoingBuffer* dest, IncomingBuffer* src) {
        return crypto_aead_aegis256_decrypt(
            dest->payload, &dest->len, nullptr,
            src->packet.payload, src->len - sizeof(Header),
            reinterpret_cast<u8*>(&src->packet.header), static_cast<u64>(sizeof(Header)),
            src->packet.header.aegis256_nonce, rx_key
        );
    }
};

class Client {
private:
    asio::io_context ctx;
    std::vector<std::thread> workers;
    asio::executor_work_guard<asio::io_context::executor_type> guard;
    udp::socket   socket;
    udp::endpoint endpoint;

    int tun_fd = -1;
    asio::posix::stream_descriptor tun_stream;

    SharedPool<IncomingBuffer> incoming_pool;
    SharedPool<OutgoingBuffer> outgoing_pool;

    u8 publicKey [crypto_kx_PUBLICKEYBYTES];
    u8 privateKey[crypto_kx_SECRETKEYBYTES];

    std::atomic<std::shared_ptr<Session>> session;

public:
    Client(Config& config, int tun_fd) : ctx{}, guard{asio::make_work_guard(ctx)}, socket{udp::socket(ctx, udp::endpoint(udp::v4(), 0))}, tun_stream{ctx, tun_fd} {
        udp::socket socket(ctx, udp::endpoint(udp::v4(), 0));

        udp::resolver resolver(ctx);
        udp::resolver::results_type endpoints = resolver.resolve(udp::v4(), config.server_addr, std::to_string(config.server_port));
        endpoint = *endpoints.begin();

        if (sodium_init() < 0) 
            panic("failed to initialize libsodium");

        if (!crypto_aead_aes256gcm_is_available()) 
            panic("AES256-GCM is not supported by your hardware (CPU)");

        if (crypto_kx_seed_keypair(publicKey, privateKey, config.seed) != 0) {
            panic("check provided private key again");
        }

        workers.reserve(4);
        for (int i = 0; i < 4; ++i) 
            workers.emplace_back([this](){
                ctx.run();
            });

    }

    // TODO: add an eternal loop with availability check
    void Handshake() {
        std::cout << "[INFO]: Handshake with " << endpoint << std::endl;
        u32 idx = incoming_pool.Acquire();
        IncomingBuffer* buf = &incoming_pool.data[idx];

        buf->len    = 32;
        buf->packet = Packet {
            .header  = Header {
                .packetType     = HANDSHAKE,
                .peerIndex      = 0,
                .counter        = 0,
                .aegis256_nonce = {0}
            },
            .payload = {0},
        };

        memcpy(&buf->packet.payload, publicKey, 32);

        socket.async_send_to(
            asio::buffer(&buf->packet, buf->len+sizeof(Header)), 
            endpoint, 
        [this, idx](asio::error_code e, std::size_t sent_len)
        {
            incoming_pool.Release(idx);
        });
    }

    void Incoming() {
        u32 idx = incoming_pool.Acquire();
        IncomingBuffer* buf = &incoming_pool.data[idx];

        socket.async_receive_from(
            asio::buffer(&buf->packet, sizeof(buf->packet)),
            endpoint, // TODO: fix endpoint replacement
        [this, idx](asio::error_code e, std::size_t len) {
            if (e.value() != 0) {
                std::cout << "Incoming() failed: " << e.message() << std::endl;
            } else {
                Incoming();

                IncomingBuffer* buf = &incoming_pool.data[idx];
                buf->len = len;

                switch (buf->packet.header.packetType) {
                case DATA: {
                    u32 out_idx = outgoing_pool.Acquire();
                    OutgoingBuffer* out_buf = &outgoing_pool.data[out_idx];

                    {
                        auto s = session.load(std::memory_order_acquire);
                        if (!s) goto cleanup;

                        int ret = s->Decrypt(out_buf, buf);
                        if (ret < 0) {
                            std::cout << "Error decrypting incoming message: code " << ret << std::endl; 
                            outgoing_pool.Release(out_idx);
                            goto cleanup;
                        }
                    }

                    tun_stream.async_write_some(
                        asio::buffer(&out_buf->payload, out_buf->len),
                    [this, out_idx](asio::error_code e, std::size_t len){
                        outgoing_pool.Release(out_idx); 
                    });

                    break;
                }

                case HANDSHAKE: {
                    if (len != 32 + sizeof(Header)) goto cleanup;

                    auto s = std::make_shared<Session>();
                    
                    if (crypto_kx_client_session_keys(
                        s->rx_key, s->tx_key,
                        publicKey, privateKey,
                        buf->packet.payload
                    ) < 0) goto cleanup;

                    puts("The shared secret was computed!");

                    int peer_idx = ((buf->packet.header.peerIndex >> 24) & 0xFF)-2;
                    if (peer_idx < 0)
                        goto cleanup;

                    s->peerIndex = peer_idx;
                
                    session.store(s, std::memory_order_release);

                    break;
                }

                default:
                    goto cleanup;
                    break;
                }
            }

        cleanup:
            incoming_pool.Release(idx);
        });
    }

    void Outgoing() {
        u32 idx = outgoing_pool.Acquire();
        OutgoingBuffer* buf = &outgoing_pool.data[idx];

        tun_stream.async_read_some(
            asio::buffer(buf->payload, sizeof(buf->payload)),
        [this, idx](const asio::error_code& e, std::size_t len) {
            if (e.value() != 0) 
                std::cout << "Outgoing() failed: " << e.message() << std::endl;
            else {
                Outgoing();

                OutgoingBuffer* buf = &outgoing_pool.data[idx];
                buf->len = len;

                u32 out_idx = incoming_pool.Acquire();
                IncomingBuffer* out_buf = &incoming_pool.data[out_idx];

                {
                    auto s = session.load(std::memory_order_acquire);
                    u64 c = std::atomic_ref<u64>(s->counter).fetch_add(1, std::memory_order_relaxed);

                    out_buf->packet.header = {
                        .packetType = DATA, 
                        .peerIndex  = s->peerIndex,
                        .counter    = c,
                    };

                    randombytes_buf(out_buf->packet.header.aegis256_nonce, 32);

                    int ret = s->Encrypt(out_buf, buf);
                    if (ret < 0) {
                        incoming_pool.Release(out_idx);
                        goto cleanup;
                    }
                }

                socket.async_send_to(
                    asio::buffer(&out_buf->packet, out_buf->len+sizeof(Header)), 
                    endpoint, 
                [this, out_idx](asio::error_code e, std::size_t)
                {
                    incoming_pool.Release(out_idx);
                });
            }

        cleanup:
            outgoing_pool.Release(idx);
        }); 
    }

    void Wait() {
        workers[0].join();
    }
};

int main(void) {
    std::cout << "[INFO]: Starting..." << std::endl;

    Config config = Config::load_from_file("config.ini");

    int tun_fd = open_tun("cw2"); // TODO: move name definition to config
    if (tun_fd < 0) {
        perror("TUN");
        return 1;
    }

    Client app(config, tun_fd);

    app.Handshake();
    app.Incoming();
    app.Outgoing();

    app.Wait();

    return 0;
}
