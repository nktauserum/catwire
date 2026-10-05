#pragma once

#include "../common/models.h"
#include "routing.hpp"

struct Context {
    Channel<u32>*               incoming_channel, outgoing_channel;
    SharedPool<IncomingBuffer>* incoming_pool;
    SharedPool<OutgoingBuffer>* outgoing_pool;
    RoutingTable*               rtable;
};
