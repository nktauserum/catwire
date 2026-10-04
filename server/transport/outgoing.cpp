#include "../transport.hpp"

void Transport::Worker::Outgoing() {
    while (true) {
        u32 idx = *queue->read();
        queue->pop();

        __enqueue(idx);    
    }
}

void Transport::Worker::__enqueue(u32 idx) {
    
}
