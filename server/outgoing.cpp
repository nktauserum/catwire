#include "outgoing.hpp"

#include <fcntl.h>
#include <linux/if_tun.h>
#include <cstring>
#include <sys/ioctl.h>
#include <unistd.h>

#define QUEUE_ENTRIES 256
#define BUFFER_SIZE 65535

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

    if (!ring.init(QUEUE_ENTRIES, BUFFER_SIZE)) 
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
