#pragma once

#include <stdio.h>
#include <string.h>
#include <netinet/udp.h>
#include <arpa/inet.h>
#include <unistd.h>

#include <sys/mman.h>
#include <liburing.h>

#define BUF_COUNT 256
#define BUF_SIZE 65535+16
#define UDP_PORT 45230

#define BUF_OFFSET(base, idx) (reinterpret_cast<unsigned char*>(base) + BUF_SIZE*idx)

class UDP {
private:
    struct io_uring ring;
    struct msghdr msg;
    int fd = 0;

    struct io_uring_buf_ring* buf_ring;
    void* base;
    size_t map_size;

    void recycle(unsigned int idx) {
        io_uring_buf_ring_add(buf_ring, BUF_OFFSET(base, idx), BUF_SIZE, idx, io_uring_buf_ring_mask(BUF_COUNT), 0);
        io_uring_buf_ring_advance(buf_ring, 1);
    }

    bool get_sqe(struct io_uring_sqe** sqe) {
        *sqe = io_uring_get_sqe(&ring);

        if (!*sqe) {
            io_uring_submit(&ring);
            *sqe = io_uring_get_sqe(&ring);    
        }
        if (!*sqe) {
            fprintf(stderr, "cannot get sqe\n");
            return true;
        }
        return false;
    }

    bool add_recv() {
        struct io_uring_sqe *sqe;
        if (get_sqe(&sqe))
            return false;

        io_uring_prep_recvmsg_multishot(sqe, 0, &msg, MSG_TRUNC);

        sqe->flags |= IOSQE_FIXED_FILE;
        sqe->flags |= IOSQE_BUFFER_SELECT;
        sqe->buf_group = 0;
        io_uring_sqe_set_data64(sqe, BUF_COUNT + 1);

        return true;
    }


public:
    int Open();
    bool Setup();
    void Listen();

    ~UDP() {
        close(fd);
        munmap(reinterpret_cast<void*>(buf_ring), map_size);
    }
};
