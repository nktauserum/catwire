#pragma once

#include <vector>

#include "../../common/types.h"
#include "../../common/models.h"

#include "../models/channel.h"
#include "../models/pool.h"
#include "config.hpp"

#include "../routing.hpp"
#include "../interface.hpp"

struct Context {
    Channel<u32>               *incoming_channel, *outgoing_channel;
    SharedPool<IncomingBuffer>* incoming_pool;
    SharedPool<OutgoingBuffer>* outgoing_pool;
    RoutingTable*               rtable;

    Context(Config config) :
        incoming_channel{new Channel<u32>(config.num_cores)},
        outgoing_channel{new Channel<u32>(config.num_cores)},
        incoming_pool{new SharedPool<IncomingBuffer>()},
        outgoing_pool{new SharedPool<OutgoingBuffer>()},
        rtable{new RoutingTable(config.seed, config.clients)}
    {}
};
