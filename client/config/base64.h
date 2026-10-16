#pragma once

#include <stdlib.h>
#include <stdint.h>

size_t base64_decode(char* dst, const char* src, size_t len);
size_t base64_decoded_len(size_t length);
