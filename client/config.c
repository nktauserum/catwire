#include "config.h"

#include <stdlib.h>
#include <stdio.h>
#include <string.h>

#include "logs.h"

// server_addr handler
#include <arpa/inet.h>

// seed handler
#include "base64.h"

int config_parse(Config* config, const char* filename) {
    FILE* file = fopen(filename, "r");
    if (!file) {
        log_error("config_parse(): couldn't open file");
        return -1;
    }

    log_warning("config_parse(): missing fields are currently not being detected");

    char line[1024];
    while(fgets(line, 1024, file) != NULL) {
        size_t len = strlen(line);
        if (len == 0) continue;
        if (line[0] == '[') continue;

        const char* separator = strchr(line, '=');
        if (!separator) {
            log_error("config_parse(): couldn't find a separator");
            return -1;
        }
        
        size_t key_len = separator - line;
        if (strncmp(line, "seed", key_len) == 0) {
            size_t value_len = len - key_len - 4; // somehow it works
            // printf("len(%lu): %.*s\n", value_len, (int)value_len, separator+2);

            if (base64_decoded_len(value_len) != 34){
                log_error("config_parse(): seed: string have an incorrect size");
                return -1;
            }

            base64_decode(config->seed, separator+2, value_len);
        } else if (strncmp(line, "server_addr", key_len) == 0) {
            size_t value_len = len - key_len - 4; // somehow it works
            if (value_len >= 16) {
                log_error("config_parse(): server_addr: wrong size");
                return -1; 
            }

            char addr_buf[16];
            strncpy(addr_buf, separator+2, value_len);

            if (inet_pton(AF_INET, addr_buf, &config->server_addr) != 1) {
                log_error("config_parse(): server_addr: parsing error");
                return -1; 
            } 
        } else if (strncmp(line, "server_port", key_len) == 0) {
            config->server_port = strtol(separator+1, NULL, 10);
            if (config->server_port == 0) {
                log_error("config_parse(): server_port: error parsing integer");
                return -1; 
            } 
        } else {
            log_warning("config_parse(): unknown field");
            continue;
        }
    }

    return 0;
}
