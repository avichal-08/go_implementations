package ringbuffer

import (
	"fmt"
	"sync"
)

// one producer pushes 0..n-1, the caller pops and checks fifo order
func Run1P1C(q Queue[int], n int) {
	done := make(chan struct{})
	go func() {
		for i := 0; i < n; i++ {
			q.Push(i)
		}
		close(done)
	}()
	for i := 0; i < n; i++ {
		if v := q.Pop(); v != i {
			panic(fmt.Sprintf("order violated: got %d want %d", v, i))
		}
	}
	<-done
}

// same as Run1P1C but moves items in batches
func RunBatch1P1C(q *SPSCCached[int], n, batch int) {
	done := make(chan struct{})
	go func() {
		in := make([]int, batch)
		for i := 0; i < n; {
			m := min(batch, n-i)
			for j := 0; j < m; j++ {
				in[j] = i + j
			}
			for off := 0; off < m; {
				off += q.PushBatch(in[off:m])
			}
			i += m
		}
		close(done)
	}()
	out := make([]int, batch)
	for i := 0; i < n; {
		k := q.PopBatch(out[:min(batch, n-i)])
		for j := 0; j < k; j++ {
			if out[j] != i+j {
				panic(fmt.Sprintf("order violated: got %d want %d", out[j], i+j))
			}
		}
		i += k
	}
	<-done
}

// p producers and c consumers move n items, returns count and sum to detect loss or duplicates
func RunMPMC(q Queue[int], n, p, c int) (count int, sum int64) {
	per := n / (p * c)
	total := per * p * c
	var wg sync.WaitGroup
	for i := 0; i < p; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < total/p; j++ {
				q.Push(base + j)
			}
		}(i * (total / p))
	}
	counts := make([]int, c)
	sums := make([]int64, c)
	var cw sync.WaitGroup
	for i := 0; i < c; i++ {
		cw.Add(1)
		go func(id int) {
			defer cw.Done()
			// locals, since adjacent slice elements would false share
			var cnt int
			var s int64
			for j := 0; j < total/c; j++ {
				s += int64(q.Pop())
				cnt++
			}
			counts[id], sums[id] = cnt, s
		}(i)
	}
	wg.Wait()
	cw.Wait()
	for i := range counts {
		count += counts[i]
		sum += sums[i]
	}
	return count, sum
}
