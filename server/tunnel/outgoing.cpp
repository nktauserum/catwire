#include "../interface.hpp"

void Tunnel::Worker::flush() {
    u32 len = count.load(std::memory_order_relaxed);
    for (u32 i = 0; i < len; ++i) {
        u32 idx = window[i]; 
        auto data = &tunnel_pool->data[idx];
        struct io_uring_sqe* sqe = io_uring_get_sqe(&ring); 
        if (!sqe) continue;
       
        io_uring_prep_write(sqe, fd, reinterpret_cast<void*>(data->payload), data->len, 0);

        __info info = {
            .op  = WRITE,
            .bid = idx
        };

        u64 info_ = 0;
        memcpy(&info_, &info, sizeof(u64));

        io_uring_sqe_set_data64(sqe, i);
    }

    io_uring_submit(&ring);

}

int Tunnel::Worker::enqueue(u32 idx) {


    return 0;
}

int Tunnel::Enqueue(u32 idx) {
    int worker_idx = next_worker.fetch_add(1, std::memory_order_relaxed) % workers.size();
    return workers[worker_idx].enqueue(idx);
}
