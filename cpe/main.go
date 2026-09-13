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
	inputCh := make(chan Payload)
	cleanInCh := make(chan Payload)
	validateInCh := make(chan Payload)
	cleanOutCh := make(chan CleanResult)
	validateOutCh := make(chan ValidateResult)
	outputCh := make(chan string)

	go splitter(inputCh, cleanInCh, validateInCh)
	go cleaner(cleanInCh, cleanOutCh)
	go validator(validateInCh, validateOutCh)
	go transformer(cleanOutCh, validateOutCh, outputCh)

	start := time.Now()

	go func() {
		for i := 1; i <= 3; i++ {
			inputCh <- Payload{ID: i, Text: " rawdata "}
		}
		close(inputCh)
	}()

	for result := range outputCh {
		fmt.Println(">>> result:", result)
	}
	fmt.Printf("concurrent pipeline finished successfully in %v\n", time.Since(start))
}

func splitter(in <-chan Payload, out1, out2 chan<- Payload) {
	for p := range in {
		log("splitter", p.ID, "routing to branches")
		out1 <- p
		out2 <- p
	}
	close(out1)
	close(out2)
}

func cleaner(in <-chan Payload, out chan<- CleanResult) {
	for p := range in {
		log("cleaner", p.ID, "started")
		time.Sleep(200 * time.Millisecond)
		out <- CleanResult{ID: p.ID, Cleaned: p.Text + "[cleaned]"}
		log("cleaner", p.ID, "finished")
	}
	close(out)
}

func validator(in <-chan Payload, out chan<- ValidateResult) {
	for p := range in {
		log("validator", p.ID, "started")
		time.Sleep(300 * time.Millisecond)
		out <- ValidateResult{ID: p.ID, IsValid: true}
		log("validator", p.ID, "finished")
	}
	close(out)
}

func transformer(cleanIn <-chan CleanResult, valIn <-chan ValidateResult, out chan<- string) {
	for {
		cRes, ok1 := <-cleanIn
		vRes, ok2 := <-valIn

		if !ok1 || !ok2 {
			break
		}

		log("transformer", cRes.ID, "joining results")
		time.Sleep(100 * time.Millisecond)

		finalStr := fmt.Sprintf("id: %d | data: '%s' | valid: %t", cRes.ID, cRes.Cleaned, vRes.IsValid)
		out <- finalStr
		log("transformer", cRes.ID, "finished")
	}
	close(out)
}
