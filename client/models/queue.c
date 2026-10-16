#include "channel.h"

#include <string.h>

#include "futex.h"

#define unlikely(x)  __builtin_expect(!!(x), 0)

void queue_init(Queue* q) {
    memset(q->queue, 0, sizeof(q->queue)); // each queue was allocated with calloc, so this is not needed

    __atomic_clear(&q->head,    __ATOMIC_SEQ_CST);
    __atomic_clear(&q->tail,    __ATOMIC_SEQ_CST);
    __atomic_clear(&q->waiting, __ATOMIC_SEQ_CST);
    __atomic_clear(&q->closed,  __ATOMIC_SEQ_CST);
}

Status queue_write(Queue* q, int value) {
    if (__atomic_load_n(&q->head, __ATOMIC_ACQUIRE) - q->tail_cache == QUEUE_SIZE) {
        q->tail_cache = __atomic_load_n(&q->tail, __ATOMIC_ACQUIRE);
        if (unlikely(__atomic_load_n(&q->head, __ATOMIC_ACQUIRE) - q->tail_cache == QUEUE_SIZE))
            return ESIZE;
    }

    q->queue[__atomic_fetch_add(&q->head, 1, __ATOMIC_RELEASE) % QUEUE_SIZE] = value;

    return OK;
}

Status queue_read(Queue* q, int* buf) {
    if (__atomic_load_n(&q->tail, __ATOMIC_ACQUIRE) == __atomic_load_n(&q->head, __ATOMIC_ACQUIRE)) {
        if (unlikely(__atomic_load_n(&q->closed, __ATOMIC_ACQUIRE)))
            return ECLOSED;
        return ESIZE;
    }

    *buf = q->queue[__atomic_fetch_add(&q->tail, 1, __ATOMIC_RELEASE) % QUEUE_SIZE];

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
