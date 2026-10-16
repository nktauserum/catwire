#include "worker.h"

#include "../../common/models.h"
#include "../log/logs.h"

#include "../interface/tun.h"

static inline void __handle_data(worker_ctx* ctx, Packet* packet, size_t len) {
    if (!session_available(&ctx->session)) return;

    u32 idx = pool_acquire(ctx->pool);
    Buffer* dest = &ctx->pool->data[idx];

    if (crypto_aead_aegis256_decrypt(
        dest->payload, (u64*)&dest->len, NULL,
        packet->payload, len,
        (u8*)&packet->header, (u64)sizeof(Header),
        packet->header.aegis256_nonce, ctx->session.rx_key
    ) < 0) return;

    tun_write(ctx->tun_fd, (const u8*)&dest->payload, dest->len);
}

static inline void __handle_handshake(worker_ctx* ctx, Packet* packet, size_t len) {
    if (len != 32 + sizeof(Header)) return;
    if (session_available(&ctx->session)) return;

    if (crypto_kx_client_session_keys(
        ctx->session.rx_key, ctx->session.tx_key,
        ctx->session.publicKey, ctx->session.privateKey,
        packet->payload
    ) < 0) return;

    int peer_idx = ((packet->header.peerIndex >> 24) & 0xFF)-2;
    if (peer_idx < 0) return;

    log_info("The shared secret was computed!");

    ctx->session.peerIndex = peer_idx;

    session_set_available(&ctx->session);
}

void worker_process_receive(worker_ctx* ctx, Buffer* buf) {
    if (buf->len + 1 < sizeof(Header)) return;

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
