#include "../worker.hpp"

void Worker::Run() {
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
                std::cout << "[WARNING]: operation failed: code " << cqe->res << std::endl;
                if (info.op == READ) goto cleanup;
                else if (info.op == WRITE) {
                     if (info.fd == 0) transport_pool.Release(info.bid);
                     if (info.fd == 1) tunnel_pool.Release(info.bid);
                }
                continue;
            }

            switch (info.fd) {
            case 0:
                handle_transport(cqe, info);
                break;
            case 1:
                handle_tunnel(cqe, info);
                break;
            default:
                break;
            }

        cleanup:
            io_uring_buf_ring_add(buf_ring, BUF_OFFSET(buffers, idx), buffer_size, idx, io_uring_buf_ring_mask(entries), 0);
        }

        io_uring_buf_ring_advance(buf_ring, count);
        io_uring_cq_advance(&ring, count);

        if (io_uring_sq_ready(&ring) > 0)
            io_uring_submit(&ring);
    }

}
