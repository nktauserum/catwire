#pragma once

#include <thread>
#include <vector>

#include <liburing.h>

#include "interface.hpp"
#include "../common/models.h"
#include "context.hpp"
#include "config.hpp"

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
        
        typedef struct {
            struct msghdr msg;
            struct iovec vec;
        } send_msg;

        std::vector<send_msg> send_queue;
        void __enqueue(u32);
    public:
        void Incoming();
        void Outgoing();

        Worker(Context, int, Queue<u32>*);
    };

    Transport(Context, Config); // TODO: move some fields to config
};
