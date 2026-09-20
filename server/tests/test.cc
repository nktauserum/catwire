#define TESTING

#include "../alloc.h"

typedef struct {
    int data[64];
} testValue;

int main(void) {
    auto pool = EventPool<testValue>();
}
