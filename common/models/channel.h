#pragma once

#include <thread>
#include <atomic>

#include "queue.h"
#include "../types.h"
#include "../macro.h"

template <typename T>
struct Channel {
private:
    Queue<T> *workers;
    alignas(64) int size = 1;
    alignas(64) int count = 0;
    alignas(64) u32 next = 0;

public:
    Queue<T>* add_worker() {
        return &workers[reinterpret_cast<std::atomic<int>*>(&count)->fetch_add(1, std::memory_order_relaxed)];
    }

    void push(T item) {
        int idx = (reinterpret_cast<std::atomic<u32>*>(&next)->fetch_add(1, std::memory_order_relaxed) - 1) % size;
        Queue<T>& worker = workers[idx];

        T* buf;
        while (!(buf = worker.acquire())) {
            std::this_thread::yield(); // give the item to another worker if this is unavailable?
        }
        *buf = item;
        worker.push();

        if (worker.waiting.load(std::memory_order_relaxed)) {
            worker.waiting.store(0, std::memory_order_relaxed);
            worker.futex.wake(&worker.waiting);
        }
    }

    Channel(int size) : size{size} {
        workers = reinterpret_cast<Queue<T>*>(calloc(size, sizeof(Queue<T>)));
        if (!workers) throw panic("buy more ram lol");
    }
};

