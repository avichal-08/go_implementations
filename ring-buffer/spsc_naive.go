package ringbuffer

import "sync/atomic"

// v1: lock-free single producer single consumer ring
// head and tail share one cache line, so every store by one core invalidates it on the other (false sharing)
type SPSCNaive[T any] struct {
	buf  []T
	mask uint64
	head atomic.Uint64 // next slot to read, written only by consumer
	tail atomic.Uint64 // next slot to write, written only by producer
}

func NewSPSCNaive[T any](capacity int) *SPSCNaive[T] {
	n := nextPow2(capacity)
	return &SPSCNaive[T]{buf: make([]T, n), mask: n - 1}
}

func (q *SPSCNaive[T]) Push(v T) {
	t := q.tail.Load()
	var b backoff
	for t-q.head.Load() == uint64(len(q.buf)) {
		b.wait()
	}
	q.buf[t&q.mask] = v
	q.tail.Store(t + 1) // publishes the slot to the consumer
}

func (q *SPSCNaive[T]) Pop() T {
	h := q.head.Load()
	var b backoff
	for h == q.tail.Load() {
		b.wait()
	}
	v := q.buf[h&q.mask]
	q.head.Store(h + 1) // frees the slot for the producer
	return v
}
