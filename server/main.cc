#include <stdio.h>
#include <string.h>
#include <netinet/udp.h>
#include <arpa/inet.h>
#include <unistd.h>

#include <sys/mman.h>
#include <liburing.h>


class BufferRing {
private:
    struct io_uring_buf_ring* buf_ring;
    void* base;

public:
    int count = 0;
    int size = 0;

    bool setup() {
        int size = (sizeof(struct io_uring_buf) + this->size) * this->count;
        void* mapped = mmap(NULL, this->size, PROT_READ | PROT_WRITE,
                  MAP_ANONYMOUS | MAP_PRIVATE, 0, 0);
        if (mapped == MAP_FAILED) {
            fprintf(stderr, "buf_ring mmap: %s\n", strerror(errno));
            return false;
        }
        this->buf_ring = (struct io_uring_buf_ring *)mapped;
        io_uring_buf_ring_init(this->buf_ring);
        
        return true; 
    }

    BufferRing(int buffer_count, int buffer_size) : count{buffer_count}, size{buffer_size} {}
};

class Ring {
private:
    struct io_uring ring;

public:
    BufferRing* buffer_ring;

    bool get_sqe(struct io_uring_sqe** sqe) {
        *sqe = io_uring_get_sqe(&this->ring);

        if (!*sqe) {
            io_uring_submit(&this->ring);
            *sqe = io_uring_get_sqe(&this->ring);
        }
        if (!*sqe) {
            fprintf(stderr, "cannot get sqe\n");
            return true;
        }
        return false;
    }

    bool setup() {
        unsigned int buffer_count = this->buffer_ring->count;

        struct io_uring_params params;
        memset(&params, 0, sizeof(params));
        memset(&this->ring, 0, sizeof(ring));

        params.cq_entries = buffer_count * 2;
        params.flags = IORING_SETUP_SUBMIT_ALL | IORING_SETUP_COOP_TASKRUN | IORING_SETUP_CQSIZE;

        int ret = io_uring_queue_init_params(buffer_count, &this->ring, &params);
        if (ret < 0) {
            fprintf(stderr, "queue_init failed: %s\n", strerror(-ret));
            return false;
        }

        if (!this->buffer_ring->setup()) {
            fprintf(stderr, "buffer_ring setup failed");
            io_uring_queue_exit(&this->ring);
            return false;
        }

        return true;
    }

    bool register_fd(int fd) {
        int ret = io_uring_register_files(&this->ring, &fd, 1);
        if (ret) {
            fprintf(stderr, "register files: %s\n", strerror(-ret));
            return false;
        }

        return true;
    }

    Ring(int count, int size) : buffer_ring{new BufferRing(count, size)} {};
};

class Socket {
private:
    int port = 0;
    Ring* ring = nullptr;
    struct msghdr msg;

public:
    int fd = 0;

    bool register_ring(Ring *ring) {
        ring->register_fd(this->fd);
        this->ring = std::move(ring);

        struct io_uring_sqe *sqe;
        if (ring->get_sqe(&sqe))
            return false;

        io_uring_prep_recvmsg_multishot(sqe, 0, &this->msg, MSG_TRUNC);

        sqe->flags |= IOSQE_FIXED_FILE;
        sqe->flags |= IOSQE_BUFFER_SELECT;
        sqe->buf_group = 0;
        io_uring_sqe_set_data64(sqe, ring->buffer_ring->count + 1);
        return 0;

    }

    bool open() {
        int fd = socket(AF_INET, SOCK_DGRAM, 0); // only ipv4 is supported
        if (fd < 0) {
            fprintf(stderr, "sock_init: %s\n", strerror(errno));
            return false;
        }

        struct sockaddr_in addr = {
            .sin_family = AF_INET,
            .sin_port = htons(port),
            .sin_addr = { INADDR_ANY }
        };

        int ret = bind(fd, (struct sockaddr *) &addr, sizeof(addr));
        if (ret) {
            fprintf(stderr, "sock_bind: %s\n", strerror(errno));
            close(fd);
            return false;
        }

        this->fd = fd;
        return true;
    }

    Socket(int port) : port{port} {};
};

int main(void) {
    Socket socket = Socket(43250);
    bool ok = socket.open();
    if (!ok) 
        return 1;

    Ring ring = Ring(256, 65535+16);
    ok = ring.setup();
    if (!ok) 
        return 1;

    return 0;
}
