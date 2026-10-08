#pragma once

#include "../../common/pool.h"
#include "../models/buffer.h"
#include "config.hpp"

#include "../routing.hpp"

struct Context {
    SharedPool<TransportBuffer>* transport_pool;
    SharedPool<TunnelBuffer>*    tunnel_pool;
    RoutingTable*                rtable;

    Context(Config config) :
        transport_pool{new SharedPool<TransportBuffer>()},
        tunnel_pool   {new SharedPool<TunnelBuffer>()},
        rtable        {new RoutingTable(config.seed, config.clients)}
    {}
};
