#pragma once

#include <stdlib.h>

#include "../../common/types.h"

int udp_open();
int udp_send(int, const u8*, size_t, Address);
