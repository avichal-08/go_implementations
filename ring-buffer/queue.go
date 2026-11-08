package ringbuffer

import "runtime"

type Queue[T any] interface {
	Push(v T) // blocks while full
	Pop() T   // blocks while empty
}

const cacheLine = 64 // L1 cache line size on x86

// rounds n up to a power of two so we can index with i & mask instead of i % cap
func nextPow2(n int) uint64 {
	if n < 2 {
		return 2
	}
	p := uint64(1)
	for p < uint64(n) {
		p <<= 1
	}
	return p
}

// spins for a while then yields, the goroutine never parks
type backoff struct{ n int }

func (b *backoff) wait() {
	if b.n < 64 {
		b.n++
		return
	}
	runtime.Gosched()
}

// buffered go channel behind the same interface, used as the baseline
type Chan[T any] struct{ c chan T }

func NewChan[T any](capacity int) *Chan[T] { return &Chan[T]{c: make(chan T, nextPow2(capacity))} }
func (q *Chan[T]) Push(v T)                { q.c <- v }
func (q *Chan[T]) Pop() T                  { return <-q.c }
