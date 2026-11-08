package main

import (
	"fmt"
	"runtime"
	"sort"
	"testing"

	rb "ringbuffer"
)

const capacity = 1024
const runs = 5

type bench struct {
	name string
	fn   func(n int)
}

// runs the benchmark several times and returns median ns per item
func median(fn func(n int)) float64 {
	res := make([]float64, runs)
	for i := range res {
		r := testing.Benchmark(func(b *testing.B) { fn(b.N) })
		res[i] = float64(r.T.Nanoseconds()) / float64(r.N)
	}
	sort.Float64s(res)
	return res[runs/2]
}

func table(title string, benches []bench) {
	fmt.Printf("\n### %s\n\n", title)
	fmt.Println("| Queue | ns/item | M items/s | vs chan |")
	fmt.Println("| :--- | ---: | ---: | ---: |")
	var base float64
	for i, b := range benches {
		ns := median(b.fn)
		if i == 0 {
			base = ns
		}
		fmt.Printf("| %s | %.1f | %.1f | %.2fx |\n", b.name, ns, 1e3/ns, base/ns)
	}
}

func main() {
	fmt.Printf("GOMAXPROCS=%d, cap=%d, median of %d runs\n", runtime.GOMAXPROCS(0), capacity, runs)

	table("1 producer / 1 consumer", []bench{
		{"chan", func(n int) { rb.Run1P1C(rb.NewChan[int](capacity), n) }},
		{"v0 mutex", func(n int) { rb.Run1P1C(rb.NewMutex[int](capacity), n) }},
		{"v1 spsc naive", func(n int) { rb.Run1P1C(rb.NewSPSCNaive[int](capacity), n) }},
		{"v2 spsc padded", func(n int) { rb.Run1P1C(rb.NewSPSCPadded[int](capacity), n) }},
		{"v3 spsc cached", func(n int) { rb.Run1P1C(rb.NewSPSCCached[int](capacity), n) }},
		{"v4 spsc batch 64", func(n int) { rb.RunBatch1P1C(rb.NewSPSCCached[int](capacity), n, 64) }},
	})

	table("4 producers / 4 consumers", []bench{
		{"chan", func(n int) { rb.RunMPMC(rb.NewChan[int](capacity), max(n, 16), 4, 4) }},
		{"v0 mutex", func(n int) { rb.RunMPMC(rb.NewMutex[int](capacity), max(n, 16), 4, 4) }},
		{"mpmc", func(n int) { rb.RunMPMC(rb.NewMPMC[int](capacity), max(n, 16), 4, 4) }},
	})
}
