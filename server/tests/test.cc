#define TESTING

#include "../alloc.h"

typedef struct {
    int data[64];
} testValue;

int main(void) {
    auto pool = EventPool<testValue>();
    printf("idx: %d\n", pool.Acquire());
    printf("idx: %d\n", pool.Acquire());
}
