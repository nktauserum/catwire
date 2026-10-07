#pragma once

#include <vector>
#include <thread>
#include <liburing.h>

#include "../common/types.h"
#include "../common/models.h"

#include "models/queue.h"
#include "models/channel.h"
#include "models/pool.h"
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
    Queue<u32>* queue;
    Channel<u32>* ch;

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

class Tunnel;
class Transport {
    class Worker final : public Interface {
        struct msghdr msg;

        static const int entries = 64;
        static const int buffer_size = 1500;

        SharedPool<OutgoingBuffer>* pool;
        SharedPool<IncomingBuffer>* incoming_pool;

        Tunnel* send;

        int __process_data(Packet*, u32, Address);
        int __process_handshake(Packet*, u32, Address);
        
        typedef struct {
            struct msghdr msg;
            struct iovec vec;
        } send_msg;

        std::vector<send_msg> send_queue;

    public:
        int enqueue(u32);
        void incoming();

        Worker(Context, int, Queue<u32>*, Tunnel*);
    };

    std::vector<std::jthread> threads;
    std::vector<Worker> workers;
    std::atomic<u32> next_worker = 0;
public:
    int Enqueue(u32);
    void Init(Context, Config, Tunnel*);
};

class Tunnel {
    class Worker final : public Interface {
        static const int entries = 64;
        static const int buffer_size = 1500;

        Transport* send;

        SharedPool<IncomingBuffer>* pool;
        SharedPool<OutgoingBuffer>* incoming_pool; // TODO: provide
    public: 
        void incoming();
        int enqueue(u32);

        Worker(Context, int, Queue<u32>*, Transport*);
    };

    std::vector<std::jthread> threads;
    std::vector<Worker> workers;
    std::atomic<u32> next_worker = 0;
public:
    int Enqueue(u32);
    void Init(Context, Config, Transport*);
    void Join();
};
