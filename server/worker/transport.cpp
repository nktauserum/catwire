#include "../worker.hpp"

#include <sodium.h>

constexpr u32 subnet_mask = 0xFFFFFF00; 
constexpr u32 subnet_addr_val = 0x0A000500; // 10.0.5.0
constexpr u32 subnet_addr = subnet_addr_val & subnet_mask;

void Worker::handle_transport(struct io_uring_cqe* cqe, __info& info) {
    int idx = (cqe->flags >> 16);

    if (info.op == READ) {
        struct io_uring_recvmsg_out *out = io_uring_recvmsg_validate(BUF_OFFSET(buffers, idx), cqe->res, &msg);
        if (unlikely(!out)) return;

        struct sockaddr_in* addr = reinterpret_cast<struct sockaddr_in*>(io_uring_recvmsg_name(out));
        Packet* packet = reinterpret_cast<Packet*>(io_uring_recvmsg_payload(out, &msg));
        u32 sz = io_uring_recvmsg_payload_length(out, cqe->res, &msg);
        if (sz <= sizeof(Header)) return;

        switch (packet->header.packetType) {
        case DATA: 
            __transport_process_incoming(packet, sz, *addr);
            break;

        case HANDSHAKE: 
            __transport_process_handshake(packet, sz, *addr);
            break;

        default:
            break;
        }

    } else {
        transport_pool.Release(info.bid);
    } 
}

void Worker::__transport_process_incoming(Packet* packet, u32 sz, Address addr) {
    u32 peer_idx = packet->header.peerIndex;
    if (peer_idx >= MAX_CLIENTS) return; 

    int session_idx = rtable->lookup[peer_idx].load(std::memory_order_acquire);
    if (session_idx == -1) return;

    auto session = &rtable->session_pool.data[session_idx];
    if (!session->is_active) {
        std::cout << "[INFO]: Send packet to an inactive session with idx " << session_idx << std::endl;
        return;
    }
    
    u32 idx = tunnel_pool.Acquire();
    TunnelBuffer* buf = &tunnel_pool.data[idx];

    int ret = crypto_aead_aegis256_decrypt(
            buf->payload, &buf->len, nullptr,
            packet->payload, sz - sizeof(Header),
            reinterpret_cast<u8*>(&packet->header), static_cast<u64>(sizeof(Header)),
            packet->header.aegis256_nonce, session->rx_key
        );
    if (ret < 0) {
        tunnel_pool.Release(idx);
        return;
    }

    if (((addr.sin_addr.s_addr & subnet_mask) == subnet_addr) && addr.sin_addr.s_addr != htonl(0x0A000501)) {
        __tunnel_process_incoming(buf->payload, buf->len);
        tunnel_pool.Release(idx);
        return;
    } 

    buf->counter = packet->header.counter; // will be important soon

    // TODO: update address it it has changed
    
    __tunnel_process_outgoing(idx);
}

void Worker::__transport_process_handshake(Packet* packet, u32 sz, Address addr) {
    if (sz != 32+sizeof(Header)) return;

    int sessionIndex = rtable->Handshake(packet->payload, addr);
    if (sessionIndex < 0) return;

    auto session = &rtable->session_pool.data[sessionIndex];

    printf("The shared secret was computed!\n");
    fflush(stdout);

    // send the server's private key as a response
    u32 out_idx = transport_pool.Acquire();
    TransportBuffer* buf = &transport_pool.data[out_idx];

    buf->idx         = out_idx;
    buf->addr        = addr;
    buf->payload_len = crypto_kx_PUBLICKEYBYTES;
    buf->packet      = Packet {
        .header  = Header {
            .packetType     = HANDSHAKE,
            .peerIndex      = session->local_addr,
            .counter        = session->counter.fetch_add(1, std::memory_order_relaxed),
            .aegis256_nonce = {0},
        },
        .payload = {0},
    };
    memcpy(&buf->packet.payload, rtable->publicKey, crypto_kx_PUBLICKEYBYTES);

    __transport_process_outgoing(out_idx);
}

void Worker::__transport_process_outgoing(u32 idx) {
    TransportBuffer* buf = &transport_pool.data[idx];
    struct io_uring_sqe* sqe = io_uring_get_sqe(&ring); 
    if (!sqe) return;

    buf->vec = (struct iovec) {
        .iov_base = reinterpret_cast<void*>(&buf->packet),
        .iov_len  = buf->payload_len + sizeof(Header),
    };

    buf->hdr = (struct msghdr) { 
        .msg_name       = &buf->addr,
        .msg_namelen    = sizeof(Address),
        .msg_iov        = &buf->vec,
        .msg_iovlen     = 1,
        .msg_control    = nullptr,
        .msg_controllen = 0,
        .msg_flags      = 0,
    };

    io_uring_prep_sendmsg(sqe, fds[0], &buf->hdr, 0);
    __info info = {
        .fd = 0,
        .op = WRITE, 
        .bid = static_cast<u16>(idx)
    };

    u64 i = 0;
    memcpy(&i, &info, sizeof(u64));

    io_uring_sqe_set_data64(sqe, i);
    io_uring_submit(&ring); // TODO: add batching queue 
}
