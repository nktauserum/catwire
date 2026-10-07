#include "../interface.hpp"

#include <vector>
#include <linux/ip.h>
#include <liburing.h>

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
            if (cqe->res < 0) continue;

            __info info;
            memcpy(&info, &cqe->user_data, sizeof(__info));

            u32 idx = cqe->flags >> 16;
            void* payload = BUF_OFFSET(buffers, idx);
            int sz = cqe->res;

            u32 dest_ip = reinterpret_cast<struct iphdr*>(payload)->daddr;
            u32 session_idx = ((dest_ip >> 24) & 0xFF) - 2; // subtract two from the fourth byte 

            auto session = &rtable->table[session_idx];
            if (!session->is_active.load(std::memory_order_consume)) goto cleanup;

            {
                u8 nonce[12] = {0};
                u64 counter  = session->add_counter();
                u64 bcounter = __builtin_bswap64(counter);
                memcpy(&nonce[4], &bcounter, sizeof(u64));
                
                u32 out_idx = pool->Acquire();
                IncomingBuffer* out_buf = &pool->data[out_idx];

                {
                    std::shared_lock<std::shared_mutex> lock(session->mtx);

                    int res = crypto_aead_aes256gcm_encrypt_afternm(
                        out_buf->packet.payload, &out_buf->len,
                        reinterpret_cast<u8*>(payload), sz,
                        NULL, 0, NULL,
                        nonce, &session->crypto_ctx
                    );
                    if (res < 0) {
                        std::cout << "Error encrypt outgoing packet: ret " << res << std::endl;
                        pool->Release(out_idx);
                        goto cleanup;
                    }
                    out_buf->addr = session->remote_addr;
                }

                out_buf->packet.header = {
                    .packetType = DATA,
                    .peerIndex = session_idx,
                    .counter = counter,
                };

                send->Enqueue(out_idx);
            }

        cleanup:
            io_uring_buf_ring_add(buf_ring, BUF_OFFSET(buffers, idx), buffer_size, idx, io_uring_buf_ring_mask(entries), 0);
        }
        
        io_uring_buf_ring_advance(buf_ring, count);
        io_uring_cq_advance(&ring, count);
    }
}
