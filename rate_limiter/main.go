package main

import (
	"fmt"
	"time"
)

func main() {
	limiter := NewRateLimiter(5, 10)

	fmt.Println("sending initial burst of 15 requests")
	for i := 1; i <= 15; i++ {
		if limiter.Allow() {
			fmt.Printf("request %2d: ALLOWED\n", i)
		} else {
			fmt.Printf("request %2d: REJECTED\n", i)
		}
	}

	fmt.Println("\nwaiting 2 seconds for tokens to refill (expect 10 tokens)")
	time.Sleep(2 * time.Second)

	fmt.Println("\nsending more requests slowly")
	for i := 16; i <= 36; i++ {
		if limiter.Allow() {
			fmt.Printf("request %2d: ALLOWED\n", i)
		} else {
			fmt.Printf("request %2d: REJECTED\n", i)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
