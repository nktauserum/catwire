package models

import "sync/atomic"

const queueSize = 256

const (
	Q_OK uint32 = iota
	Q_ESIZE
	Q_ECLOSED
)

type Queue[T any] struct {
	queue [queueSize]T

	head       uint32
	_          [60]byte
	tail       uint32
	_          [60]byte
	tail_cache uint32
	_          [60]byte
	waiting    uint32
	_          [60]byte
	closed     uint32
}

func (q *Queue[T]) Push(item T) bool { // `error` is too heavy
	head := q.head // atomic.LoadUint32(&q.head)
	if head - q.tail_cache == queueSize {
		q.tail_cache = atomic.LoadUint32(&q.tail)

		if head - q.tail_cache == queueSize {
			if atomic.LoadUint32(&q.closed) == 1 {
				return false
			}
			// wait and smth
		}
	}

	q.queue[(head+1) % queueSize] = item

	atomic.AddUint32(&q.head, 1)
	return true
}

func (q *Queue[T]) Read() (T, bool) {
	tail := q.tail
	head := atomic.LoadUint32(&q.head)

	if tail == head {
		if atomic.LoadUint32(&q.closed) == 1 {
			var empty T
			return empty, false
		}
		// wait
	}

	item := q.queue[(tail+1) % queueSize]
	atomic.AddUint32(&q.tail, 1)
	return item, true
}

func (q *Queue[T]) wait() {}
func (q *Queue[T]) wake() {}

/*

int queue_write(Queue* q, cont_t* values, size_t size) {
    u32 head = __atomic_load_n(&q->head, __ATOMIC_RELAXED);
    if (head - q->tail_cache == CW_QUEUE_SIZE) {
        q->tail_cache = __atomic_load_n(&q->tail, __ATOMIC_ACQUIRE);
        if (unlikely(head - q->tail_cache == CW_QUEUE_SIZE))
            return -Q_ESIZE;
    }

    u32 available = CW_QUEUE_SIZE - (head - q->tail_cache);
    u32 pending = size <= available ? size : available;

    for (u32 i = 0; i < pending; ++i)
        q->queue[(head+i) % CW_QUEUE_SIZE] = values[i];

    __atomic_store_n(&q->head, head + pending, __ATOMIC_RELEASE);

    return pending;
}

int queue_read(Queue* q, cont_t* buf, size_t cap) {
    u32 tail = __atomic_load_n(&q->tail, __ATOMIC_RELAXED);
    u32 head = __atomic_load_n(&q->head, __ATOMIC_ACQUIRE);

    if (tail == head) {
        if (unlikely(__atomic_load_n(&q->closed, __ATOMIC_ACQUIRE)))
            return -Q_ECLOSED;
        return -Q_ESIZE;
    }

    u32 pending = (head - tail) <= cap ? (head - tail) : cap;
    for (u32 i = 0; i < pending; ++i)
        buf[i] = q->queue[(tail+i) % CW_QUEUE_SIZE];

    __atomic_store_n(&q->tail, tail + pending, __ATOMIC_RELEASE);

    return pending;
}


*/
