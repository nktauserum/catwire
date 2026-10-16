#include "worker.h"

#include "../../common/models.h"

void worker_process_receive(worker_ctx* ctx, Buffer* buf) {
    Packet* packet = (Packet*)buf->payload;

    switch (packet->header.packetType) {
    case DATA:
        break;

    case HANDSHAKE:
        break;

    default:
        break;
    }
}
