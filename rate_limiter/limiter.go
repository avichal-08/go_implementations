package main

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu       sync.Mutex
	capacity float64
	tokens   float64
	stopCh   chan struct{}
}

func NewRateLimiter(rate float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		capacity: float64(burst),
		tokens:   float64(burst),
		stopCh:   make(chan struct{}),
	}

	interval := time.Duration(float64(time.Second) / rate)
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				rl.mu.Lock()
				if rl.tokens < rl.capacity {
					rl.tokens++
				}
				rl.mu.Unlock()
			case <-rl.stopCh:
				ticker.Stop()
				return
			}
		}
	}()

	return rl
}

func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.tokens >= 1.0 {
		rl.tokens--
		return true
	}

	return false
}
