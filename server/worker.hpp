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

    void handle_transport(struct io_uring_cqe*, __info&);
    void handle_tunnel(struct io_uring_cqe*, __info&);

    void __transport_process_incoming(Packet*, u32, Address);
    void __transport_process_handshake(Packet*, u32, Address);
    void __transport_process_outgoing(u32);

    void __tunnel_process_incoming(void*, int);
    void __tunnel_process_outgoing(u32);
public:
    void Run();
    Worker(RoutingTable*, Config config);
};
