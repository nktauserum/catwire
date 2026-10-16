#include "worker.h"

#include <string.h>

#include "../interface/udp.h"
#include "../log/logs.h"
#include "../../common/models.h"

void* worker_init(void* ctx_ptr) {
    worker_ctx* ctx = ctx_ptr;
    Queue* q = queue_acquire(ctx->ch);

    log_info("Worker started");

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

void worker_handshake(worker_ctx* ctx) {
    Buffer buf = {0};
    Packet* packet = (Packet*)&buf.payload;

    packet->header = (Header) {
        .packetType = HANDSHAKE,
        .peerIndex = 0,
        .counter = 0,
        .aegis256_nonce = {0}
    };
    memcpy(packet->payload, ctx->session.publicKey, 32);

    udp_send(ctx->udp_fd, (const u8*)&buf.payload, 32+sizeof(Header), ctx->session.remoteAddr);
}
