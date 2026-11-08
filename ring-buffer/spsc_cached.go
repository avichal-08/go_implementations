package ringbuffer

import "sync/atomic"

// v3: each side keeps a stale local copy of the other side's index
// and only reloads it when the copy says the ring is full/empty, so most ops never read the other core's line
type SPSCCached[T any] struct {
	buf  []T
	mask uint64
	_    [cacheLine]byte

	head      atomic.Uint64
	tailCache uint64 // consumer's last seen tail
	_         [cacheLine - 16]byte

	tail      atomic.Uint64
	headCache uint64 // producer's last seen head
	_         [cacheLine - 16]byte
}

func NewSPSCCached[T any](capacity int) *SPSCCached[T] {
	n := nextPow2(capacity)
	return &SPSCCached[T]{buf: make([]T, n), mask: n - 1}
}

func (q *SPSCCached[T]) Push(v T) {
	t := q.tail.Load()
	if t-q.headCache == uint64(len(q.buf)) {
		var b backoff
		for {
			q.headCache = q.head.Load()
			if t-q.headCache != uint64(len(q.buf)) {
				break
			}
			b.wait()
		}
	}
	q.buf[t&q.mask] = v
	q.tail.Store(t + 1)
}

func (q *SPSCCached[T]) Pop() T {
	h := q.head.Load()
	if h == q.tailCache {
		var b backoff
		for {
			q.tailCache = q.tail.Load()
			if h != q.tailCache {
				break
			}
			b.wait()
		}
	}
	v := q.buf[h&q.mask]
	q.head.Store(h + 1)
	return v
}
