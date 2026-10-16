#include "worker.h"

#include "../interface/udp.h"
#include "../../common/models.h"

void worker_process_transmit(worker_ctx* ctx, Buffer* buf) {
    if (!session_available(&ctx->session)) return;

    u32 idx = pool_acquire(ctx->pool);
    Buffer* dest = &ctx->pool->data[idx];

    Packet* packet = (Packet*)dest->payload;
    packet->header = (Header){
        .packetType = DATA, 
        .peerIndex  = ctx->session.peerIndex,
        .counter    = __atomic_fetch_add(&ctx->session.counter, 1, __ATOMIC_RELAXED),
    };
    randombytes_buf(packet->header.aegis256_nonce, 32);

    if (crypto_aead_aegis256_encrypt(
        packet->payload, (u64*)&dest->len,
        buf->payload, buf->len,
        (u8*)&packet->header, (u64)sizeof(Header), NULL,
        packet->header.aegis256_nonce, ctx->session.tx_key 
    ) < 0) return;

    udp_send(ctx->udp_fd, dest->payload, sizeof(Header)+dest->len, ctx->session.remoteAddr);
    pool_release(ctx->pool, idx);
}
