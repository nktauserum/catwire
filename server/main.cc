#include <string.h>
#include <stdio.h>
#include <thread>

#define BOOST_BEAST_HEADER_ONLY
#include <boost/beast/core/detail/base64.hpp>

#include "types.h"
#include "memory.h"
#include "networking.h"

#define WORKERS_COUNT 8

static u64 incoming_counter = 0;

class Application : public Handler {
private:
    char privateKeyBytes[32];

    Channel<u32, WORKERS_COUNT> channel;
    SharedPool<IncomingBuffer> pool;
public: 
    inline void worker() {
        auto queue = channel.add_worker();

        while (true) {
            u32 idx = *queue->read();

            // do some work

            queue->pop();
            pool.Release(idx);
        }
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

    Application(const char* key) : channel{Channel<u32, WORKERS_COUNT>()}, pool{SharedPool<IncomingBuffer>()} {
        size_t n = boost::beast::detail::base64::decode(&privateKeyBytes, key, strlen(key)).first;
    };
};

int main(void) {
    UDP udp_listener; 
    bool ok = udp_listener.Setup();
    if (!ok) 
        return 1;

    Application app{"zb1NPTbALjQmO/aWqF2YUnRJC1igyulIsk6zQK5nhEE="};

    std::thread workers[WORKERS_COUNT];
    for (int i = 0; i < WORKERS_COUNT; ++i) {
        workers[i] = std::thread([&app](){
            app.worker();
        });
    }

    std::thread incoming([&app, &udp_listener](){
        udp_listener.Listen(&app);
    });


    incoming.join();

    return 0;
}
