#pragma once

#include <atomic>

#include "../../common/types.h"
#include "../../common/macro.h"
#include "futex.h"

#define QUEUE_SIZE 64

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

    T* acquire() {
        if (head - tail_cache == QUEUE_SIZE) {
            tail_cache = std::atomic_ref<u32>(tail).load(std::memory_order_consume);
            if (unlikely(head - tail_cache == QUEUE_SIZE)) {
                return nullptr;
            }
        }

        return &data[head % QUEUE_SIZE];
    }

    inline void push() {
        std::atomic_ref<u32>(head).fetch_add(1, std::memory_order_release);
    }

    T* read() {
        if (tail == reinterpret_cast<std::atomic<u32>*>(&head)->load(std::memory_order_acquire)) { // TODO: don't fall asleep instantly - spin a little
            return nullptr;
        }
        return &data[tail % QUEUE_SIZE];
    }

    inline void pop() {
        std::atomic_ref<u32>(tail).fetch_add(1, std::memory_order_release);
    }

    void wait() {
        waiting.store(1, std::memory_order_relaxed);
        futex.wait(&waiting, 1);
    }
};

