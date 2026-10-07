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
        SharedPool<IncomingBuffer>* pool;
        SharedPool<OutgoingBuffer>* incoming_pool; // TODO: provide
    public: 
        void incoming();
        int enqueue(u32);

        Worker(Context, int, Queue<u32>*);
    };

    int Enqueue();
    void Init(Context, Config);

    Tunnel();
};
