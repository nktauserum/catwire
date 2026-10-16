#include "worker.h"

void* worker_init(void* ctx_ptr) {
    worker_ctx* ctx = ctx_ptr;
    Queue* q = queue_acquire(ctx->ch);

    worker_start(ctx, q);
    
    return NULL;
}

void worker_start(worker_ctx* ctx, Queue* q) {
    for (;;) {
        int idx;
        Status res = queue_read(q, &idx);
        if (res != OK) {
            if (res == ESIZE) {
                queue_wait(q);
                continue;
            } else if (res == ECLOSED) break;
        }

        Buffer* buf = &ctx->pool->data[idx];

        switch (buf->origin) {
        case Receive:
            worker_process_receive(ctx, buf);
            break;

        case Transmit:
            worker_process_transmit(ctx, buf);
            break;

        default:
            break;
        }

        pool_release(ctx->pool, idx);
    }
}
