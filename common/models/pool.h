#pragma once

#include <thread>
#include <atomic>

#include "../types.h"
#include "../macro.h"

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
        if (unlikely(idx >= 64 || idx < 0)) return;
        bitmap.fetch_or(1ull << idx);
    }

    SharedPool() : bitmap{static_cast<u64>(~0)} {}

};
