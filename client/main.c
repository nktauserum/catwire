#include "config.h"

#include <stdio.h>

int main(int argc, char *argv[]) {
    if (argc != 2) return -1; // log ERROR + usage

    Config config;
    if (config_parse(&config, argv[1]) < 0) return -1;

    puts("Started successfully");
}
