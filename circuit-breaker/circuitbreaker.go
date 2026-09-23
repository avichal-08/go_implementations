package main

import (
	"errors"
	"sync"
	"time"
)

type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

var ErrCircuitOpen = errors.New("circuit breaker is open")

type CircuitBreaker struct {
	mu               sync.Mutex
	state            State
	failureThreshold uint32
	consecutiveFails uint32
	cooldown         time.Duration
	expiry           time.Time
}

func NewCircuitBreaker(threshold uint32, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: threshold,
		cooldown:         cooldown,
	}
}

func (cb *CircuitBreaker) Execute(req func() error) error {
	cb.mu.Lock()

	now := time.Now()

	if cb.state == StateOpen {
		if now.After(cb.expiry) {
			cb.state = StateHalfOpen
		} else {
			cb.mu.Unlock()
			return ErrCircuitOpen
		}
	} else if cb.state == StateHalfOpen {
		cb.mu.Unlock()
		return ErrCircuitOpen
	}

	currentState := cb.state
	cb.mu.Unlock()

	err := req()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.consecutiveFails++
		if currentState == StateHalfOpen || cb.consecutiveFails >= cb.failureThreshold {
			cb.state = StateOpen
			cb.expiry = time.Now().Add(cb.cooldown)
		}
	} else {
		if currentState == StateHalfOpen {
			cb.state = StateClosed
		}
		cb.consecutiveFails = 0
	}

	return err
}

func (cb *CircuitBreaker) StateString() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF-OPEN"
	default:
		return "CLOSED"
	}
}
