#pragma once

#include <stdlib.h>

#include "../../common/types.h"

int tun_open(const char*);
int tun_write(int, const u8*, size_t);
