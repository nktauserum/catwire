#pragma once

#include <limits.h>
#include <linux/futex.h>
#include <stdint.h>
#include <stdlib.h>
#include <sys/syscall.h>
#include <sys/time.h>
#include <unistd.h>
#include <atomic>

#include "types.h"

struct Futex {
private:
    inline int futex(std::atomic<u32>* lock, int futex_op, u32 val, const struct timespec *timeout, u32 *uaddr2, u32 val3) {
        return syscall(SYS_futex, lock, futex_op, val, timeout, uaddr2, val3);
    }

public:
    inline int wait(std::atomic<u32>* lock, u32 expect_val) {
      return futex(lock, FUTEX_WAIT_PRIVATE, expect_val, NULL, NULL, 0);
    }

    inline int wake(std::atomic<u32>* lock) {
      return futex(lock, FUTEX_WAKE_PRIVATE, 1, NULL, NULL, 0);
    }
};
