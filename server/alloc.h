#pragma once
#ifndef ALLOC_H
#define ALLOC_H

#include <atomic>
#include <chrono>
#include <bit>
#include <thread>

#include "types.h"

#ifdef TESTING
#include <stdio.h>
#endif

#define DEFAULT_EP_SIZE 64

template <typename T>
class EventPool {
private:
    std::atomic<u64> bitmap;

public:
    T data[64];

    int Acquire() {
        auto backoff = std::chrono::nanoseconds(1);
        
        for (;;) {
            u64 b = bitmap.load();
            if (b != 0) {
                int offset = __builtin_ctzll(b);
                if (bitmap.compare_exchange_strong(b, b^(1ull<<offset))) return offset;

                backoff = std::chrono::nanoseconds(1);
                continue;
            }
            
            std::this_thread::sleep_for(backoff);
            if (backoff < std::chrono::milliseconds(1)) {
                backoff *= 2;
            }
        }
    }

    void Release(int idx) {
        
    }

    EventPool() : bitmap{static_cast<u64>(~0)} {
#ifdef TESTING
        for (int offset = 0; offset < 64; ++offset) {
            char bit = bitmap & (1 << offset) ? '1' : '0';
            putc(bit, stdout);
        }
        putc('\n', stdout);
#endif
    }
};

#endif // ALLOC_H
