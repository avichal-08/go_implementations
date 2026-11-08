package ringbuffer

import (
	"sync"
	"sync/atomic"
	"testing"
)

// two goroutines each bump their own counter, the only difference is whether the counters share a cache line

type sharedCounters struct {
	a, b atomic.Uint64
}

type paddedCounters struct {
	a atomic.Uint64
	_ [cacheLine - 8]byte
	b atomic.Uint64
	_ [cacheLine - 8]byte
}

func bumpBoth(n int, x, y *atomic.Uint64) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			x.Add(1)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			y.Add(1)
		}
	}()
	wg.Wait()
}

func BenchmarkFalseSharing(b *testing.B) {
	b.Run("same-line", func(b *testing.B) {
		var c sharedCounters
		bumpBoth(b.N, &c.a, &c.b)
	})
	b.Run("padded", func(b *testing.B) {
		var c paddedCounters
		bumpBoth(b.N, &c.a, &c.b)
	})
}
