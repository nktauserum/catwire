#include "../transport.hpp"

#include <vector>
#include <cstring>

#include <liburing.h>

void Transport::Worker::Incoming() {
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

            if (info.op == READ) {
                struct io_uring_recvmsg_out *out = io_uring_recvmsg_validate(BUF_OFFSET(buffers, idx), cqe->res, &msg);
                if (unlikely(!out)) continue;
                if (unlikely(out->flags & MSG_TRUNC)) {
                    io_uring_buf_ring_add(buf_ring, BUF_OFFSET(buffers, idx), buffer_size, idx, io_uring_buf_ring_mask(entries), 0);
                }

                struct sockaddr_in* addr = reinterpret_cast<struct sockaddr_in*>(io_uring_recvmsg_name(out));
                
                void* payload = io_uring_recvmsg_payload(out, &msg);
                u32 sz = io_uring_recvmsg_payload_length(out, cqe->res, &msg);
            }

            io_uring_buf_ring_add(buf_ring, BUF_OFFSET(buffers, idx), buffer_size, idx, io_uring_buf_ring_mask(entries), 0);
        }

        io_uring_buf_ring_advance(buf_ring, count);
        io_uring_cq_advance(&ring, count);
    }
}
