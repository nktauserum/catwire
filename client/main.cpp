#include <iostream>

#include <thread>
#include <mutex>
#include <shared_mutex>

#ifdef __linux__
#include <fcntl.h>
#include <sys/ioctl.h>
#include <linux/if_tun.h>
#endif

#define BOOST_BEAST_HEADER_ONLY
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

    std::atomic<u32> peerIndex = 0;
    std::atomic<u64> counter   = 0;

    std::shared_mutex mtx;
    u8 rx_key[32];
    u8 tx_key[32];

    u8 publicKey [crypto_kx_PUBLICKEYBYTES] = {0};
    u8 privateKey[crypto_kx_SECRETKEYBYTES] = {0};
    
public:
    Application(std::string server_addr, u16 server_port, u8* key, int tun_fd) : ctx{}, guard{boost::asio::make_work_guard(ctx)}, socket{udp::socket(ctx, udp::endpoint(udp::v4(), 0))}, tun_stream{ctx, tun_fd} {
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
        std::cout << "[INFO]: Handshake with " << endpoint << std::endl;
        u32 idx = incoming_pool.Acquire();
        IncomingBuffer* buf = &incoming_pool.data[idx];

        buf->len    = 32;
        buf->packet = Packet {
            .header  = Header {
                .packetType     = HANDSHAKE,
                .peerIndex      = peerIndex.load(),
                .counter        = counter.fetch_add(1),
                .aegis256_nonce = {0}
            },
            .payload = {0},
        };

        memcpy(&buf->packet.payload, publicKey, 32);

        socket.async_send_to(
            boost::asio::buffer(&buf->packet, buf->len+sizeof(Header)), 
            endpoint, 
        [this, idx](boost::system::error_code e, std::size_t sent_len)
        {
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
                IncomingBuffer* buf = &incoming_pool.data[idx];

                switch (buf->packet.header.packetType) {
                case DATA: {
                    u32 out_idx = outgoing_pool.Acquire();
                    OutgoingBuffer* out_buf = &outgoing_pool.data[out_idx];

                    std::shared_lock<std::shared_mutex> lock(mtx);

                    int ret = crypto_aead_aegis256_decrypt(
                        out_buf->payload, &out_buf->len, nullptr,
                        buf->packet.payload, len - sizeof(Header),
                        reinterpret_cast<u8*>(&buf->packet.header), static_cast<u64>(sizeof(Header)),
                        buf->packet.header.aegis256_nonce, rx_key
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

                        if(crypto_kx_client_session_keys(
                            rx_key, tx_key,
                            publicKey, privateKey,
                            buf->packet.payload
                        ) < 0) goto cleanup;
                    }

                    sodium_memzero(hash_args, 32*3);

                    puts("The shared secret was computed!");

                    int peer_idx = ((buf->packet.header.peerIndex >> 24) & 0xFF)-2;
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
                OutgoingBuffer* buf = &outgoing_pool.data[idx];

                u64 c = counter.fetch_add(1, std::memory_order_relaxed);
 
                u32 out_idx = incoming_pool.Acquire();
                IncomingBuffer* out_buf = &incoming_pool.data[out_idx];

                out_buf->packet.header = {
                    .packetType = DATA, 
                    .peerIndex  = peerIndex.load(std::memory_order_relaxed),
                    .counter    = c,
                };

                randombytes_buf(out_buf->packet.header.aegis256_nonce, 32);

                {
                    std::shared_lock<std::shared_mutex> lock(mtx);
                    int res = crypto_aead_aegis256_encrypt(
                        out_buf->packet.payload, &out_buf->len,
                        buf->payload, len,
                        reinterpret_cast<u8*>(&out_buf->packet.header), static_cast<u64>(sizeof(Header)), nullptr,
                        out_buf->packet.header.aegis256_nonce, tx_key 
                    );
                    if (res < 0) {
                        incoming_pool.Release(out_idx);
                        goto cleanup;
                    }
                }
                
                socket.async_send_to(
                    boost::asio::buffer(&out_buf->packet, out_buf->len+sizeof(Header)), 
                    endpoint, 
                [this, out_idx](boost::system::error_code e, std::size_t)
                {
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
    std::cout << "[INFO]: Starting..." << std::endl;

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
