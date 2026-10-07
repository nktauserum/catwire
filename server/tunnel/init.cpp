#include "../tunnel.hpp"

#include <fcntl.h>
#include <linux/if.h>
#include <linux/if_tun.h>
#include <cstring>
#include <sys/ioctl.h>
#include <unistd.h>
#include <error.h>
#include <sys/mman.h>


static const char* ifname = "cw0";

void Tunnel::Tunnel::Init(Context ctx, Config config, Interface* interface) {
    workers.reserve(config.num_cores*2);

    struct ifreq ifr = {0};
    ifr.ifr_flags = IFF_TUN | IFF_NO_PI | IFF_MULTI_QUEUE;
    memcpy(ifr.ifr_ifrn.ifrn_name, ifname, strlen(ifname));

    for (int i = 0; i < config.num_cores*2; ++i) {
        int qfd = open("/dev/net/tun", O_RDWR);
        if (qfd == -1) {
            perror("tun");
            panic("open tun");
        }

        int rc = ioctl(qfd, TUNSETIFF, &ifr);
        if (rc == -1) {
            close(qfd);
            perror("ioctl: tun");
            panic("setup tun");
        }

        Worker w(ctx, qfd, ctx.outgoing_channel->add_worker(), interface);
        workers[i] = std::thread([&w](){
            w.incoming();
        });
    }
}

Tunnel::Worker::Worker(Context ctx, int fd, Queue<u32>* queue, Interface* interface) {
    this->fd = fd;
    this->queue = queue;
    this->ch = ctx.outgoing_channel;
    this->pool = ctx.incoming_pool;
    this->incoming_pool = ctx.outgoing_pool;
    this->send = interface;

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

    struct io_uring_sqe* sqe = io_uring_get_sqe(&ring);
    if (!sqe) {
        panic("cannot get sqe");
    }

    io_uring_prep_read_multishot(sqe, 0, 0, 0, MSG_TRUNC);
    sqe->flags |= IOSQE_FIXED_FILE;
    sqe->buf_group = 0;

    u64 inf = 0;
    __info info = {
        .fd  = static_cast<u32>(fd),
        .op  = READ,
        .bid = reg.bgid
    };
    memcpy(&inf, &info, sizeof(__info));
    io_uring_sqe_set_data64(sqe, inf);

    io_uring_submit(&ring);
}
