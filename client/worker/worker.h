#pragma once

#include <stdlib.h>

#include "../models/channel.h"
#include "../models/pool.h"

typedef struct {
    Channel* ch;
    Pool*    pool;
} worker_arg;

void* worker_init(void*); 

void worker_start(Queue*, Pool*);
