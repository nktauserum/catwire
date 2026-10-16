#include "worker.h"

void* worker_init(void* arg_ptr) {
    worker_arg* arg = arg_ptr;
    Queue* q = queue_acquire(arg->ch);
    Pool*  p = arg->pool;

    worker_start(q, p);
    
    return NULL;
}

void worker_start(Queue* q, Pool* p) {
    for (;;) {
        int idx;
        Status res = queue_read(q, &idx);
        if (res != OK) {
            if (res == ESIZE) {
                queue_wait(q);
                continue;
            } else if (res == ECLOSED) break;
        }

        Buffer* buf = &p->data[idx];
    }
}
