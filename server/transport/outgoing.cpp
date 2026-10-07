#include "../interface.hpp"

#include <vector>
#include <liburing.h>

int Transport::Worker::enqueue(u32 idx) {
    IncomingBuffer* b = &incoming_pool->data[idx];
    struct io_uring_sqe* sqe = io_uring_get_sqe(&ring); 
    if (!sqe) return -1;

    auto buf = &send_queue[b->idx];
    buf->vec = (struct iovec) {
        .iov_base = reinterpret_cast<void*>(&b->packet),
        .iov_len  = b->len + sizeof(Header),
    };

    buf->msg = (struct msghdr) { 
        .msg_name       = &b->addr,
        .msg_namelen    = sizeof(Address),
        .msg_iov        = &buf->vec,
        .msg_iovlen     = 1,
        .msg_control    = nullptr,
        .msg_controllen = 0,
        .msg_flags      = 0,
    };

    io_uring_prep_sendmsg(sqe, fd, &buf->msg, 0);
    io_uring_sqe_set_data64(sqe, b->idx);

    io_uring_submit(&ring); // TODO: add batching queue 

    return 0;
}

int Transport::Enqueue(u32 idx) {
    int worker_idx = next_worker.fetch_add(1, std::memory_order_relaxed) % workers.size();
    return workers[worker_idx].enqueue(idx);
}
