#include "networking.h"
#include "types.h"

int main(void) {
    UDP udp_listener = UDP(); 
    bool ok = udp_listener.Setup();
    if (!ok) 
        return 1;

    udp_listener.Listen();

    return 0;
}
