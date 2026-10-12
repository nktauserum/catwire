#pragma once 

#include <thread>
#include <liburing.h>

#include "routing.hpp"
#include "../common/pool.h"
#include "models/buffer.h"

class Worker {
    std::thread t;
    u32 fds[2];

    struct io_uring ring;
    struct io_uring_buf_ring* buf_ring;
    void* buffers;

    static const int entries = 64;
    static const int buffer_size = 2048;

    struct __info {
        u32 fd;
        u16 op;
        u16 bid;
    };

    RoutingTable* rtable;

    SharedPool<TransportBuffer>* transport_pool;
    SharedPool<TunnelBuffer>*    tunnel_pool;

public:
    Worker(RoutingTable*);
};


