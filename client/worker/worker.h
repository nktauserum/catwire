#pragma once

#include <stdlib.h>

#include "../models/channel.h"
#include "../models/pool.h"
#include "../session/session.h"

typedef struct {
    Channel* ch;
    Pool*    pool;
    Session  session;

    int udp_fd, tun_fd;
} worker_ctx;

void* worker_init(void*); 

void worker_start(worker_ctx*, Queue*);
void worker_process_receive (worker_ctx*, Buffer*);
void worker_process_transmit(worker_ctx*, Buffer*);
