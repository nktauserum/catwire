#pragma once

#include "../../common/models.h"

#include "../models/pool.h"
#include "config.hpp"

#include "../routing.hpp"

struct Context {
    SharedPool<IncomingBuffer>* incoming_pool;
    SharedPool<OutgoingBuffer>* outgoing_pool;
    RoutingTable*               rtable;

    Context(Config config) :
        incoming_pool{new SharedPool<IncomingBuffer>()},
        outgoing_pool{new SharedPool<OutgoingBuffer>()},
        rtable{new RoutingTable(config.seed, config.clients)}
    {}
};
