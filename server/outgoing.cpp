#include "outgoing.hpp"

#include <fcntl.h>
#include <linux/if_tun.h>
#include <cstring>
#include <sys/ioctl.h>
#include <unistd.h>

bool Outgoing::init(const char* ifname) {
    int tun_fd, rc;
	size_t ifname_len;
	struct ifreq setiff_request;

    i16 flags = IFF_TUN | IFF_NO_PI | IFF_MULTI_QUEUE | IFF_VNET_HDR;

	if (ifname != NULL) {
		ifname_len = strlen(name);
		if (ifname_len >= IFNAMSIZ) {
			return false;
		}
	}

	tun_fd = open("/dev/net/tun", O_RDWR | O_CLOEXEC);
	if (tun_fd == -1) {
		return false;
	}

	memset(&setiff_request, 0, sizeof setiff_request);
	setiff_request.ifr_flags = flags;
	rc = ioctl(tun_fd, TUNSETIFF, &setiff_request);
	if (rc == -1) {
		close(tun_fd);
		return false;
	}

    fd = tun_fd;
    memcpy(name, setiff_request.ifr_name, IFNAMSIZ);

    return true;
}
