package ringbuffer

import "sync"

// v0: ring buffer guarded by a single mutex, safe for any number of producers and consumers
type Mutex[T any] struct {
	mu       sync.Mutex
	notFull  sync.Cond
	notEmpty sync.Cond
	buf      []T
	mask     uint64
	head     uint64
	tail     uint64
}

func NewMutex[T any](capacity int) *Mutex[T] {
	n := nextPow2(capacity)
	q := &Mutex[T]{buf: make([]T, n), mask: n - 1}
	q.notFull.L = &q.mu
	q.notEmpty.L = &q.mu
	return q
}

func (q *Mutex[T]) Push(v T) {
	q.mu.Lock()
	for q.tail-q.head == uint64(len(q.buf)) {
		q.notFull.Wait()
	}
	q.buf[q.tail&q.mask] = v
	q.tail++
	q.mu.Unlock()
	q.notEmpty.Signal()
}

func (q *Mutex[T]) Pop() T {
	q.mu.Lock()
	for q.tail == q.head {
		q.notEmpty.Wait()
	}
	v := q.buf[q.head&q.mask]
	var zero T
	q.buf[q.head&q.mask] = zero
	q.head++
	q.mu.Unlock()
	q.notFull.Signal()
	return v
}
