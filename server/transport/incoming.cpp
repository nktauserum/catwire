#include "incoming.hpp"

#include <vector>

#include <liburing.h>

void Incoming::Worker::Start() {
    std::vector<struct io_uring_cqe*> cqes(entries*2);

    while (true) {
        
    }
}
