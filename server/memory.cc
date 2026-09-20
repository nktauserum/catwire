#include "memory.h"

#include <thread>

template <typename T>
int EventPool<T>::Acquire() {
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

template <typename T>
void EventPool<T>::Release(int idx) { 
   if (idx >= 64 || idx < 0) return;
   bitmap.fetch_or(1ull << idx);
}
