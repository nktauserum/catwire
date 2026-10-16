#pragma once

#include <stdlib.h>
#include <stdint.h>

#include "../../common/types.h"

size_t base64_decode(u8* dst, const u8* src, size_t len);
size_t base64_decoded_len(size_t length);
