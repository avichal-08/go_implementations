package main

import (
	"errors"
	"fmt"
	"time"
)

func main() {
	cb := NewCircuitBreaker(3, 2*time.Second)

	requestCount := 0
	callService := func() error {
		requestCount++
		if requestCount <= 5 {
			return errors.New("service unavailable")
		}
		return nil
	}

	for i := 1; i <= 20; i++ {
		prevState := cb.StateString()

		err := cb.Execute(callService)

		currState := cb.StateString()

		if err != nil {
			if err == ErrCircuitOpen {
				fmt.Printf("req %d | %s | REJECTED (fast fail)\n", i, currState)
			} else {
				fmt.Printf("req %d | %s | FAILED (service error)\n", i, currState)
			}
		} else {
			fmt.Printf("req %d | %s | SUCCESS\n", i, currState)
		}

		if prevState != currState {
			fmt.Printf("> STATE TRANSITION: %s -> %s <\n", prevState, currState)
		}

		time.Sleep(500 * time.Millisecond)
	}
}
