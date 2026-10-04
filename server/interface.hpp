#pragma once

#include <liburing.h>

#include "../common/types.h"
#include "../common/models.h"

#define BUF_OFFSET(base, idx) reinterpret_cast<void*>(reinterpret_cast<unsigned char*>(base) + buffer_size*idx)

class Interface {
protected:
    int fd;

    struct io_uring ring;
    void* buffers;

    Queue<u32>* queue;

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
    void Incoming();
    void Outgoing();
};
