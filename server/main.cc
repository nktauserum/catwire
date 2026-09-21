#include <string.h>
#include <stdio.h>
#include <thread>

#include "types.h"
#include "memory.h"
#include "networking.h"

#define WORKERS_COUNT 8

static u64 incoming_counter = 0;

class IncomingHandler : public Handler {
public: 
    Channel<u32, WORKERS_COUNT> channel;
    SharedPool<IncomingBuffer> pool;

    inline void worker() {
        
    }

    inline void handleIncoming(UDPPacket packet) override {
        int idx = pool.Acquire();
        IncomingBuffer* buffer = &pool.data[idx];

        memcpy(&buffer->packet, packet.payload, packet.size); // but if the incoming packet was greater than 65535+17?
        buffer->incoming_addr = packet.addr;
        buffer->idx = incoming_counter++;

        channel.push(idx);

        printf("Incoming packet: payload %lu bytes, idx %lu, buf idx %d\n", packet.size, incoming_counter - 1, idx);
        fflush(stdout);
    }

    IncomingHandler() : channel{Channel<u32, WORKERS_COUNT>()}, pool{SharedPool<IncomingBuffer>()} {};
};

int main(void) {
    UDP udp_listener; 
    bool ok = udp_listener.Setup();
    if (!ok) 
        return 1;

    IncomingHandler handler;
    std::thread incoming([&handler, &udp_listener](){
        udp_listener.Listen(&handler);
    });
    
    incoming.join();

    return 0;
}
