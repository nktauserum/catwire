#pragma once

#include <vector>
#include <thread>

#include "interface.hpp"
#include "utils/context.hpp"
#include "utils/config.hpp"

#include "models/pool.h"
#include "models/queue.h"


class Tunnel final : public Interface {
    std::vector<std::thread> workers;

public:
    class Worker final : public WorkerInterface {
        static const int entries = 64;
        static const int buffer_size = 1500;
        SharedPool<IncomingBuffer>* pool;
        SharedPool<OutgoingBuffer>* incoming_pool; // TODO: provide
    public: 
        void incoming();
        int enqueue(u32);

        Worker(Context, int, Queue<u32>*, Interface*);
    };

    int Enqueue(u32);
    void Init(Context, Config, Interface*);

    Tunnel();
};
