# Go DAG Executor

A minimal, Directed Acyclic Graph (DAG) task execution written in Go.

## What is a DAG?
A Directed Acyclic Graph (DAG) is a conceptual model used to represent a workflow or pipeline. 
- **Nodes** represent individual tasks (e.g., `fetch`, `clean`, `save`).
- **Edges** (arrows) represent dependencies (e.g., `clean` must run after `fetch`).
- **Acyclic** means there are no circular loops. A task cannot depend on itself, either directly or indirectly. 

## Dependency Scheduling & Concurrency
The executor uses an active, centralized scheduler utilizing Go's synchronization primitives:
- Each task maintains an **in-degree**, which is the count of upstream dependencies it is waiting for.
- When `Run()` is called, any task with an `in-degree` of 0 is considered independent and is immediately launched in a new goroutine.
- **Why Concurrent?** Independent tasks share no data and are completely decoupled. By utilizing Go's lightweight concurrency (Goroutines), independent paths in your graph execute in parallel automatically, vastly reducing total pipeline runtime.
- As each task succeeds, the scheduler decreases the `in-degree` of downstream tasks. When a downstream task hits 0, it is immediately launched.

## Architecture and Design Decisions
- **Separation of Concerns:** The graph creation (`Add`, `DependsOn`) is safely isolated from execution (`Run()`).
- **Validation First:** Using Kahn's Topological Sort algorithm, the graph ensures that all referenced dependencies exist and guarantees no deadlocks (cycles) occur during execution.
- **Fail-Fast Error Handling:** If any task returns an error, the central scheduling loop breaks immediately. Dependent tasks are aborted (never scheduled).
- **Graceful Cleanup:** Despite the fail-fast behavior, running goroutines are not rudely abandoned. A `sync.WaitGroup` works alongside channel draining to ensure all currently executing tasks safely exit without memory leaks or closed-channel panics.
