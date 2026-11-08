package ringbuffer

import (
	"testing"
	"unsafe"
)

func allSPSC() map[string]func(int) Queue[int] {
	return map[string]func(int) Queue[int]{
		"chan":   func(c int) Queue[int] { return NewChan[int](c) },
		"mutex":  func(c int) Queue[int] { return NewMutex[int](c) },
		"naive":  func(c int) Queue[int] { return NewSPSCNaive[int](c) },
		"padded": func(c int) Queue[int] { return NewSPSCPadded[int](c) },
		"cached": func(c int) Queue[int] { return NewSPSCCached[int](c) },
		"mpmc":   func(c int) Queue[int] { return NewMPMC[int](c) },
	}
}

func TestFIFOAndWraparound(t *testing.T) {
	// tiny ring forces lots of wraparounds and full/empty transitions
	for name, mk := range allSPSC() {
		t.Run(name, func(t *testing.T) {
			Run1P1C(mk(4), 200_000)
		})
	}
}

func TestSingleGoroutineFillDrain(t *testing.T) {
	for name, mk := range allSPSC() {
		if name == "chan" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			q := mk(8)
			for round := 0; round < 10; round++ {
				for i := 0; i < 8; i++ {
					q.Push(round*8 + i)
				}
				for i := 0; i < 8; i++ {
					if v := q.Pop(); v != round*8+i {
						t.Fatalf("got %d want %d", v, round*8+i)
					}
				}
			}
		})
	}
}

func TestBatch(t *testing.T) {
	for _, batch := range []int{1, 3, 7, 64, 1000} {
		RunBatch1P1C(NewSPSCCached[int](16), 200_000, batch)
	}
}

func TestMPMCNoLossNoDup(t *testing.T) {
	impls := map[string]Queue[int]{
		"mpmc":  NewMPMC[int](64),
		"mutex": NewMutex[int](64),
		"chan":  NewChan[int](64),
	}
	const n = 1 << 20
	for name, q := range impls {
		t.Run(name, func(t *testing.T) {
			count, sum := RunMPMC(q, n, 4, 4)
			want := int64(n) * int64(n-1) / 2
			if count != n || sum != want {
				t.Fatalf("count=%d sum=%d, want count=%d sum=%d", count, sum, n, want)
			}
		})
	}
}

// checks that padding actually separates head and tail
func TestPaddingLayout(t *testing.T) {
	var p SPSCPadded[int]
	if d := unsafe.Offsetof(p.tail) - unsafe.Offsetof(p.head); d < cacheLine {
		t.Fatalf("padded: head/tail only %d bytes apart", d)
	}
	var c SPSCCached[int]
	if d := unsafe.Offsetof(c.tail) - unsafe.Offsetof(c.head); d < cacheLine {
		t.Fatalf("cached: head/tail only %d bytes apart", d)
	}
	var n SPSCNaive[int]
	if d := unsafe.Offsetof(n.tail) - unsafe.Offsetof(n.head); d >= cacheLine {
		t.Fatalf("naive should share a line, got %d bytes apart", d)
	}
}
