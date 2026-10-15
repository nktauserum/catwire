#include "udp.h"

#include <stdio.h>
#include <string.h>

#include <sys/socket.h>
#include <netinet/in.h>
#include "logs.h"

int udp_open() {
    int fd = socket(AF_INET, SOCK_DGRAM, 0);
    if (fd < 0) {
        perror("udp_open(): socket");
        log_error("udp_open(): error constructing socket");
        return -1;
    }

    struct sockaddr_in addr;
    memset(&addr, 0, sizeof(addr));
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = htonl(INADDR_ANY);
    addr.sin_port = 0;

    if (bind(fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        perror("udp_open(): bind");
        log_error("udp_open(): error binding socket");
        return -1;
    }

    return fd;
}
