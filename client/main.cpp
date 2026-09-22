#include <thread>
#include <iostream>

#include <boost/asio.hpp>
using boost::asio::ip::udp;

#include "../common/types.h"
#include "../common/models.h"

class Client {
private:
    boost::asio::io_context ctx;
    std::thread run_ctx;
    boost::asio::executor_work_guard<boost::asio::io_context::executor_type> guard;

    udp::socket   socket;
    udp::endpoint endpoint;
    
    SharedPool<IncomingBuffer> pool;

    std::atomic<u64> peerIndex = 0;
    std::atomic<u64> counter   = 0;
    
public:

    Client(const char* server_addr, const char* server_port) : ctx{}, guard{boost::asio::make_work_guard(ctx)}, socket{udp::socket(ctx, udp::endpoint(udp::v4(), 0))} {
        udp::resolver resolver(ctx);
        udp::resolver::results_type endpoints = resolver.resolve(udp::v4(), server_addr, server_port);
        endpoint = *endpoints.begin();

        run_ctx = std::thread([this](){
            ctx.run();
        });
    }

    // TODO: add an eternal loop
    void Handshake() {
        u32 idx = pool.Acquire();
        IncomingBuffer* buf = &pool.data[idx];

        buf->len    = 32;
        buf->packet = Packet {
            .header  = Header {
                .packetType = HANDSHAKE,
                .peerIndex  = peerIndex.load(),
                .counter    = counter.load(),
            },
            .payload = {0},
        };

        socket.async_send_to(
            boost::asio::buffer(&buf->packet, buf->len), 
            endpoint, 
        [this, &idx](boost::system::error_code e, std::size_t sent_len)
        {
            pool.Release(idx);
        });
    }
};

int main(void) {
    Client client("127.0.0.1", "43250");

    std::thread handshake([&client](){
        client.Handshake();
    });

    handshake.join();

    return 0;
}
