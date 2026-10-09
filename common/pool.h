#pragma once

#include <cstdlib>
#include <thread>
#include <atomic>

#include "types.h"
#include "macro.h"

template <typename T>
class SharedPool {
private:
    std::atomic<u64> bitmap;

public:
    T* data;

    int Acquire() {
        for (;;) {
            u64 b = bitmap.load(std::memory_order_relaxed);
            if (b != 0) {
                int offset = __builtin_ctzll(b);
                if (bitmap.compare_exchange_strong(b, b^(1ull<<offset)), std::memory_order_acquire) return offset;
                continue;
            }
            
            std::this_thread::yield();
        }
    }

    void Release(int idx) { 
        if (unlikely(idx >= 64 || idx < 0)) return;
        bitmap.fetch_or(1ull << idx, std::memory_order_release);
    }

    SharedPool () : bitmap{~0ULL}, data{new T[64]} {}
    ~SharedPool() {
        delete[] data;
    }
};
