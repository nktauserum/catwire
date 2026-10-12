#include "../worker.hpp"

#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <fcntl.h>
#include <linux/if.h>
#include <linux/if_tun.h>
#include <unistd.h>
#include <error.h>
#include <sys/mman.h>
#include <sys/ioctl.h>

#include "../../common/macro.h"

static const char* ifname = "cw0";

Worker::Worker(RoutingTable* rtable, Config config) : 
    rtable{rtable},
    tunnel_pool{SharedPool<TunnelBuffer>()},
    transport_pool{SharedPool<TransportBuffer>()}
{
    {
        struct sockaddr_in addr;
        memset(&addr, 0, sizeof(addr));
        addr.sin_family = AF_INET;
        addr.sin_addr.s_addr = htonl(INADDR_ANY);
        addr.sin_port = htons(config.port);

        int opt = 1;

        fds[0] = socket(AF_INET, SOCK_DGRAM, 0);
        if (fds[0] < 0) {
            perror("UDP socket");
            panic("create UDP socket");
        }

        if (setsockopt(fds[0], SOL_SOCKET, SO_REUSEPORT, &opt, sizeof(opt)) < 0) {
            perror("SO_REUSEPORT");
            panic("setsockopt: re-use port");
        }

        if (setsockopt(fds[0], SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt)) < 0) {
            perror("SO_REUSEADDR");
            panic("setsockopt: re-use addr");
        }

        if (bind(fds[0], (struct sockaddr*)&addr, sizeof(addr)) < 0) {
            perror("bind");
            panic("bind");
        }
    }

    {
        struct ifreq ifr = {0};
        ifr.ifr_flags = IFF_TUN | IFF_NO_PI | IFF_MULTI_QUEUE;
        memcpy(ifr.ifr_ifrn.ifrn_name, ifname, strlen(ifname));

        fds[1] = open("/dev/net/tun", O_RDWR);
        if (fds[1] == -1) {
            perror("tun");
            panic("open tun");
        }

        int rc = ioctl(fds[1], TUNSETIFF, &ifr);
        if (rc == -1) {
            close(fds[1]);
            perror("ioctl: tun");
            panic("setup tun");
        }
    }

    {
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
        if (buffers == MAP_FAILED) {
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

        ret = io_uring_register_files(&ring, fds, 2);
        if (ret) {
            fprintf(stderr, "register files: %s\n", strerror(-ret));
            panic("register fd");
        }
    }

    {
        struct sockaddr_storage temp;
        memset(&msg, 0, sizeof(msg));
        msg.msg_name = &temp;
        msg.msg_namelen = sizeof(struct sockaddr_storage);
        msg.msg_controllen = 0;

        struct io_uring_sqe* sqe = io_uring_get_sqe(&ring);
        if (!sqe) {
            panic("cannot get sqe");
        }

        io_uring_prep_recvmsg_multishot(sqe, 0, &msg, 0);
        sqe->flags |= IOSQE_FIXED_FILE;
        sqe->flags |= IOSQE_BUFFER_SELECT;
        sqe->buf_group = 0;

        u64 inf = 0;
        __info info = {
            .fd  = 0,
            .op  = READ,
            .bid = 0 
        };

        memcpy(&inf, &info, sizeof(__info));
        io_uring_sqe_set_data64(sqe, inf);
    } 

    {
        struct io_uring_sqe* sqe = io_uring_get_sqe(&ring);
        if (!sqe) {
            panic("cannot get sqe");
        }

        io_uring_prep_read_multishot(sqe, 1, 0, 0, 0);
        sqe->flags |= IOSQE_FIXED_FILE;
        sqe->buf_group = 0;

        u64 inf = 0;
        __info info = {
            .fd  = 1,
            .op  = READ,
            .bid = 0
        };
        memcpy(&inf, &info, sizeof(__info));
        io_uring_sqe_set_data64(sqe, inf);
    }

    io_uring_submit(&ring);
}
