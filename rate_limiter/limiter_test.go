package main

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 5)

	for i := 0; i < 5; i++ {
		if !rl.Allow() {
			t.Errorf("expected allow for initial burst, failed at %d", i)
		}
	}

	if rl.Allow() {
		t.Error("expected rejection when bucket is empty")
	}

	time.Sleep(105 * time.Millisecond)
	if !rl.Allow() {
		t.Error("expected allow after partial refill")
	}
	if rl.Allow() {
		t.Error("expected rejection after consuming the refilled token")
	}

	time.Sleep(550 * time.Millisecond)
	for i := 0; i < 5; i++ {
		if !rl.Allow() {
			t.Errorf("expected allow for burst after full refill, failed at %d", i)
		}
	}
	if rl.Allow() {
		t.Error("expected rejection after burst")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(100, 50)
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow() {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if successCount != 50 {
		t.Errorf("expected exactly 50 successful requests, got %d", successCount)
	}
}

func BenchmarkRateLimiter(b *testing.B) {
	rl := NewRateLimiter(1000, 100)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rl.Allow()
		}
	})
}
