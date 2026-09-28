#pragma once

#include <cstring>
#include <netinet/udp.h>
#include <arpa/inet.h>
#include <unistd.h>

#include <liburing.h>

#include "io_uring.h"
#include "../common/types.h"
#include "../common/models.h"

class Incoming {
    int fd;

    static const u32 entries = 256; 
    static const u64 buffer_size = 65535+sizeof(Header);

    u64 counter = 0;

    struct {
        struct msghdr msg;
        struct iovec vec;
    } send_queue[entries];

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
    bool init(u16 port);
    void listen(SharedPool<IncomingBuffer>* pool, Channel<u32>* channel);
    bool send(IncomingBuffer* b);
};
