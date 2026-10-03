#pragma once

#include <stdexcept>
#include <iostream>
#include <sstream>
#include <iomanip>

#define likely(x)    __builtin_expect(!!(x), 1)
#define unlikely(x)  __builtin_expect(!!(x), 0)

#define panic(s) std::runtime_error(s)

#define print_hex(arr, size) do { \
    std::stringstream ss; \
    for (size_t i = 0; i < size; ++i) { \
        ss << std::hex << std::setw(2) << std::setfill('0') << (int)arr[i] << " "; \
    } \
    std::cout << ss.str() << std::endl; \
} while (0);
