package main

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu         sync.Mutex
	rate       float64
	capacity   float64
	tokens     float64
	lastUpdate time.Time
}

func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		capacity:   float64(burst),
		tokens:     float64(burst),
		lastUpdate: time.Now(),
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	elapsed := now.Sub(rl.lastUpdate).Seconds()

	rl.tokens += elapsed * rl.rate

	if rl.tokens > rl.capacity {
		rl.tokens = rl.capacity
	}

	rl.lastUpdate = now

	if rl.tokens >= 1.0 {
		rl.tokens--
		return true
	}

	return false
}
