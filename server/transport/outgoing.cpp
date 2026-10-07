#include "../interface.hpp"

#include <vector>
#include <liburing.h>

int Transport::Worker::enqueue(u32 idx) {
    TransportBuffer* buf = &transport_pool->data[idx];
    struct io_uring_sqe* sqe = io_uring_get_sqe(&ring); 
    if (!sqe) return -1;

    buf->vec = (struct iovec) {
        .iov_base = reinterpret_cast<void*>(&buf->packet),
        .iov_len  = buf->len + sizeof(Header),
    };

    buf->hdr = (struct msghdr) { 
        .msg_name       = &buf->addr,
        .msg_namelen    = sizeof(Address),
        .msg_iov        = &buf->vec,
        .msg_iovlen     = 1,
        .msg_control    = nullptr,
        .msg_controllen = 0,
        .msg_flags      = 0,
    };

    io_uring_prep_sendmsg(sqe, fd, &buf->hdr, 0);
    __info info = {
        .fd = static_cast<u32>(fd),
        .op = WRITE, 
        .bid = 0
    };

    u64 i = 0;
    memcpy(&i, &info, sizeof(u64));

    io_uring_sqe_set_data64(sqe, i);
    io_uring_submit(&ring); // TODO: add batching queue 

    return 0;
}

int Transport::Enqueue(u32 idx) {
    int worker_idx = next_worker.fetch_add(1, std::memory_order_relaxed) % workers.size();
    return workers[worker_idx].enqueue(idx);
}
