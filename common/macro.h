#pragma once

#include <stdexcept>

#define likely(x)    __builtin_expect(!!(x), 1)
#define unlikely(x)  __builtin_expect(!!(x), 0)

#define panic(s) std::runtime_error(s)
