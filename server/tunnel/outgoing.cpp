#include "../interface.hpp"

int Tunnel::Worker::enqueue(u32 idx) {
    auto data = &incoming_pool->data[idx];
    struct io_uring_sqe* sqe = io_uring_get_sqe(&ring); 
    if (!sqe) return -1;
   
    io_uring_prep_write(sqe, fd, reinterpret_cast<void*>(data->payload), data->len, 0);
    io_uring_sqe_set_data64(sqe, idx); // TODO: provide __info struct

    io_uring_submit(&ring);

    return 0;
}

int Tunnel::Enqueue(u32 idx) {
    int worker_idx = next_worker.fetch_add(1, std::memory_order_relaxed) % workers.size();
    return workers[worker_idx].enqueue(idx);
}
