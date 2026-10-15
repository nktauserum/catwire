#include "pool.h"

#include <string.h>
#include <sched.h>

#define unlikely(x)  __builtin_expect(!!(x), 0)

void pool_init(Pool* pool) {
    memset(pool->data, 0, sizeof(pool->data[0]) * 64);
    pool->bitmap = ~0ull;
}

u32 pool_acquire(Pool* pool) {
    for (;;) {
        u64 b = atomic_load_explicit(&pool->bitmap, memory_order_relaxed);
        if (b != 0) {
            int offset = __builtin_ctzll(b);
            if (atomic_compare_exchange_strong_explicit(&pool->bitmap, &b, b^(1ull<<offset), memory_order_acquire, memory_order_relaxed)) return offset;
            continue;
        }

        sched_yield();
    }
}

void pool_release(Pool* pool, u32 idx) {
    if (unlikely(idx >= 64)) return; 
    atomic_fetch_or_explicit(&pool->bitmap, 1ull << idx, memory_order_release);
}
