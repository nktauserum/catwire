#include "../interface.hpp"

#include <iostream>
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <unistd.h>
#include <error.h>
#include <sys/mman.h>

#include "../../common/macro.h"

void Transport::Init(Context ctx, Config config, Tunnel* interface) {
    workers.reserve(config.num_cores);
    threads.reserve(config.num_cores);

    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = htonl(INADDR_ANY);
    addr.sin_port = htons(config.port);

    int opt = 1;

    for (int i = 0; i < config.num_cores; ++i) {
        int fd = socket(AF_INET, SOCK_DGRAM, 0);
        if (fd < 0) {
            perror("UDP socket");
            panic("create UDP socket");
        }

        if (setsockopt(fd, SOL_SOCKET, SO_REUSEPORT, &opt, sizeof(opt)) < 0) {
            perror("SO_REUSEPORT");
            panic("setsockopt: re-use port");
        }

        if (setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt)) < 0) {
            perror("SO_REUSEADDR");
            panic("setsockopt: re-use addr");
        }

        if (bind(fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
            perror("bind");
            panic("bind");
        }

        workers.push_back(Worker(ctx, fd, interface));
        auto w = &workers[i];
        threads.push_back(std::thread([w](){
            w->incoming();
        }));
    }

    std::cout << "[INFO]: Listening on UDP 0.0.0.0:" << config.port << " (" << config.num_cores << " threads/fds)" << std::endl;
}

Transport::Worker::Worker(Context ctx, int fd, Tunnel* interface) {
    this->fd     = fd;
    this->tunnel_pool = ctx.tunnel_pool;
    this->transport_pool = ctx.transport_pool;
    this->rtable = ctx.rtable;
    this->send   = interface;

    memset(&ring, 0, sizeof(ring));

    struct io_uring_params params;
    memset(&params, 0, sizeof(params));

    params.cq_entries = entries * 2;
    params.flags = IORING_SETUP_SUBMIT_ALL | IORING_SETUP_COOP_TASKRUN | IORING_SETUP_CQSIZE;

    int ret = io_uring_queue_init_params(entries, &ring, &params);
    if (ret < 0) {
        fprintf(stderr, "queue_init failed: %s\n", strerror(-ret));
        panic("queue_init failed");
    }

    size_t map_size = sizeof(struct io_uring_buf) * entries;
    void* mapped = mmap(NULL, map_size, PROT_READ | PROT_WRITE,
          MAP_ANONYMOUS | MAP_PRIVATE, 0, 0);
    if (mapped == MAP_FAILED) {
        fprintf(stderr, "buf_ring mmap: %s\n", strerror(errno));
        panic("mmap failed");
    }

    buf_ring = reinterpret_cast<struct io_uring_buf_ring*>(mapped);
    io_uring_buf_ring_init(buf_ring);

    buffers = mmap(NULL, entries*buffer_size, PROT_READ | PROT_WRITE,
          MAP_ANONYMOUS | MAP_PRIVATE, 0, 0);
    if (mapped == MAP_FAILED) {
        fprintf(stderr, "buf_ring mmap: %s\n", strerror(errno));
        panic("mmap failed");
    }

    struct io_uring_buf_reg reg;
    memset(&reg, 0, sizeof(reg));
    reg.ring_addr = reinterpret_cast<u64>(mapped);
    reg.ring_entries = entries;

    ret = io_uring_register_buf_ring(&ring, &reg, 0);
    if (ret) {
        fprintf(stderr, "buf_ring init failed: %s\n"
                "NB This requires a kernel version >= 6.0\n",
                strerror(-ret));
        panic("register_buf_ring");
    }

    for (u32 i = 0; i < entries; i++) {
        io_uring_buf_ring_add(buf_ring, BUF_OFFSET(buffers, i), buffer_size, i,
                      io_uring_buf_ring_mask(entries), i);
    }
    io_uring_buf_ring_advance(buf_ring, entries);

    ret = io_uring_register_files(&ring, &fd, 1);
    if (ret) {
        fprintf(stderr, "register files: %s\n", strerror(-ret));
        panic("register fd");
    }

    memset(&msg, 0, sizeof(msg));
    msg.msg_namelen = sizeof(struct sockaddr_storage);
    msg.msg_controllen = 0;

    struct io_uring_sqe* sqe = io_uring_get_sqe(&ring);
    if (!sqe) {
        panic("cannot get sqe");
    }

    io_uring_prep_recvmsg_multishot(sqe, 0, &msg, MSG_TRUNC);

    sqe->flags |= IOSQE_FIXED_FILE;
    sqe->flags |= IOSQE_BUFFER_SELECT;
    sqe->buf_group = 0;

    u64 inf = 0;
    __info info = {
        .fd  = static_cast<u32>(fd),
        .op  = READ,
        .bid = reg.bgid
    };
    memcpy(&inf, &info, sizeof(__info));
    io_uring_sqe_set_data64(sqe, inf);
}
