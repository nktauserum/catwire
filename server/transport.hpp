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

    public:
        void Incoming();
        void Outgoing();
        Worker(int fd, Queue<u32>* queue, SharedPool<OutgoingBuffer>* pool, RoutingTable* rtable);
    };

    Transport(int num_cores, int port, Channel<u32>* ch, SharedPool<OutgoingBuffer>* pool, RoutingTable* rtable); // TODO: move some fields to config
};
