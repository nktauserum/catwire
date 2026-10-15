#include "tun.h"
#include "udp.h"

#include "config.h"
#include "logs.h"

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

    int udp_fd = udp_open();
    if (udp_fd < 0) {
        return 1;
    }
}
