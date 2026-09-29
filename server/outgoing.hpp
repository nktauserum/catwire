#pragma once

#include <linux/if.h>

#include "io_uring.h"

#include "../common/types.h"
#include "../common/models.h"

#define TUN_QUEUE_COUNT 4

static bool f(struct io_uring*, struct msghdr*) {
    return true;
}

class Outgoing {
    Ring<f> ring;
    int fds[TUN_QUEUE_COUNT];
    char name[IFNAMSIZ];
public:
    bool init(const char*);
    void listen();
};
