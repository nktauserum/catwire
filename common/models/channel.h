#pragma once

#include <thread>
#include <atomic>

#include "queue.h"
#include "../types.h"

template <typename T, int workers_count = 1>
class Channel {
private:
    Queue<T> workers[workers_count];
    alignas(64) int count = 0;
    alignas(64) u32 next = 0;

public:
    Queue<T>* add_worker() {
        return &workers[reinterpret_cast<std::atomic<int>*>(&count)->fetch_add(1, std::memory_order_relaxed)];
    }

    void push(T item) {
        int idx = (reinterpret_cast<std::atomic<u32>*>(&next)->fetch_add(1, std::memory_order_relaxed) - 1) % workers_count;
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
};

