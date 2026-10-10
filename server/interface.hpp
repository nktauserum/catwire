#pragma once

#include <vector>
#include <thread>
#include <liburing.h>

#include "../common/types.h"
#include "../common/models.h"

#include "models/buffer.h"
#include "../common/pool.h"
#include "utils/context.hpp"
#include "utils/config.hpp"

#include "routing.hpp"

#define BUF_OFFSET(base, idx) reinterpret_cast<void*>(reinterpret_cast<unsigned char*>(base) + buffer_size*idx)

class Interface {
protected:
    int fd;

    struct io_uring ring;
    struct io_uring_buf_ring* buf_ring;
    void* buffers;

    RoutingTable* rtable;

    enum : u16 {
        READ,
        WRITE,
    };

    struct __info {
        u32 op;
        u32 bid;
    };

    int entries, buffer_size;

public:
    int enqueue(u32);
    void incoming();
};

class Tunnel;
class Transport {
    class Worker final : public Interface {
        struct msghdr msg;
        static const int entries = 64;
        static const int buffer_size = 2048;

        SharedPool<TransportBuffer>* transport_pool;
        SharedPool<TunnelBuffer>*    tunnel_pool;

        Tunnel* send;

        void __process_data(Packet*, u32, Address);
        void __process_handshake(Packet*, u32, Address);

    public:
        int enqueue(u32);
        void incoming();

        Worker(Context, int, Tunnel*);
    };

    std::vector<std::thread> threads;
    std::vector<Worker> workers;
    std::atomic<u32> next_worker = 0;
public:
    int Enqueue(u32);
    void Init(Context, Config, Tunnel*);
};

class Tunnel {
    class Worker final : public Interface {
        static const int entries = 64;
        static const int buffer_size = 2048;

        Transport* send;

        SharedPool<TransportBuffer>* transport_pool;
        SharedPool<TunnelBuffer>*    tunnel_pool;

        static const int window_size = 16;
        int window[window_size];

        std::atomic<u32> head, count = 0;
    public: 
        void incoming();
        int  enqueue(u32);
        void flush();

        void __process_data(void*, int);

        Worker(Context, int, Transport*);
    };

    std::vector<std::thread> threads;
    std::vector<Worker> workers;
    std::atomic<u32> next_worker = 0;
public:
    int  Enqueue(u32);
    void Send(void*, int);
    void Init(Context, Config, Transport*);
    void Join();
};
