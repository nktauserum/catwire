#pragma once


#include <cstring>
#include <netinet/udp.h>
#include <arpa/inet.h>
#include <unistd.h>

#include <liburing.h>

#include "io_uring.h"
#include "../common/types.h"
#include "../common/macro.h"
#include "../common/models.h"

class UDP {
    int fd;

    u32 entries = 256; 
    u64 buffer_size = 65535+sizeof(Header);

    u64 counter = 0;

    static inline bool setup_ring(struct io_uring* ring, struct msghdr* msg) {
        memset(msg, 0, sizeof(struct msghdr));
        msg->msg_namelen = sizeof(struct sockaddr_storage);
        msg->msg_controllen = 0;

        struct io_uring_sqe* sqe = io_uring_get_sqe(ring);
        if (!sqe) {
            io_uring_submit(ring);
            sqe = io_uring_get_sqe(ring);    
            if (!sqe) {
                fprintf(stderr, "cannot get sqe\n");
                return false;
            }

        }

        io_uring_prep_recvmsg_multishot(sqe, 0, msg, MSG_TRUNC);

        sqe->flags |= IOSQE_FIXED_FILE;
        sqe->flags |= IOSQE_BUFFER_SELECT;
        sqe->buf_group = 0;

        io_uring_sqe_set_data64(sqe, 256 + 1);

        return true;
    }

    Ring<setup_ring> ring;

public: 
    bool init(u16 port) {
        fd = socket(AF_INET, SOCK_DGRAM, 0); // only ipv4 is supported
        if (fd < 0) {
            fprintf(stderr, "sock_init: %s\n", strerror(errno));
            return false;
        }

        struct sockaddr_in addr = {
            .sin_family = AF_INET,
            .sin_port = htons(port),
            .sin_addr = { INADDR_ANY },
            .sin_zero = {0}
        };

        int ret = bind(fd, (struct sockaddr *) &addr, sizeof(addr));
        if (ret) {
            fprintf(stderr, "sock_bind: %s\n", strerror(errno));
            close(fd);
            return false;
        }

        if (!ring.init(entries, buffer_size)) return false;

        return ring.register_fd(fd);
    }

    void listen(SharedPool<IncomingBuffer>* pool, Channel<u32>* channel) {
        struct io_uring_cqe* *cqes = reinterpret_cast<struct io_uring_cqe**>(calloc(entries*2, sizeof(struct io_uring_cqe*))); // possibly null, idc
                                                                                                                             
        while (true) {
            int ret = ring.wait();
            if (unlikely(ret == -EINTR))
                continue;
            if (unlikely(ret < 0)) {
                fprintf(stderr, "submit and wait failed %d\n", ret);
                break;
            }

            int count = ring.batch(cqes, entries*2);
            for (int i = 0; i < count; ++i) {
                if (unlikely(!ring.packet_check(cqes[i]))) continue;
                if (cqes[i]->res < 0) continue;

                if (cqes[i]->user_data > entries) {
                    struct io_uring_recvmsg_out *msg = ring.packet_process(cqes[i]);
                    if (unlikely(!msg))
                        continue;

                    struct sockaddr_in *addr = ring.packet_address(msg);

                    u32 idx = pool->Acquire();
                    IncomingBuffer* buf = &pool->data[idx];

                    u32 sz = ring.packet_size(msg, cqes[i]->res);
                    memcpy(&buf->packet, ring.packet_payload(msg), sz);
                    buf->addr = *addr;
                    buf->idx = counter++;
                    buf->len = sz - sizeof(Header);

                    channel->push(idx);

                    printf("Incoming packet: size %d\n", sz);
                    fflush(stdout);
                } else {
                    pool->Release(cqes[i]->user_data);
                }

                ring.packet_recycle(cqes[i]);
            }

            ring.advance_queue(count);
        }
    }
};
