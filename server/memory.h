#pragma once

#include <atomic>
#include <linux/futex.h>
#include <thread>

#include "types.h"
#include "futex.h"

#define QUEUE_SIZE 64

#ifdef TESTING
#include <stdio.h>
#endif

template <typename T>
class SharedPool {
private:
    std::atomic<u64> bitmap;

public:
    T data[64];

    int Acquire() {
        for (;;) {
            u64 b = bitmap.load();
            if (b != 0) {
                int offset = __builtin_ctzll(b);
                if (bitmap.compare_exchange_strong(b, b^(1ull<<offset))) return offset;

                continue;
            }
            
            std::this_thread::yield();
        }
    }

    void Release(int idx) { 
        if (idx >= 64 || idx < 0) return;
        bitmap.fetch_or(1ull << idx);
    }

    SharedPool() : bitmap{static_cast<u64>(~0)} {
#ifdef TESTING
        for (int offset = 0; offset < 64; ++offset) {
            char bit = bitmap & (1 << offset) ? '1' : '0';
            putc(bit, stdout);
        }
        putc('\n', stdout);
#endif
    }

};

template <typename T>
struct Queue {
private:
    alignas(64) u32 head = 0;
    alignas(64) u32 tail_cache = 0;
    alignas(64) u32 tail = 0;
   
    T data[QUEUE_SIZE]; // size must be a power of two

public:
    std::atomic<u32> waiting;
    Futex futex;

    inline T* acquire() {
        if (head - tail_cache == QUEUE_SIZE) {
            tail_cache = reinterpret_cast<std::atomic<u32>*>(&tail)->load(std::memory_order_consume);
            if (__builtin_expect(head - tail_cache == QUEUE_SIZE, 0)) {
                return nullptr;
            }
        }

        return &data[head % QUEUE_SIZE];
    }

    inline void push() {
        reinterpret_cast<std::atomic<u32>*>(&head)->store(head+1, std::memory_order_release);
    }

    inline T* read() {
        if (tail == reinterpret_cast<std::atomic<u32>*>(&head)->load(std::memory_order_acquire)) return nullptr;
        return &data[tail % QUEUE_SIZE];
    }

    inline void pop() {
        reinterpret_cast<std::atomic<u32>*>(&tail)->store(tail+1, std::memory_order_release);
    }
};

template <typename T, int workers_count>
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
