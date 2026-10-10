#include "../interface.hpp"

#include <vector>
#include <cstring>

#include <liburing.h>
#include <sodium.h>

#include <linux/ip.h>

constexpr u32 subnet_mask = 0xFFFFFF00; 
constexpr u32 subnet_addr_val = 0x0A000500; // 10.0.5.0
constexpr u32 subnet_addr = subnet_addr_val & subnet_mask;

void Transport::Worker::incoming() {
    std::vector<struct io_uring_cqe*> cqes(entries*2);

    while (true) {
        int ret = io_uring_submit_and_wait(&ring, 1); 
        if (unlikely(ret == -EINTR))
            continue;
        if (unlikely(ret < 0)) {
            fprintf(stderr, "submit and wait failed %d\n", ret);
            break;
        }

        int count = io_uring_peek_batch_cqe(&ring, &cqes[0], entries*2);
        for (int i = 0; i < count; ++i) {
            struct io_uring_cqe* cqe = cqes[i];

            __info info;
            memcpy(&info, &cqe->user_data, sizeof(__info));
            int idx = (cqe->flags >> 16);

            if (cqe->res < 0) {
                std::cout << "[WARNING]: Transport op failed: code " << cqe->res << std::endl;
                if (info.op == READ) goto cleanup;
                else if (info.op == WRITE) transport_pool->Release(info.bid);
                continue;
            }

            if (info.op == READ) {
                struct io_uring_recvmsg_out *out = io_uring_recvmsg_validate(BUF_OFFSET(buffers, idx), cqe->res, &msg);
                if (unlikely(!out)) continue;

                struct sockaddr_in* addr = reinterpret_cast<struct sockaddr_in*>(io_uring_recvmsg_name(out));
                Packet* packet = reinterpret_cast<Packet*>(io_uring_recvmsg_payload(out, &msg));
                u32 sz = io_uring_recvmsg_payload_length(out, cqe->res, &msg);
                if (sz <= sizeof(Header)) goto cleanup;

                std::cout << "[INFO]: Incoming packet size " << sz << std::endl;
                switch (packet->header.packetType) {
                case DATA: 
                    __process_data(packet, sz, *addr);
                    break;

                case HANDSHAKE: 
                    __process_handshake(packet, sz, *addr);
                    break;

                default:
                    goto cleanup;
                }

            } else {
                transport_pool->Release(info.bid);
                continue;
            } 

        cleanup:
            io_uring_buf_ring_add(buf_ring, BUF_OFFSET(buffers, idx), buffer_size, idx, io_uring_buf_ring_mask(entries), 0);
        }

        io_uring_buf_ring_advance(buf_ring, count);
        io_uring_cq_advance(&ring, count);
    }
}

void Transport::Worker::__process_data(Packet* packet, u32 sz, Address addr) {
    u32 peer_idx = packet->header.peerIndex;
    if (peer_idx >= MAX_CLIENTS) return; 

    int session_idx = rtable->lookup[peer_idx].load(std::memory_order_acquire);
    if (session_idx == -1) return;

    auto session = &rtable->session_pool.data[session_idx];
    if (!session->is_active) {
        std::cout << "[INFO]: Send packet to an inactive session with idx " << session_idx << std::endl;
        return;
    }
    
    u32 idx = tunnel_pool->Acquire();
    TunnelBuffer* buf = &tunnel_pool->data[idx];

    int ret = crypto_aead_aegis256_decrypt(
            buf->payload, &buf->len, nullptr,
            packet->payload, sz - sizeof(Header),
            reinterpret_cast<u8*>(&packet->header), static_cast<u64>(sizeof(Header)),
            packet->header.aegis256_nonce, session->rx_key
        );
    if (ret < 0) {
        std::cout << "Error decrypt an incoming message: code " << ret << std::endl;
        tunnel_pool->Release(idx);
        return;
    }

    if (((addr.sin_addr.s_addr & subnet_mask) == subnet_addr) && addr.sin_addr.s_addr != htonl(0x0A000501)) {
        send->Send(buf->payload, buf->len);
        tunnel_pool->Release(idx);
        return;
    } 

    buf->counter = packet->header.counter; // will be important soon

    // TODO: update address it it has changed
    
    send->Enqueue(idx);
}

void Transport::Worker::__process_handshake(Packet* packet, u32 sz, Address addr) {
    if (sz != 32+sizeof(Header)) return;

    int sessionIndex = rtable->Handshake(packet->payload, addr);
    if (sessionIndex < 0) return;

    printf("The shared secret was computed!\n");
    fflush(stdout);

    // send the server's private key as a response
    u32 out_idx = transport_pool->Acquire();
    TransportBuffer* buf = &transport_pool->data[out_idx];

    buf->idx         = out_idx;
    buf->addr        = addr;
    buf->payload_len = crypto_kx_PUBLICKEYBYTES;
    buf->packet      = Packet {
        .header  = Header {
            .packetType     = HANDSHAKE,
            .peerIndex      = rtable->table[sessionIndex].local_addr,
            .counter        = rtable->table[sessionIndex].counter.fetch_add(1, std::memory_order_relaxed),
            .aegis256_nonce = {0},
        },
        .payload = {0},
    };
    memcpy(&buf->packet.payload, rtable->publicKey, crypto_kx_PUBLICKEYBYTES);

    enqueue(out_idx);
}
