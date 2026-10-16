#include "worker.h"

#include "../../common/models.h"

static inline void __handle_data(worker_ctx* ctx, Packet* packet, size_t len);

static inline void __handle_handshake(worker_ctx* ctx, Packet* packet, size_t len) {
    if (len != 32 + sizeof(Header)) return;
    if (session_available(&ctx->session)) return;

    

    session_set_available(&ctx->session);
}

void worker_process_receive(worker_ctx* ctx, Buffer* buf) {
    Packet* packet = (Packet*)buf->payload;
    size_t len = buf->len - sizeof(Header);

    switch (packet->header.packetType) {
    case DATA:
        __handle_data(ctx, packet, len);
        break;

    case HANDSHAKE:
        __handle_handshake(ctx, packet, len);
        break;

    default:
        break;
    }
}
