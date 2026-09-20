#define TESTING

#include "../alloc.h"

typedef struct {
    int data[64];
} testValue;

int main(void) {
    auto pool = EventPool<testValue>();
    for (int i = 0; i < 63; i++) {
        printf("idx: %d\n", pool.Acquire());
    }

    pool.Release(0);
    printf("idx: %d\n", pool.Acquire());

}
