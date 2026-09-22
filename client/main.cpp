#include <thread>

#include <boost/asio.hpp>
using boost::asio::ip::udp;

#include "../common/types.h"
#include "../common/models.h"

class Client {
private:
    udp::socket   socket;
    udp::endpoint endpoint;

    SharedPool<IncomingBuffer> pool;

    std::atomic<u64> peerIndex = 0;
    std::atomic<u64> counter   = 0;
    
public:
    boost::asio::io_context ctx;

    Client(const char* server_addr, const char* server_port) : socket{udp::socket(ctx, udp::endpoint(udp::v4(), 0))} {
        udp::resolver resolver(ctx);
        udp::resolver::results_type endpoints = resolver.resolve(udp::v4(), server_addr, server_port);
        endpoint = *endpoints.begin();
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

    std::thread ctx([&client](){
        client.ctx.run();
    });

    std::thread handshake([&client](){
        client.Handshake();
    });

    handshake.join();

    return 0;
}
