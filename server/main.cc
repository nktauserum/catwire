#include <string.h>

#include "memory.h"
#include "networking.h"
#include "types.h"

#define WORKERS_COUNT 8

static u64 incoming_counter = 0;

static inline void incomingHandler(Channel<u32, WORKERS_COUNT>& channel, SharedPool<IncomingBuffer>* pool, UDPPacket packet) {
    int idx = pool->Acquire();
    IncomingBuffer* buffer = &pool->data[idx];

    memcpy(&buffer->packet, packet.payload, packet.size); // but if the incoming packet was greater than 65535+17?
    buffer->incoming_addr = packet.addr;
    buffer->idx = incoming_counter++;

    channel.push(idx);
}

int main(void) {
    auto incomingPool = SharedPool<IncomingBuffer>();
    auto incomingCh = Channel<u32, WORKERS_COUNT>();

    UDP udp_listener = UDP(); 
    bool ok = udp_listener.Setup();
    if (!ok) 
        return 1;

    

    return 0;
}
