#pragma once

#include <atomic>
#include "types.h"

#ifdef TESTING
#include <stdio.h>
#endif

template <typename T>
class EventPool {
private:
    std::atomic<u64> bitmap;

public:
    T data[64];

    int Acquire();
    void Release(int idx);

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

