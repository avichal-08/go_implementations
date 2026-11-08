package ringbuffer

import "sync/atomic"

// vyukov's bounded multi producer multi consumer queue, the fair rival to a go channel
// each slot has a sequence number: seq == pos means empty, seq == pos+1 means full
type MPMC[T any] struct {
	buf  []cell[T]
	mask uint64
	_    [cacheLine]byte
	enq  atomic.Uint64
	_    [cacheLine - 8]byte
	deq  atomic.Uint64
	_    [cacheLine - 8]byte
}

type cell[T any] struct {
	seq atomic.Uint64
	val T
}

func NewMPMC[T any](capacity int) *MPMC[T] {
	n := nextPow2(capacity)
	q := &MPMC[T]{buf: make([]cell[T], n), mask: n - 1}
	for i := range q.buf {
		q.buf[i].seq.Store(uint64(i))
	}
	return q
}

func (q *MPMC[T]) Push(v T) {
	var b backoff
	for {
		pos := q.enq.Load()
		c := &q.buf[pos&q.mask]
		diff := int64(c.seq.Load()) - int64(pos)
		switch {
		case diff == 0:
			if q.enq.CompareAndSwap(pos, pos+1) {
				c.val = v
				c.seq.Store(pos + 1) // hands the slot to consumers
				return
			}
		case diff < 0:
			b.wait() // full
		}
		// diff > 0 means another producer took pos, retry
	}
}

func (q *MPMC[T]) Pop() T {
	var b backoff
	for {
		pos := q.deq.Load()
		c := &q.buf[pos&q.mask]
		diff := int64(c.seq.Load()) - int64(pos+1)
		switch {
		case diff == 0:
			if q.deq.CompareAndSwap(pos, pos+1) {
				v := c.val
				var zero T
				c.val = zero
				c.seq.Store(pos + q.mask + 1) // hands the slot back to producers for the next lap
				return v
			}
		case diff < 0:
			b.wait() // empty
		}
	}
}
