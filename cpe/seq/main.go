package main

import (
	"fmt"
	"time"
)

type Payload struct {
	ID   int
	Text string
}

type CleanResult struct {
	ID      int
	Cleaned string
}

type ValidateResult struct {
	ID      int
	IsValid bool
}

func log(stage string, id int, msg string) {
	fmt.Printf("[%s] %-10s | Item %d | %s\n", time.Now().Format("15:04:05.000"), stage, id, msg)
}

func main() {
	start := time.Now()

	for i := 1; i <= 3; i++ {
		p := Payload{ID: i, Text: " rawdata "}

		cRes := clean(p)

		vRes := validate(p)

		finalStr := transform(cRes, vRes)

		fmt.Println(">>> result:", finalStr)
	}

	fmt.Printf("sequential pipeline finished successfully in %v\n", time.Since(start))
}

func clean(p Payload) CleanResult {
	log("cleaner", p.ID, "started")
	time.Sleep(200 * time.Millisecond)
	log("cleaner", p.ID, "finished")
	return CleanResult{ID: p.ID, Cleaned: p.Text + "[cleaned]"}
}

func validate(p Payload) ValidateResult {
	log("validator", p.ID, "started")
	time.Sleep(300 * time.Millisecond)
	log("validator", p.ID, "finished")
	return ValidateResult{ID: p.ID, IsValid: true}
}

func transform(cRes CleanResult, vRes ValidateResult) string {
	log("transformer", cRes.ID, "joining results")
	time.Sleep(100 * time.Millisecond)
	log("transformer", cRes.ID, "finished")
	return fmt.Sprintf("id: %d | data: '%s' | valid: %t", cRes.ID, cRes.Cleaned, vRes.IsValid)
}
