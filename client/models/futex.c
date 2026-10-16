#include "futex.h"

#define _GNU_SOURCE

#include <unistd.h>
#include <linux/futex.h>
#include <sys/syscall.h>
#include <sys/time.h>

static int futex(uint32_t* lock, int futex_op, uint32_t val, const struct timespec *timeout, uint32_t *uaddr2, uint32_t val3) {
    return syscall(SYS_futex, lock, futex_op, val, timeout, uaddr2, val3);
}


int futex_wait(uint32_t* lock) {
    return futex(lock, FUTEX_WAIT_PRIVATE, 1, NULL, NULL, 0);
}

int futex_wake(uint32_t* lock) {
    return futex(lock, FUTEX_WAKE_PRIVATE, 1, NULL, NULL, 0);
}
