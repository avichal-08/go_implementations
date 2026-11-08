# Lock-Free Ring Buffer vs Go Channels

A bounded lock-free queue built from scratch in Go, inspired by the LMAX Disruptor, benchmarked step by step against a buffered `chan`.

Final result: **~7.5 ns per item, ~29x faster than a buffered channel** for one producer and one consumer, with 0 allocations.

## Why a Channel Is Slow Here
A channel is a general tool. Every send and receive takes a lock inside `runtime.hchan`, and when the buffer is empty or full the goroutine is parked and later woken by the scheduler. A dedicated single-producer/single-consumer ring needs none of that:
- only the producer writes `tail` and only the consumer writes `head`, so one atomic store per operation is enough (no lock, no CAS)
- the capacity is a power of two, so `i & mask` replaces `i % cap`
- a waiting side spins briefly and then calls `runtime.Gosched()`; it never parks

## Optimization Journey

All numbers below come from `go test -run x -bench . -benchmem` on an Intel i3-1005G1 (2 cores / 4 threads), `GOMAXPROCS=4`, capacity 1024, `int` items. ns/op is the time to move one item from producer to consumer. Every version does 0 B/op and 0 allocs/op.

### 1 Producer / 1 Consumer

| Version | What changed | ns/op | vs chan |
| :--- | :--- | ---: | ---: |
| `chan` | buffered channel (baseline) | 222.2 | 1.0x |
| v0 `mutex.go` | ring + `sync.Mutex` + `sync.Cond` | 149.5 | 1.5x |
| v1 `spsc_naive.go` | lock-free, atomic head/tail | 139.2 | 1.6x |
| v2 `spsc_padded.go` | head and tail on separate cache lines | 99.97 | 2.2x |
| v3 `spsc_cached.go` | each side caches the other's index | 66.29 | 3.4x |
| v4 `spsc_batch.go` | publish 16 items per atomic store | 10.90 | 20x |
| v4 `spsc_batch.go` | publish 64 items per atomic store | 8.47 | 26x |
| v4 `spsc_batch.go` | publish 256 items per atomic store | **7.54** | **29x** |

On this laptop v1, v2 and v3 vary between runs and can swap order. The batched versions stay consistently 25x+ faster than `chan`.

### 4 Producers / 4 Consumers

A channel is multi-producer/multi-consumer, so the fair rival here is `mpmc.go`, Dmitry Vyukov's bounded MPMC queue. Each slot carries a sequence number that says whether it is ready to be written or read, so producers and consumers each need only one CAS.

| Version | ns/op | vs chan |
| :--- | ---: | ---: |
| `chan` | 274.9 | 1.0x |
| v0 `mutex.go` | 390.6 | 0.7x |
| `mpmc.go` | 239.1 | 1.15x |

With 8 goroutines contending for the same two counters, the lock-free queue's advantage almost disappears.

## False Sharing
CPU caches move memory in 64-byte lines. If the producer's `tail` and the consumer's `head` sit in the same line, each store by one core invalidates the line in the other core's cache, even though the two never touch the same variable.

`falsesharing_test.go` isolates the effect: two goroutines each increment their own counter.

| Layout | ns/op |
| :--- | ---: |
| both counters in one cache line | 62.92 |
| counters padded 64 bytes apart | 22.05 |

That is **~2.9x from 56 bytes of padding**.

In the queue, padding gives a smaller gain (v1 → v2), because the consumer runs right behind the producer, so every item's slot in `buf` also has to travel between cores. That is true sharing: padding cannot remove it, and batching (v4) is what amortizes it.

## Tradeoffs
- A waiting goroutine burns CPU while it spins; a channel parks it
- SPSC versions are only correct with exactly one producer and one consumer
- Fixed capacity, no `close`, no `select`
- For most Go code a channel is still the right choice. This is for hot paths that move millions of items per second between two goroutines.

## How to Run

**Run the tests:**
```bash
go test ./...
```

**Run the tests with the race detector (needs cgo/gcc):**
```bash
go test -race ./...
```

**Run the benchmarks:**
```bash
go test -run x -bench . -benchmem
```

**Print the comparison tables (median of 5 runs):**
```bash
go run ./cmd/bench
```
