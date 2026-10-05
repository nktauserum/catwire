#pragma once

#include <vector>
#include <thread>

#include "interface.hpp"
#include "utils/context.hpp"
#include "utils/config.hpp"

#include "models/pool.h"
#include "models/queue.h"


class Tunnel {
    std::vector<std::thread> workers;

public:
    class Worker final : public Interface {
        static const int entries = 64;
        static const int buffer_size = 1500;
        SharedPool<OutgoingBuffer>* pool;
        SharedPool<IncomingBuffer>* incoming_pool; // TODO: provide
    public: 
        void Incoming();
        void Outgoing();

        Worker(Context, int, Queue<u32>*);
    };

    Tunnel(Context, Config);
};
