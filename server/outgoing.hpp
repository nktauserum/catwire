#pragma once

#include <linux/if.h>

#include "io_uring.h"

#include "../common/types.h"
#include "../common/models.h"

#define TUN_QUEUE_COUNT 4

class Outgoing {
    static bool setup_ring(struct io_uring* ring, struct msghdr*) {
        struct io_uring_sqe* sqe = io_uring_get_sqe(ring);
        if (!sqe) {
            io_uring_submit(ring);
            sqe = io_uring_get_sqe(ring);    
            if (!sqe) {
                fprintf(stderr, "cannot get sqe\n");
                return false;
            }

        }

        io_uring_prep_recv_multishot(sqe, 0, NULL, 0, 0);

        sqe->flags |= IOSQE_FIXED_FILE;
        sqe->flags |= IOSQE_BUFFER_SELECT;
        sqe->buf_group = 0;

        io_uring_sqe_set_data64(sqe, 256 + 1);

        return true;
    }

    Ring<setup_ring> ring;

    int fds[TUN_QUEUE_COUNT];
    char name[IFNAMSIZ];

    static const u32 entries = 256; 
    static const u64 buffer_size = 65535;

    u64 counter = 0;

public:
    bool init(const char*);
    void listen(SharedPool<OutgoingBuffer>*, Channel<u32>*);
};
