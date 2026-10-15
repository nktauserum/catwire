#include "tun.h"

#include <stdio.h>
#include <string.h>

#include <fcntl.h>
#include <sys/ioctl.h>
#include <linux/if.h>
#include <linux/if_tun.h>
#include <unistd.h>

#include "logs.h"

int tun_open(const char* ifname) {
	size_t ifname_len = strlen(ifname);
    if (ifname_len >= IFNAMSIZ) {
        log_error("tun_open(): too large interface name (>= 16)");
        return -1;
    }

    struct ifreq ifr = {0};
    ifr.ifr_flags = IFF_TUN | IFF_NO_PI;
    strncpy(ifr.ifr_name, ifname, ifname_len);

    int qfd = open("/dev/net/tun", O_RDWR | O_NONBLOCK);
    if (qfd == -1) {
        perror("tun_open(): open");
        log_error("tun_open(): error open interface");
        return -1;
    }

    int rc = ioctl(qfd, TUNSETIFF, &ifr);
    if (rc == -1) {
        perror("tun_open(): ioctl");
        log_error("tun_open(): error setup interface");
        close(qfd);
        return -1;
    }

    return qfd;
}
