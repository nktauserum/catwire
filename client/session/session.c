#include "session.h"

bool session_available(Session* session) {
    return __atomic_load_n(&session->available, __ATOMIC_ACQUIRE) != 0;
}

void session_set_available(Session* session) {
    __atomic_store_n(&session->available, 1, __ATOMIC_RELEASE);
}
