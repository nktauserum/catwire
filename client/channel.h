#pragma once

#include <stdlib.h>

#include "../common/types.h"

#define QUEUE_SIZE 64

typedef enum : int {
    OK,
    ESIZE,
    ECLOSED,
} Status;

typedef struct {
    u32  head, tail, tail_cache;
    u32  waiting, closed;

    int queue[QUEUE_SIZE];
} Queue;

typedef struct {
    Queue* queues;
    size_t capacity;

    u32 count, next;
} Channel;


int  channel_init (Channel*, size_t);
void channel_push (Channel*, int);
void channel_close(Channel*);


Queue* queue_acquire(Channel*);
void   queue_init   (Queue*);

Status queue_write  (Queue*, int);
Status queue_read   (Queue*, int*);

void   queue_wait   (Queue*);
void   queue_wake   (Queue*);
void   queue_close  (Queue*);
