#include "types.h"

#ifdef TESTING
#include <stdio.h>
#endif

#define DEFAULT_EP_SIZE 64

template <typename T>
class EventPool {
private:
    u64 bitmap;

public:
    T data[64];

    int Acquire() {
        return -1;
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
