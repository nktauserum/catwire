#pragma once

#include <stdint.h>

int futex_wait(uint32_t*);
int futex_wake(uint32_t*); 
