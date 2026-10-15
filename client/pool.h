#pragma once

#include <stdlib.h>
#include <stdatomic.h>

#include "buffer.h"

typedef struct {
    _Atomic u64  bitmap;
    Buffer       data[64];
} Pool;

void pool_init(Pool*);
u32 pool_acquire(Pool*);
void pool_release(Pool*, u32);

