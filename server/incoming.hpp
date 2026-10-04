#pragma once

#include <thread>
#include <vector>

#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <unistd.h>
#include <error.h>
#include <sys/mman.h>

#include <liburing.h>

#include "../common/types.h"
#include "../common/macro.h"

// static const auto num_cores = std::thread::hardware_concurrency();

#define BUF_OFFSET(base, idx) reinterpret_cast<void*>(reinterpret_cast<unsigned char*>(base) + buffer_size*idx)

class Incoming {
    std::vector<std::thread> workers;

    class Worker {
        struct io_uring ring;
        void* buffers;

        int fd;
        struct msghdr msg;

        static const int entries = 64;
        static const int buffer_size = 1500;

        enum : u16 {
            READ,
            WRITE,
        };

        struct __info {
            u32 fd;
            u16 op;
            u16 bid;
        };

    public:
        void Start();
        Worker(int fd);    
    };

public:
    bool Enqueue(int idx) noexcept;
    Incoming(int num_cores, int port); 
};
