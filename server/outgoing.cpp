#include "outgoing.hpp"

#include <fcntl.h>
#include <linux/if_tun.h>
#include <cstring>
#include <sys/ioctl.h>
#include <unistd.h>

bool Outgoing::init(const char* ifname) {
	size_t ifname_len = strlen(ifname);
    if (ifname_len >= IFNAMSIZ) {
        return false;
    }

    struct ifreq ifr = {0};
    ifr.ifr_flags = IFF_TUN | IFF_NO_PI | IFF_MULTI_QUEUE | IFF_VNET_HDR;
    memcpy(ifr.ifr_ifrn.ifrn_name, ifname, ifname_len);

    int i;
    for (i = 0; i < TUN_QUEUE_COUNT; ++i) {
        int qfd = open("/dev/net/tun", O_RDWR);
        if (qfd == -1) {
            goto cleanup;
        }

        int rc = ioctl(qfd, TUNSETIFF, &ifr);
        if (rc == -1) {
            close(qfd);
            goto cleanup;
        }

        fds[i] = qfd;
    }

    memcpy(name, ifr.ifr_name, IFNAMSIZ);

    if (!ring.init(entries, buffer_size)) 
        goto cleanup;

    for (int j = 0; j < TUN_QUEUE_COUNT; j++) {
        if (!ring.register_fd(fds[j]))
            goto cleanup;
    }

    return true;

cleanup:
    for (--i; i >= 0; i--)
        close(fds[i]);
    return false;
}

void Outgoing::listen(SharedPool<OutgoingBuffer>* pool, Channel<u32>* channel) {
    struct io_uring_cqe* *cqes = reinterpret_cast<struct io_uring_cqe**>(calloc(entries*2, sizeof(struct io_uring_cqe*))); // possibly null, idc
                                                                                 
    while (true) {
        int ret = ring.wait();
        if (unlikely(ret == -EINTR))
            continue;
        if (unlikely(ret < 0)) {
            fprintf(stderr, "submit and wait failed %d\n", ret);
            break;
        }

        int count = ring.batch(cqes, entries*2);
        for (int i = 0; i < count; ++i) {
            if (unlikely(!ring.packet_check(cqes[i]))) continue;
            if (cqes[i]->res < 0) continue;

            if (cqes[i]->user_data > entries) {
                u32 idx = pool->Acquire();
                OutgoingBuffer* buf = &pool->data[idx];

                u32 sz = cqes[i]->res;
                memcpy(&buf->payload, ring.payload(cqes[i]->flags >> 16), sz);
                buf->len = sz;
                buf->idx = counter++;

                channel->push(idx);

                printf("Incoming packet: size %d\n", sz);
                fflush(stdout);
            } else {
                pool->Release(cqes[i]->user_data);
            }

            ring.packet_recycle(cqes[i]);
        }

        ring.advance_queue(count);
    }
}
