package ringbuffer

import "sync/atomic"

// v2: same as v1 but head and tail are padded onto separate cache lines
type SPSCPadded[T any] struct {
	buf  []T // read only after init, fine to share
	mask uint64
	_    [cacheLine]byte
	head atomic.Uint64
	_    [cacheLine - 8]byte
	tail atomic.Uint64
	_    [cacheLine - 8]byte
}

func NewSPSCPadded[T any](capacity int) *SPSCPadded[T] {
	n := nextPow2(capacity)
	return &SPSCPadded[T]{buf: make([]T, n), mask: n - 1}
}

func (q *SPSCPadded[T]) Push(v T) {
	t := q.tail.Load()
	var b backoff
	for t-q.head.Load() == uint64(len(q.buf)) {
		b.wait()
	}
	q.buf[t&q.mask] = v
	q.tail.Store(t + 1)
}

func (q *SPSCPadded[T]) Pop() T {
	h := q.head.Load()
	var b backoff
	for h == q.tail.Load() {
		b.wait()
	}
	v := q.buf[h&q.mask]
	q.head.Store(h + 1)
	return v
}
