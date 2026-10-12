#include "../worker.hpp"

#include <linux/ip.h>
#include <sodium.h>

void Worker::handle_tunnel(struct io_uring_cqe* cqe, __info& info) {
    if (info.op == READ) {
        __tunnel_process_incoming(BUF_OFFSET(buffers, (cqe->flags >> 16)), cqe->res);
    } else {
        tunnel_pool.Release(info.bid);
    }
}

void Worker::__tunnel_process_incoming(void* payload, int sz) {
    u32 dest_ip = reinterpret_cast<struct iphdr*>(payload)->daddr;
    u32 ip_byte = (dest_ip >> 24) & 0xFF; 
    if (ip_byte < 2 || ip_byte >= MAX_CLIENTS+2) return;

    u32 lookup_idx = ip_byte - 2;
    int session_idx = rtable->lookup[lookup_idx].load(std::memory_order_acquire);
    if (session_idx == -1) return;

    auto session = &rtable->session_pool.data[session_idx];
    if (!session->is_active) {
        std::cout << "[INFO]: Send packet to an inactive session with idx " << session_idx << std::endl;
        return;
    }
    
    u32 out_idx = transport_pool.Acquire();
    TransportBuffer* out_buf = &transport_pool.data[out_idx];

    u64 counter = session->counter.fetch_add(1, std::memory_order_relaxed);

    out_buf->packet.header = {
        .packetType = DATA,
        .peerIndex  = lookup_idx,
        .counter    = counter,
    };

    randombytes_buf(out_buf->packet.header.aegis256_nonce, 32);

    int res = crypto_aead_aegis256_encrypt(
        out_buf->packet.payload, &out_buf->payload_len,
        reinterpret_cast<u8*>(payload), sz,
        reinterpret_cast<u8*>(&out_buf->packet.header), static_cast<u64>(sizeof(Header)), // Header as additional data
        nullptr, // nsec is never used
        out_buf->packet.header.aegis256_nonce, session->tx_key
    );
    if (res < 0) {
        std::cout << "Error encrypt outgoing packet: ret " << res << std::endl;
        transport_pool.Release(out_idx);
        return;
    }
    out_buf->addr = session->remote_addr;

    __transport_process_outgoing(out_idx);
}

void Worker::__tunnel_process_outgoing(u32 idx) {
    auto data = &tunnel_pool.data[idx];
    struct io_uring_sqe* sqe = io_uring_get_sqe(&ring); 
    if (!sqe) return;
   
    io_uring_prep_write(sqe, fds[1], reinterpret_cast<void*>(data->payload), data->len, 0);

    __info info = {
        .fd  = 1,
        .op  = WRITE,
        .bid = static_cast<u16>(idx)
    };

    u64 info_ = 0;
    memcpy(&info_, &info, sizeof(u64));

    io_uring_sqe_set_data64(sqe, info_);

    io_uring_submit(&ring); // TODO: add batching
}
