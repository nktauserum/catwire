#pragma once 

#include <liburing.h>

#include "routing.hpp"
#include "utils/config.hpp"
#include "../common/pool.h"
#include "models/buffer.h"

#define BUF_OFFSET(base, idx) reinterpret_cast<void*>(reinterpret_cast<unsigned char*>(base) + buffer_size*idx)

class Worker {
    int fds[2];

    struct io_uring ring;
    struct io_uring_buf_ring* buf_ring;
    void* buffers;

    struct msghdr msg;

    static const int entries = 64;
    static const int buffer_size = 2048;

    enum : u16 {
        READ,
        WRITE,
    };

    struct __info {
        u32 fd;
        u16 op;
        u16 bid;
    };

    RoutingTable* rtable;

    SharedPool<TransportBuffer> transport_pool;
    SharedPool<TunnelBuffer>    tunnel_pool;

public:
    void Run();
    Worker(RoutingTable*, Config config);
};
