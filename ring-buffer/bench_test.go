package ringbuffer

import (
	"strconv"
	"testing"
)

const benchCap = 1024

// ns/op = cost of moving one item from producer to consumer

func Benchmark1P1C(b *testing.B) {
	for _, name := range []string{"chan", "mutex", "naive", "padded", "cached", "mpmc"} {
		mk := allSPSC()[name]
		b.Run(name, func(b *testing.B) {
			q := mk(benchCap)
			b.ReportAllocs()
			b.ResetTimer()
			Run1P1C(q, b.N)
		})
	}
	for _, batch := range []int{16, 64, 256} {
		b.Run("batch"+strconv.Itoa(batch), func(b *testing.B) {
			q := NewSPSCCached[int](benchCap)
			b.ReportAllocs()
			b.ResetTimer()
			RunBatch1P1C(q, b.N, batch)
		})
	}
}

func Benchmark4P4C(b *testing.B) {
	impls := []struct {
		name string
		mk   func() Queue[int]
	}{
		{"chan", func() Queue[int] { return NewChan[int](benchCap) }},
		{"mutex", func() Queue[int] { return NewMutex[int](benchCap) }},
		{"mpmc", func() Queue[int] { return NewMPMC[int](benchCap) }},
	}
	for _, im := range impls {
		b.Run(im.name, func(b *testing.B) {
			q := im.mk()
			b.ReportAllocs()
			b.ResetTimer()
			RunMPMC(q, max(b.N, 16), 4, 4)
		})
	}
}
