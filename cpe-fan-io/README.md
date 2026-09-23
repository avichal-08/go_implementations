# Tiny Go Concurrent Pipeline

This project is a minimal, dependency-free executor designed to demonstrate how to build concurrent pipelines in Go using **goroutines** and **channels**. It includes both a concurrent and a sequential implementation to clearly illustrate the performance benefits of concurrent design.

## The Concurrency Model

This pipeline uses a **Fan-out / Fan-in** architecture:
* **Fan-out:** A `Splitter` duplicates incoming data and routes it to both a `Cleaner` and a `Validator` simultaneously.
* **Concurrent Processing:** The `Cleaner` (200ms) and `Validator` (300ms) process the same item at the exact same time in separate goroutines.
* **Fan-in:** A `Transformer` (100ms) waits to read from both the cleaner's and validator's output channels before merging their results.

## Performance Comparison: Concurrent vs. Sequential

We process 3 data items to compare the total execution time between the two approaches. 

### 1. Sequential Execution
In the sequential model, each item must pass completely through all three stages before the next item can begin. 

* **The Math:** `(Clean: 200ms + Validate: 300ms + Transform: 100ms) × 3 items = 1800ms`
* **The Proof:** As seen in the sequential run output below, the pipeline blocks at every step and takes approximately **1.81 seconds** to complete.

### 2. Concurrent Execution
In the concurrent model, `Clean` and `Validate` happen simultaneously. Furthermore, because it's a pipeline, item 2 can start processing while item 1 is finishing up.

* **The Math:** The pipeline's speed is dictated by its slowest stage (Validate at 300ms). The total time is roughly the bottleneck multiplied by the number of items, plus the final stage time: `(3 items × 300ms) + Transform: 100ms = 1000ms`.
* **The Proof:** As seen in the concurrent run output below, the pipeline takes approximately **1.00 seconds** to complete, drastically reducing the total execution time.

## Running the Code

To run the concurrent pipeline:
```bash
go run main.go
```

To run the sequential pipeline:
```bash
cd sequential
go run main.go
```
