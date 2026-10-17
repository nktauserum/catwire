#include "channel.h"

// https://gcc.gnu.org/onlinedocs/gcc/_005f_005fatomic-Builtins.html

int channel_init(Channel* ch, size_t capacity) {
    ch->capacity = capacity;
    ch->queues   = calloc(capacity, sizeof(Queue));
    if (!ch->queues) return -1;
   
    __atomic_clear(&ch->count, __ATOMIC_SEQ_CST);
    __atomic_clear(&ch->next,  __ATOMIC_SEQ_CST);

    return 0;
}

Queue* queue_acquire(Channel* ch) {
    return &ch->queues[__atomic_fetch_add(&ch->count, 1, __ATOMIC_RELAXED)];
}

void channel_push(Channel* ch, int value) {
    for (;;) {
        u32 idx  = __atomic_fetch_add(&ch->next, 1, __ATOMIC_RELAXED) % ch->capacity;
        Queue* q = &ch->queues[idx];

        Status status = queue_write(q, value);
        if (status != OK) continue;

        if (__atomic_exchange_n(&q->waiting, 0, __ATOMIC_ACQ_REL))
            queue_wake(q);

        break;
    } 
}

void channel_close(Channel* ch) {
    for (uint32_t i = 0; i < ch->capacity; ++i) 
        queue_close(&ch->queues[i]);

    free(ch->queues); // possibly dangerous, bc we don't wait for queue workers to finish their tasks 
}
