# Lock-Free Ring Buffer vs Go Channels

A bounded lock-free queue built from scratch in Go, inspired by the LMAX Disruptor, benchmarked step by step against a buffered `chan`.

Final result: **~10 ns per item, ~25x faster than a buffered channel** for one producer and one consumer, with 0 allocations.

## Why a Channel Is Slow Here
A channel is a general tool. Every send and receive takes a lock inside `runtime.hchan`, and when the buffer is empty or full the goroutine is parked and later woken by the scheduler. A dedicated single-producer/single-consumer ring needs none of that:
- only the producer writes `tail` and only the consumer writes `head`, so one atomic store per operation is enough (no lock, no CAS)
- the capacity is a power of two, so `i & mask` replaces `i % cap`
- a waiting side spins briefly and then calls `runtime.Gosched()`; it never parks

## Optimization Journey

All numbers below: Intel i3-1005G1 (2 cores / 4 threads), `GOMAXPROCS=4`, capacity 1024, `int` items, median of 5 runs. ns/item is the time to move one item from producer to consumer.

### 1 Producer / 1 Consumer

| Version | What changed | ns/item | M items/s | vs chan |
| :--- | :--- | ---: | ---: | ---: |
| `chan` | buffered channel (baseline) | 239.8 | 4.2 | 1.00x |
| v0 `mutex.go` | ring + `sync.Mutex` + `sync.Cond` | 160.9 | 6.2 | 1.49x |
| v1 `spsc_naive.go` | lock-free, atomic head/tail | 146.1 | 6.8 | 1.64x |
| v2 `spsc_padded.go` | head and tail on separate cache lines | 197.7 | 5.1 | 1.21x |
| v3 `spsc_cached.go` | each side caches the other's index | 78.6 | 12.7 | 3.05x |
| v4 `spsc_batch.go` | publish 64 items per atomic store | **9.5** | **105.8** | **25.36x** |

On this laptop v1, v2 and v3 vary a lot between runs (roughly 75–250 ns), so their order relative to each other is not reliable. Every run lands `chan` between 240 and 300 ns and v4 between 9.5 and 10.7 ns.

### 4 Producers / 4 Consumers

A channel is multi-producer/multi-consumer, so the fair rival here is `mpmc.go`, Dmitry Vyukov's bounded MPMC queue. Each slot carries a sequence number that says whether it is ready to be written or read, so producers and consumers each need only one CAS.

| Version | ns/item | M items/s | vs chan |
| :--- | ---: | ---: | ---: |
| `chan` | 282.8 | 3.5 | 1.00x |
| v0 `mutex.go` | 391.5 | 2.6 | 0.72x |
| `mpmc.go` | 218.7 | 4.6 | 1.29x |

With 8 goroutines contending for the same two counters, the lock-free queue's advantage shrinks to ~1.3x.

## False Sharing
CPU caches move memory in 64-byte lines. If the producer's `tail` and the consumer's `head` sit in the same line, each store by one core invalidates the line in the other core's cache, even though the two never touch the same variable.

`falsesharing_test.go` isolates the effect: two goroutines each increment their own counter.

| Layout | ns/op |
| :--- | ---: |
| both counters in one cache line | ~66 |
| counters padded 64 bytes apart | ~25 |

That is **~2.6x from 56 bytes of padding**.

Padding alone did not show up in the queue (v1 → v2), because there the consumer runs right behind the producer, so every item's slot in `buf` also has to travel between cores. That is true sharing: padding cannot remove it, and batching (v4) is what amortizes it.

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

**Print the comparison tables:**
```bash
go run ./cmd/bench
```
