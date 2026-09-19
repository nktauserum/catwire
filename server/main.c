#include <stdio.h>
#include <stdbool.h>
#include <string.h>
#include <netinet/udp.h>
#include <arpa/inet.h>
#include <unistd.h>
#include <liburing.h>

#define UDP_PORT 45230

typedef struct {
    
} ctx;

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

int main(void) {
    int udpsock = setup_udp(UDP_PORT);
    if (udpsock < 0) {
        return 1;
    }

    

    return 0;
}
