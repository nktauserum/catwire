#include "networking.h"

int UDP::Open() {
    int fd = socket(AF_INET, SOCK_DGRAM, 0); // only ipv4 is supported
    if (fd < 0) {
        fprintf(stderr, "sock_init: %s\n", strerror(errno));
        return -1;
    }

    int opt = 1;
    if (setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt)) < 0) {
        perror("setsockopt failed");
        close(fd);
        return 1;
    }

    struct sockaddr_in addr = {
        .sin_family = AF_INET,
        .sin_port = htons(UDP_PORT),
        .sin_addr = { INADDR_ANY },
        .sin_zero = {0}
    };

    int ret = bind(fd, (struct sockaddr *) &addr, sizeof(addr));
    if (ret) {
        fprintf(stderr, "sock_bind: %s\n", strerror(errno));
        close(fd);
        return -1;
    }

    return fd;
}

bool UDP::Setup() {
    fd = Open();
    if (fd < 0) 
        return false;
    

    // init io_uring
    struct io_uring_params params;
    memset(&params, 0, sizeof(params));
    memset(&ring, 0, sizeof(ring));

    params.cq_entries = BUF_COUNT * 2;
    params.flags = IORING_SETUP_SUBMIT_ALL | IORING_SETUP_COOP_TASKRUN | IORING_SETUP_CQSIZE;

    int ret = io_uring_queue_init_params(BUF_COUNT, &ring, &params);
    if (ret < 0) {
        fprintf(stderr, "queue_init failed: %s\n", strerror(-ret));
        return false;
    }

    // init buffer ring
    map_size = (sizeof(struct io_uring_buf) + BUF_SIZE) * BUF_COUNT;
    void* mapped = mmap(NULL, map_size, PROT_READ | PROT_WRITE,
              MAP_ANONYMOUS | MAP_PRIVATE, 0, 0);
    if (mapped == MAP_FAILED) {
        fprintf(stderr, "buf_ring mmap: %s\n", strerror(errno));
        return false;
    }
    buf_ring = (struct io_uring_buf_ring *)mapped;
    io_uring_buf_ring_init(buf_ring);

    base = (unsigned char*)mapped + BUF_COUNT * sizeof(struct io_uring_buf);


    struct io_uring_buf_reg reg = { 
        .ring_addr = reinterpret_cast<unsigned long>(buf_ring),
        .ring_entries = BUF_COUNT,
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

    for (int i = 0; i < BUF_COUNT; i++) {
        io_uring_buf_ring_add(buf_ring, BUF_OFFSET(base, i), BUF_SIZE, i,
                      io_uring_buf_ring_mask(BUF_COUNT), i);
    }
    io_uring_buf_ring_advance(buf_ring, BUF_COUNT);

    memset(&msg, 0, sizeof(msg));
    msg.msg_namelen = sizeof(struct sockaddr_storage);
    msg.msg_controllen = 0;

    ret = io_uring_register_files(&ring, &fd, 1);
    if (ret) {
        fprintf(stderr, "register files: %s\n", strerror(-ret));
        return false;
    }

    return add_recv();
}

void UDP::Listen(Handler* handler) {
    struct io_uring_cqe *cqes[BUF_COUNT*2];
    while (true) {
        int ret = io_uring_submit_and_wait(&ring, 1);
        if (ret == -EINTR)
            continue;
        if (ret < 0) {
            fprintf(stderr, "submit and wait failed %d\n", ret);
            break;
        }

        int count = io_uring_peek_batch_cqe(&ring, &cqes[0], BUF_COUNT*2);
        for (int i = 0; i < count; i++) {
            struct io_uring_cqe *cqe = cqes[i];
            int idx = cqe->flags >> 16;

            if (cqe->user_data > BUF_COUNT) {                
                if (!(cqe->flags & IORING_CQE_F_MORE)) {
                    bool ok = add_recv();
                    if (!ok) continue;
                }
                if (cqe->res == -ENOBUFS) {                      
                    continue;
                }

                struct io_uring_recvmsg_out *out = io_uring_recvmsg_validate(BUF_OFFSET(base, idx), cqe->res, &msg);
                if (!out) {
                    continue;
                }

                if (out->flags & MSG_TRUNC) {
                    recycle(idx);
                    continue;
                }

                struct sockaddr_in *addr = reinterpret_cast<struct sockaddr_in*>(io_uring_recvmsg_name(out));
                // char buff[INET6_ADDRSTRLEN + 1];
                // void *paddr = &addr->sin_addr;
                //
                // const char* name = inet_ntop(AF_INET, paddr, buff, sizeof(buff));
                // if (!name)
                //     name = "<INVALID>";
                //
                // fprintf(stderr, "received %u bytes %d from [%s]:%d\n",
                //     io_uring_recvmsg_payload_length(out, cqe->res, &msg),
                //     out->namelen, name, (int)ntohs(addr->sin_port));

                handler->handleIncoming(UDPPacket { 
                    .addr       = *addr, // maybe provide a pointer? we copy addr twice now
                    .payload    = reinterpret_cast<const char*>(io_uring_recvmsg_payload(out, &msg)),
                    .size       = io_uring_recvmsg_payload_length(out, cqe->res, &msg)
                });
            }
          
            recycle(idx);
        }
    
        io_uring_cq_advance(&ring, count);
    }
}
