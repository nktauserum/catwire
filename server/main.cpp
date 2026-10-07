#include <sodium.h>

#include "../common/macro.h"

#include "utils/config.hpp"
#include "interface.hpp"

// #define WORKERS_COUNT 8
//
// class Application {
// private:
//     RoutingTable routingTable;
//
//     u8 publicKey [crypto_kx_PUBLICKEYBYTES] = {0};
//     u8 privateKey[crypto_kx_SECRETKEYBYTES] = {0};
//
//     Channel<u32> incomingChannel;
//     Channel<u32> outgoingChannel;
//
//     SharedPool<IncomingBuffer> incomingPool;
//     SharedPool<OutgoingBuffer> outgoingPool;
//
//     Incoming     incoming;
//     Outgoing     outgoing;
//
// public: 
//     void incomingWorker() {
//         auto queue = incomingChannel.add_worker();
//
//         while (true) {
//             u32 idx = *queue->read();
//             queue->pop();
//             IncomingBuffer* buf = &incomingPool.data[idx];
//
//             switch (buf->packet.header.packetType) {
//             case DATA: {
//                 auto peer_idx = buf->packet.header.peerIndex;
//                 std::cout << "DATA: incoming packet to session: idx " << peer_idx << " size " << buf->len << std::endl;
//                 if (peer_idx >= MAX_CLIENTS) goto cleanup;
//
//                 auto session = &routingTable.table[peer_idx];
//
//                 u8 nonce[12] = {0};
//                 u64 bcounter = __builtin_bswap64(buf->packet.header.counter);
//                 memcpy(&nonce[4], &bcounter, sizeof(u64));
//
//                 u32 out_idx = outgoingPool.Acquire();
//                 auto out_buf = &outgoingPool.data[out_idx];
//                 {
//                     std::shared_lock<std::shared_mutex> lock(session->mtx);
//                     if (!session->is_active.load(std::memory_order_relaxed)) {
//                         std::cout << "Error: send packet to the unavailable session: idx " << peer_idx << std::endl;
//                         outgoingPool.Release(out_idx);
//                         goto cleanup;
//                     }
//
//                     int ret = crypto_aead_aes256gcm_decrypt_afternm(
//                         out_buf->payload, &out_buf->len, nullptr, 
//                         buf->packet.payload, buf->len,
//                         nullptr, 0, // additional data
//                         nonce, &session->crypto_ctx
//                     );
//                     if (ret < 0) {
//                         std::cout << "Error decrypt an incoming message: code " << ret << std::endl;
//                         outgoingPool.Release(out_idx);
//                         goto cleanup;
//                     }
//                 }
//                 out_buf->idx = out_idx;
//                 outgoing.write(out_buf);
//                 break;
//             }
//
//             case HANDSHAKE: {
//                 if (buf->len != 32) goto cleanup;
//                 i32 sessionIndex = routingTable.exists(buf->packet.payload);
//                 if (sessionIndex < 0) {
//                     puts("No such session index");
//                     goto cleanup;
//                 }
//
//                 auto session = &routingTable.table[sessionIndex];
//
//                 u8 raw_secret [32]                             = {0};
//                 u8 hash_args  [32*3]                           = {0};
//
//                 if (crypto_scalarmult(raw_secret, privateKey, buf->packet.payload) != 0) goto cleanup;
//
//                 memcpy(hash_args,    raw_secret,          32);
//                 sodium_memzero(raw_secret, 32);
//                 memcpy(hash_args+32, publicKey, 32);
//                 memcpy(hash_args+64, buf->packet.payload,           32);
//
//                 {
//                     std::unique_lock<std::shared_mutex> lock(session->mtx);
//
//                     int ret = crypto_generichash(
//                         session->shared_key, 32, 
//                         hash_args,           sizeof(hash_args),
//                         nullptr,             0
//                     );
//                     if (ret < 0) goto cleanup;
//
//                     ret = crypto_aead_aes256gcm_beforenm(&session->crypto_ctx, session->shared_key);
//                     if (ret < 0) goto cleanup;
//
//                     session->remote_addr = buf->addr;
//                 }
//
//                 sodium_memzero(hash_args, 32*3);
//
//                 session->is_active.store(1);
//
//                 printf("The shared secret was computed!\n");
//                 fflush(stdout);
//
//                 // send the server's private key as a response
//                 u32 out_idx = incomingPool.Acquire();
//                 IncomingBuffer* out_buf = &incomingPool.data[out_idx];
//
//                 out_buf->idx    = out_idx;
//                 out_buf->addr   = buf->addr;
//                 out_buf->len    = crypto_kx_PUBLICKEYBYTES + sizeof(Header);
//                 out_buf->packet = Packet {
//                     .header  = Header {
//                         .packetType = HANDSHAKE,
//                         .peerIndex  = routingTable.table[sessionIndex].local_addr,
//                         .counter    = routingTable.table[sessionIndex].add_counter(),
//                     },
//                     .payload = {0},
//                 };
//                 memcpy(&out_buf->packet.payload, publicKey, crypto_kx_PUBLICKEYBYTES);
//
//                 if (!incoming.send(out_buf)) {
//                     puts("Incoming::Send() failed");
//                     goto cleanup;
//                 }
//
//                 break;
//             }
//
//             default:
//                 goto cleanup;
//                 break;
//             }
//
//     cleanup:
//             incomingPool.Release(idx);
//         }
//     }
//
//     void outgoingWorker() {
//         auto queue = outgoingChannel.add_worker(); 
//
//         while (true) {
//             u32 idx = *queue->read();
//             queue->pop();
//
//             OutgoingBuffer* buf = &outgoingPool.data[idx];
//             std::cout << "incoming: size " << buf->len << std::endl;
//             u32 out_idx = incomingPool.Acquire();
//             IncomingBuffer* out_buf = &incomingPool.data[out_idx];
//
//             {
//                 if (unlikely(buf->len <= sizeof(struct iphdr))) {
//                     incomingPool.Release(idx);
//                     goto cleanup;
//                 }
//
//                 u32 dest_ip = reinterpret_cast<struct iphdr*>(buf->payload)->daddr;
//                 u32 session_idx = reinterpret_cast<u8*>(&dest_ip)[3]-2;
//
//
//                 auto session = &routingTable.table[session_idx];
//
//                 if (!session->is_active.load(std::memory_order_relaxed)) goto cleanup;
//                 std::cout << "Out: receive a packet for session: idx " << session_idx << std::endl;
//
//                 u64 counter = session->add_counter(); // atomic operation, so keep before the lock
//                 u8 nonce[12] = {0};
//                 u64 bcounter = __builtin_bswap64(counter);
//                 memcpy(&nonce[4], &bcounter, sizeof(u64));
//
//                 print_hex(nonce, 12);
//                 std::shared_lock<std::shared_mutex> lock(session->mtx);
//
//                 int res = crypto_aead_aes256gcm_encrypt_afternm(
//                     out_buf->packet.payload, &out_buf->len,
//                     buf->payload, buf->len,
//                     NULL, 0, NULL,
//                     nonce, &session->crypto_ctx
//                 );
//                 if (res < 0) {
//                     std::cout << "Error encrypt outgoing packet: ret " << res << std::endl;
//                     incomingPool.Release(out_idx);
//                     goto cleanup;
//                 }
//                 out_buf->addr = session->remote_addr;
//
//                 out_buf->packet.header = {
//                     .packetType = DATA,
//                     .peerIndex = session_idx,
//                     .counter = counter,
//                 };
//             }
//
//             incoming.send(out_buf);
//     cleanup:
//             outgoingPool.Release(idx);
//         }
//     }
//


int main(void) {
    if (sodium_init() < 0) 
        panic("failed to initialize libsodium");

    if (!crypto_aead_aes256gcm_is_available()) 
        panic("AES256-GCM is not supported by your hardware (CPU)");

    Config config("config.ini");

    Tunnel    tun;
    Transport udp;

    Context ctx(config);

    tun.Init(ctx, config, &udp);
    udp.Init(ctx, config, &tun);

    tun.Join();

    return 0;
}
