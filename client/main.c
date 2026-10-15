#include <stdio.h>

#include "tun.h"
#include "udp.h"

#include "config.h"
#include "logs.h"
#include "pool.h"
#include "channel.h"

#include <sys/epoll.h>

#define WORKERS_COUNT 4
static const char* tun_name = "cw2";

int main(int argc, char *argv[]) {
    log_info("Starting...");
    if (argc != 2) {
        log_error("Incorrect command-line arguments");
        return -1;
    }

    Config config;
    if (config_parse(&config, argv[1]) < 0) return -1;

    int tun_fd = tun_open(tun_name);
    if (tun_fd < 0) {
        return 1;
    }

    log_info("TUN-interface opened successfully");

    int udp_fd = udp_open();
    if (udp_fd < 0) {
        return 1;
    }

    log_info("UDP-socket    opened successfully");

    Pool* pool = calloc(1, sizeof(Pool)); // you can allocate it on the stack if you want
    if (!pool) {
        log_error("No memory available for buffer pool");
        return 1;
    }
    pool_init(pool);

    Channel ch;
    if (channel_init(&ch, WORKERS_COUNT) < 0) {
        log_error("Init channel");
        return 1;
    }

    // start workers here

    int epoll_fd = epoll_create1(0);
    struct epoll_event ev = {.events = EPOLLIN};

    if (epoll_ctl(epoll_fd, EPOLL_CTL_ADD, tun_fd, &ev) < 0) {
        perror("epoll_ctl");
        log_error("epoll_ctl(): error adding TUN fd");
        return 1;
    }

    if (epoll_ctl(epoll_fd, EPOLL_CTL_ADD, udp_fd, &ev) < 0) {
        perror("epoll_ctl");
        log_error("epoll_ctl(): error adding UDP fd");
        return 1;
    }

    while (1) {
        struct epoll_event events[64];
        int n = epoll_wait(epoll_fd, events, 64, -1);

        for (int i = 0; i < n; ++i) {
            if (events[i].data.fd == tun_fd) {
            } else if (events[i].data.fd == udp_fd) {
                u32 idx = pool_acquire(pool); 
                Buffer* buf = &pool->data[idx];

                socklen_t addr_len = (socklen_t)sizeof(buf->addr);
                ssize_t n = recvfrom(udp_fd, buf->payload, sizeof(buf->payload), 0, (struct sockaddr*)&buf->addr, &addr_len);
                if (n < 0) {
                    pool_release(pool, idx);
                    continue;
                }

                channel_push(&ch, idx);
            } else continue; // add graceful shutdown
        }
    }
}
