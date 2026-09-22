#pragma once

#include "../types.h"
#include "packet.h"

class Handler {
public:
    virtual ~Handler() = default;
    virtual void incomingRecvCallback(UDPPacket) = 0;
    virtual void incomingSendCallback(u32) = 0;
};
