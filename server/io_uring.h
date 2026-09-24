#pragma once

#include <cstdlib>
#include <cstring>
#include <cstdio>
#include <liburing.h>
#include <sys/mman.h>

#include "../common/types.h"
#include "../common/macro.h"

// #define entries 256
// #define buffer_size 65535+16
#define BUF_OFFSET(base, idx) reinterpret_cast<void*>(reinterpret_cast<unsigned char*>(base) + buffer_size*idx)
using InitOpFunc = bool(struct io_uring*, struct msghdr*);

template <InitOpFunc func>
class Ring {
    struct io_uring ring;
    struct msghdr msg;
    int fd = 0;

    struct io_uring_buf_ring* buf_ring;
    void* base;
    size_t map_size;

    u32 entries = 256;
    u64 buffer_size = 65535+16;

    struct io_uring_cqe *cqes = nullptr;
public:
    inline bool init(u32 entries, u64 buffer_size) {
        this->entries = entries;
        this->buffer_size = buffer_size;

        struct io_uring_params params;
        memset(&params, 0, sizeof(params));
        memset(&ring, 0, sizeof(ring));

        params.cq_entries = entries * 2;
        params.flags = IORING_SETUP_SUBMIT_ALL | IORING_SETUP_COOP_TASKRUN | IORING_SETUP_CQSIZE;

        int ret = io_uring_queue_init_params(entries, &ring, &params);
        if (ret < 0) {
            fprintf(stderr, "queue_init failed: %s\n", strerror(-ret));
            return false;
        }

        map_size = (sizeof(struct io_uring_buf) + buffer_size) * entries;
        void* mapped = mmap(NULL, map_size, PROT_READ | PROT_WRITE,
                  MAP_ANONYMOUS | MAP_PRIVATE, 0, 0);
        if (mapped == MAP_FAILED) {
            fprintf(stderr, "buf_ring mmap: %s\n", strerror(errno));
            return false;
        }
        buf_ring = (struct io_uring_buf_ring *)mapped;
        io_uring_buf_ring_init(buf_ring);

        base = (unsigned char*)mapped + entries * sizeof(struct io_uring_buf);

        struct io_uring_buf_reg reg = { 
            .ring_addr = reinterpret_cast<unsigned long>(buf_ring),
            .ring_entries = entries,
            .bgid = 0,
            .flags = 0,
            .resv = {0}
        };

        ret = io_uring_register_buf_ring(&ring, &reg, 0);
        if (ret) {
            fprintf(stderr, "buf_ring init failed: %s\n"
                    "NB This requires a kernel version >= 6.0\n",
                    strerror(-ret));
            return ret;
        }

        for (u32 i = 0; i < entries; i++) {
            io_uring_buf_ring_add(buf_ring, BUF_OFFSET(base, i), buffer_size, i,
                          io_uring_buf_ring_mask(entries), i);
        }
        io_uring_buf_ring_advance(buf_ring, entries);

        cqes = reinterpret_cast<struct io_uring_cqe*>(calloc(entries*2, sizeof(struct io_uring_cqe*)));
        if (!cqes) return 1;

        return func(&ring, &msg);
    }

    inline bool register_fd(int fd) {
        // memset(&msg, 0, sizeof(msg));
        // msg.msg_namelen = sizeof(struct sockaddr_storage);
        // msg.msg_controllen = 0;

        int ret = io_uring_register_files(&ring, &fd, 1);
        if (ret) {
            fprintf(stderr, "register files: %s\n", strerror(-ret));
            return false;
        }

        return true;
    }

    inline int wait() {
        return io_uring_submit_and_wait(&ring, 1);
    } 

    inline int batch(struct io_uring_cqe** cqes, size_t size) {
        return io_uring_peek_batch_cqe(&ring, cqes, size);
    }

    inline bool packet_check(struct io_uring_cqe* cqe) {
        if (unlikely(!(cqe->flags & IORING_CQE_F_MORE))) {
            func(&ring, &msg); 
            return false;
        }
        if (unlikely(cqe->res == -ENOBUFS)) {
            return false;
        }
        
        return true;
    }

    inline struct io_uring_recvmsg_out* packet_process(struct io_uring_cqe* cqe) {
        int idx = cqe->flags >> 16;
        struct io_uring_recvmsg_out *out = io_uring_recvmsg_validate(BUF_OFFSET(base, idx), cqe->res, &msg);
        if (unlikely(out == nullptr)) {
            return nullptr;
        }

        if (unlikely(out->flags & MSG_TRUNC)) {
            packet_recycle(cqe);
            return nullptr;
        }

        return out;
    }

    inline struct sockaddr_in* packet_address(struct io_uring_recvmsg_out* msg) {
        return reinterpret_cast<struct sockaddr_in*>(io_uring_recvmsg_name(msg));
    }

    inline u32 packet_size(struct io_uring_recvmsg_out* out, u32 len) {
        return io_uring_recvmsg_payload_length(out, len, &msg);
    }

    inline void* packet_payload(struct io_uring_recvmsg_out* out) {
        return io_uring_recvmsg_payload(out, &msg);
    }

    inline void packet_recycle(struct io_uring_cqe* cqe) {
        int idx = cqe->flags >> 16;
        io_uring_buf_ring_add(buf_ring, BUF_OFFSET(base, idx), buffer_size, idx, io_uring_buf_ring_mask(entries), 0);
        io_uring_buf_ring_advance(buf_ring, 1);
    }

    inline void advance_queue(int count) {
        io_uring_cq_advance(&ring, count);
    }
};
