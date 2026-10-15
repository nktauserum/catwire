#include "config.h"
#include "logs.h"

int main(int argc, char *argv[]) {
    log_info("Starting...");
    if (argc != 2) {
        log_error("Incorrect command-line arguments");
        return -1;
    }

    Config config;
    if (config_parse(&config, argv[1]) < 0) return -1;
}
