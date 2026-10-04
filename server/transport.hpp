#pragma once

#include <thread>
#include <vector>

#include <liburing.h>

#include "interface.hpp"
#include "../common/models.h"

class Transport {
    std::vector<std::thread> workers;

public:
    class Worker final : public Interface {
        struct msghdr msg;

        static const int entries = 64;
        static const int buffer_size = 1500;

        SharedPool<OutgoingBuffer>* pool;
        SharedPool<IncomingBuffer>* incoming_pool; // TODO: provide

        int __process_data(Packet*, u32, Address);
        int __process_handshake(Packet*, u32, Address);

        std::vector<struct io_uring_sqe*> send_queue;
        void __enqueue(u32);
    public:
        void Incoming();
        void Outgoing();

        Worker(int fd, Queue<u32>*, SharedPool<OutgoingBuffer>*, RoutingTable*, Channel<u32>*);
    };

    Transport(int num_cores, int port, Channel<u32>* inc_ch, Channel<u32>* out_ch, SharedPool<OutgoingBuffer>* pool, RoutingTable* rtable); // TODO: move some fields to config
};
