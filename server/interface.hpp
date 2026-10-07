#pragma once

#include <liburing.h>

#include "../common/types.h"
#include "../common/models.h"

#include "models/queue.h"
#include "models/channel.h"

#include "routing.hpp"

#define BUF_OFFSET(base, idx) reinterpret_cast<void*>(reinterpret_cast<unsigned char*>(base) + buffer_size*idx)

class Interface {
public:
    int Enqueue(u32);
};

class WorkerInterface {
protected:
    int fd;

    struct io_uring ring;
    struct io_uring_buf_ring* buf_ring;
    void* buffers;

    RoutingTable* rtable;
    Queue<u32>* queue;
    Channel<u32>* ch;

    Interface* send;

    enum : u16 {
        READ,
        WRITE,
    };

    struct __info {
        u32 fd;
        u16 op;
        u16 bid;
    };

    int entries, buffer_size;

public:
    int enqueue(u32);
    void incoming();
};
