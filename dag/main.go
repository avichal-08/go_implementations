package main

import (
	"fmt"
	"log"
	"time"

	"dag/dag"
)

func makeTask(duration time.Duration) dag.TaskFunc {
	return func() error {
		time.Sleep(duration)
		return nil
	}
}

func main() {
	d := dag.NewDAG()

	d.Add("fetch", makeTask(300*time.Millisecond))
	d.Add("clean", makeTask(500*time.Millisecond))
	d.Add("validate", makeTask(200*time.Millisecond))
	d.Add("transform", makeTask(400*time.Millisecond))
	d.Add("save", makeTask(200*time.Millisecond))

	d.DependsOn("clean", "fetch")
	d.DependsOn("validate", "fetch")
	d.DependsOn("transform", "clean", "validate")
	d.DependsOn("save", "transform")

	start := time.Now()
	err := d.Run()
	if err != nil {
		log.Fatalf("execution failed: %v", err)
	}

	fmt.Printf("all tasks completed successfully in %v!\n", time.Since(start))
}
