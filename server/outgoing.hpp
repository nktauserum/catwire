#pragma once

#include <linux/if.h>

#include "../common/types.h"
#include "../common/models.h"

class Outgoing {
    int fd = -1;
    char name[IFNAMSIZ];
public:
    bool init(const char*);
    void listen();
};
