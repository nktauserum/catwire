#include "config.h"

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

// server_addr handler
#include <arpa/inet.h>

// seed handler
#include "base64.h"

int config_parse(Config* config, const char* filename) {
    FILE* file = fopen(filename, "r");
    if (!file) return -1;

    char line[1024];
    while(fgets(line, 1024, file) != NULL) {
        size_t len = strlen(line);
        if (len == 0) continue;
        if (line[0] == '[') continue;

        const char* separator = strchr(line, '=');
        if (!separator) return -1; // log ERROR
        
        size_t key_len = separator - line;
        if (strncmp(line, "seed", key_len) == 0) {
            size_t value_len = len - key_len - 1;
            if (base64_decoded_len(value_len) != 32) return -1; // log ERROR
            base64_decode(config->seed, separator+1, value_len);
        } else if (strncmp(line, "server_addr", key_len) == 0) {
            if (inet_pton(AF_INET, separator+1, &config->server_addr) != 1) return -1; // log ERROR
        } else if (strncmp(line, "server_port", key_len) == 0) {
            config->server_port = strtol(separator+1, NULL, 10);
            if (config->server_port == 0) return -1; // log ERROR
        } else continue; // log WARNING
    }

    return 0;
}
