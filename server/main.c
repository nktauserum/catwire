#include <stdio.h>
#include <stdbool.h>
#include <string.h>

#include <netinet/udp.h>
#include <arpa/inet.h>
#include <unistd.h>

#include <liburing.h>
#include <sys/mman.h>

#define UDP_PORT 45230
#define RING_BUFFERS_COUNT 256
#define BUFFER_SIZE 65535+16+1024

typedef struct {
    struct io_uring ring;
    struct io_uring_buf_ring *buf_ring;
    size_t buf_ring_size;
   	unsigned char *buffer_base;
	struct msghdr msg;
} context;

int setup_udp(int port) {
   	int fd = socket(AF_INET, SOCK_DGRAM, 0); // only ipv4 is supported
	if (fd < 0) {
		fprintf(stderr, "sock_init: %s\n", strerror(errno));
		return -1;
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
		return -1;
	}

    return fd;
}

static int setup_buffer_pool(context* ctx) {
	ctx->buf_ring_size = (sizeof(struct io_uring_buf) + BUFFER_SIZE) * RING_BUFFERS_COUNT;
	void* mapped = mmap(NULL, ctx->buf_ring_size, PROT_READ | PROT_WRITE,
		      MAP_ANONYMOUS | MAP_PRIVATE, 0, 0);
	if (mapped == MAP_FAILED) {
		fprintf(stderr, "buf_ring mmap: %s\n", strerror(errno));
		return -1;
	}
	ctx->buf_ring = (struct io_uring_buf_ring *)mapped;
	io_uring_buf_ring_init(ctx->buf_ring);

	struct io_uring_buf_reg reg = { 
		.ring_addr = (unsigned long)ctx->buf_ring,
		.ring_entries = RING_BUFFERS_COUNT,
		.bgid = 0
	};
	ctx->buffer_base = (unsigned char *)ctx->buf_ring +
			   sizeof(struct io_uring_buf) * RING_BUFFERS_COUNT;

	int ret = io_uring_register_buf_ring(&ctx->ring, &reg, 0);
	if (ret) {
		fprintf(stderr, "buf_ring init failed: %s\n"
				"NB This requires a kernel version >= 6.0\n",
				strerror(-ret));
		return ret;
	}

	for (int i = 0; i < RING_BUFFERS_COUNT; i++) {
		io_uring_buf_ring_add(ctx->buf_ring, ctx->buffer_base + i*BUFFER_SIZE, BUFFER_SIZE, i,
				      io_uring_buf_ring_mask(RING_BUFFERS_COUNT), i);
	}
	io_uring_buf_ring_advance(ctx->buf_ring, RING_BUFFERS_COUNT);

	return 0;
}

static int setup_context(context *ctx) {
	struct io_uring_params params;

	memset(&params, 0, sizeof(params));
	params.cq_entries = RING_BUFFERS_COUNT * 8;
	params.flags = IORING_SETUP_SUBMIT_ALL | IORING_SETUP_COOP_TASKRUN |
		       IORING_SETUP_CQSIZE;

	int ret = io_uring_queue_init_params(RING_BUFFERS_COUNT, &ctx->ring, &params);
	if (ret < 0) {
		fprintf(stderr, "queue_init failed: %s\n"
				"NB: This requires a kernel version >= 6.0\n",
				strerror(-ret));
		return ret;
	}

	ret = setup_buffer_pool(ctx);
	if (ret)
		io_uring_queue_exit(&ctx->ring);

	memset(&ctx->msg, 0, sizeof(ctx->msg));
	ctx->msg.msg_namelen = sizeof(struct sockaddr_storage);
	ctx->msg.msg_controllen = 0;
	return ret;
}

static bool get_sqe(context *ctx, struct io_uring_sqe **sqe) {
	*sqe = io_uring_get_sqe(&ctx->ring);

	if (!*sqe) {
		io_uring_submit(&ctx->ring);
		*sqe = io_uring_get_sqe(&ctx->ring);
	}
	if (!*sqe) {
		fprintf(stderr, "cannot get sqe\n");
		return true;
	}
	return false;
}

static int add_recv(context *ctx, int idx) {
	struct io_uring_sqe *sqe;

	if (get_sqe(ctx, &sqe))
		return -1;

	io_uring_prep_recvmsg_multishot(sqe, idx, &ctx->msg, MSG_TRUNC);
	sqe->flags |= IOSQE_FIXED_FILE;

	sqe->flags |= IOSQE_BUFFER_SELECT;
	sqe->buf_group = 0;
	io_uring_sqe_set_data64(sqe, RING_BUFFERS_COUNT + 1);
	return 0;
}

static void recycle_buffer(context *ctx, int idx)
{
	io_uring_buf_ring_add(ctx->buf_ring, ctx->buffer_base + idx*BUFFER_SIZE, BUFFER_SIZE, idx,
			      io_uring_buf_ring_mask(RING_BUFFERS_COUNT), 0);
	io_uring_buf_ring_advance(ctx->buf_ring, 1);
}

int main(void) {
    context ctx = {0};

    int udpsock = setup_udp(UDP_PORT);
    if (udpsock < 0) {
        return 1;
    }

    int ret = setup_context(&ctx); 
    if (ret < 0) {
        return 1;
    }

    ret = io_uring_register_files(&ctx.ring, &udpsock, 1);
	if (ret) {
		fprintf(stderr, "register files: %s\n", strerror(-ret));
		return -1;
	}

    ret = add_recv(&ctx, 0);
	if (ret)
		return 1;

   	struct io_uring_cqe *cqes[RING_BUFFERS_COUNT*2];

    while (true) {
		ret = io_uring_submit_and_wait(&ctx.ring, 1);
		if (ret == -EINTR)
			continue;
		if (ret < 0) {
			fprintf(stderr, "submit and wait failed %d\n", ret);
			break;
		}

		int count = io_uring_peek_batch_cqe(&ctx.ring, &cqes[0], RING_BUFFERS_COUNT*2);
		for (int i = 0; i < count; i++) {
            struct io_uring_cqe *cqe = cqes[i];

		    if (cqes[i]->user_data > RING_BUFFERS_COUNT) {                
                if (!(cqe->flags & IORING_CQE_F_MORE)) {
                    ret = add_recv(&ctx, 0);
                    if (ret)
                        return ret;
                }

                if (cqe->res == -ENOBUFS)
                    return 0;

                int idx = cqe->flags >> 16;

                struct io_uring_recvmsg_out *out = io_uring_recvmsg_validate(ctx.buffer_base + idx*BUFFER_SIZE, cqe->res, &ctx.msg);
                if (!out) {
                    fprintf(stderr, "bad recvmsg\n");
                    continue;
                }

                if (out->flags & MSG_TRUNC) {
                    unsigned int r = io_uring_recvmsg_payload_length(out, cqe->res, &ctx.msg);
                    fprintf(stderr, "truncated msg need %u received %u\n",
                            out->payloadlen, r);
                    recycle_buffer(&ctx, idx);
                    return 0;
                }

                struct sockaddr_in *addr = io_uring_recvmsg_name(out);
                char buff[INET6_ADDRSTRLEN + 1];
                void *paddr = &addr->sin_addr;

                const char* name = inet_ntop(AF_INET, paddr, buff, sizeof(buff));
                if (!name)
                    name = "<INVALID>";

                fprintf(stderr, "received %u bytes %d from [%s]:%d\n",
                    io_uring_recvmsg_payload_length(out, cqe->res, &ctx.msg),
                    out->namelen, name, (int)ntohs(addr->sin_port));
            }
          
            recycle_buffer(&ctx, cqe->user_data);
        }
		io_uring_cq_advance(&ctx.ring, count);
	}


    return 0;
}
