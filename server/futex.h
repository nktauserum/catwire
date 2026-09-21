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
    std::atomic<u32> val = 0;

    inline int futex(int futex_op, u32 val, const struct timespec *timeout, u32 *uaddr2, u32 val3) {
        return syscall(SYS_futex, (u32*)&val, futex_op, val, timeout, uaddr2, val3);
    }

public:
    inline int wait(u32 expect_val) {
      return futex(FUTEX_WAIT_PRIVATE, expect_val, NULL, NULL, 0);
    }

    inline int wake() {
      return futex(FUTEX_WAKE_PRIVATE, 1, NULL, NULL, 0);
    }
};
