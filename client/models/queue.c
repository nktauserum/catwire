#include "channel.h"

#include <string.h>

#include "futex.h"

#define unlikely(x)  __builtin_expect(!!(x), 0)

void queue_init(Queue* q) {
    memset(q->queue, 0, sizeof(q->queue)); // each queue was allocated with calloc, so this is not needed

    q->head = q->tail = q->tail_cache = q->waiting = q->closed = 0;
}

Status queue_write(Queue* q, int value) {
    u32 head = __atomic_load_n(&q->head, __ATOMIC_RELAXED);
    if (head - q->tail_cache == QUEUE_SIZE) {
        q->tail_cache = __atomic_load_n(&q->tail, __ATOMIC_ACQUIRE);
        if (unlikely(head - q->tail_cache == QUEUE_SIZE))
            return ESIZE;
    }

    q->queue[head % QUEUE_SIZE] = value;
    __atomic_store_n(&q->head, head + 1, __ATOMIC_RELEASE);

    return OK;
}

Status queue_read(Queue* q, int* buf) {
    u32 tail = __atomic_load_n(&q->tail, __ATOMIC_RELAXED);
    if (tail == __atomic_load_n(&q->head, __ATOMIC_ACQUIRE)) {
        if (unlikely(__atomic_load_n(&q->closed, __ATOMIC_ACQUIRE)))
            return ECLOSED;
        return ESIZE;
    }

    *buf = q->queue[tail % QUEUE_SIZE];
    __atomic_store_n(&q->tail, tail + 1, __ATOMIC_RELEASE);

    return OK;
}

void queue_wait(Queue* q) {
    futex_wait(&q->waiting);
}

void queue_wake(Queue* q) {
    futex_wake(&q->waiting);
}

void queue_close(Queue* q) {
    __atomic_store_n(&q->closed, 1, __ATOMIC_RELEASE);
    queue_wake(q);
}
