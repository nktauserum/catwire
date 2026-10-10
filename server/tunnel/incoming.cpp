#include "../interface.hpp"

#include <vector>
#include <linux/ip.h>
#include <liburing.h>
#include <sodium.h>

void Tunnel::Worker::incoming() {
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
            u32 idx = cqe->flags >> 16;

            if (cqe->res < 0) {
                std::cout << "[WARNING]: Tunnel op failed: code " << cqe->res << std::endl;
                if (info.op == READ) goto cleanup;
                else if (info.op == WRITE) tunnel_pool->Release(info.bid);
                continue;
            }

            if (info.op == READ) {
                void* payload = BUF_OFFSET(buffers, idx);
                int sz = cqe->res;

                __process_data(payload, sz); 
            } else {
                tunnel_pool->Release(info.bid);
            }

        cleanup:
            io_uring_buf_ring_add(buf_ring, BUF_OFFSET(buffers, idx), buffer_size, idx, io_uring_buf_ring_mask(entries), 0);
        }
        
        io_uring_buf_ring_advance(buf_ring, count);
        io_uring_cq_advance(&ring, count);
    }
}

void Tunnel::Worker::__process_data(void* payload, int sz) {
    u32 dest_ip = reinterpret_cast<struct iphdr*>(payload)->daddr;
    u32 ip_byte = (dest_ip >> 24) & 0xFF; 
    if (ip_byte < 2 || ip_byte >= MAX_CLIENTS+2) return;

    u32 lookup_idx = ip_byte - 2;
    int session_idx = rtable->lookup[lookup_idx].load(std::memory_order_acquire);
    if (session_idx == -1) return;

    auto session = &rtable->session_pool.data[session_idx];
    if (!session->is_active) {
        std::cout << "[INFO]: Send packet to an inactive session with idx " << session_idx << std::endl;
        return;
    }
    std::cout << "[INFO]: Outgoing packet for session " << lookup_idx << std::endl;
    
    u32 out_idx = transport_pool->Acquire();
    TransportBuffer* out_buf = &transport_pool->data[out_idx];

    u64 counter = session->counter.fetch_add(1, std::memory_order_relaxed);

    out_buf->packet.header = {
        .packetType = DATA,
        .peerIndex  = lookup_idx,
        .counter    = counter,
    };

    randombytes_buf(out_buf->packet.header.aegis256_nonce, 32);

    int res = crypto_aead_aegis256_encrypt(
        out_buf->packet.payload, &out_buf->payload_len,
        reinterpret_cast<u8*>(payload), sz,
        reinterpret_cast<u8*>(&out_buf->packet.header), static_cast<u64>(sizeof(Header)), // Header as additional data
        nullptr, // nsec is never used
        out_buf->packet.header.aegis256_nonce, session->tx_key
    );
    if (res < 0) {
        std::cout << "Error encrypt outgoing packet: ret " << res << std::endl;
        transport_pool->Release(out_idx);
        return;
    }
    out_buf->addr = session->remote_addr;

    send->Enqueue(out_idx);
}

void Tunnel::Send(void* payload, int sz) {
    int worker_idx = next_worker.fetch_add(1, std::memory_order_relaxed);
     workers[worker_idx].__process_data(payload, sz);
}
