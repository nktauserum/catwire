#include "logs.h"

#include <stdio.h>

static void log_str(const char* str) {
    puts(str);
}

void log_info(const char* str) {
    printf("[INFO]: ");
    log_str(str);
}

void log_warning(const char* str) {
    printf("[WARNING]: ");
    log_str(str);

}

void log_error(const char* str) {
    printf("[ERROR]: ");
    log_str(str);
}

