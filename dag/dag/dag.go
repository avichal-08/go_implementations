package dag

import (
	"fmt"
	"log"
	"sync"
)

type TaskFunc func() error

type Node struct {
	Name string
	Func TaskFunc
}

type DAG struct {
	nodes    map[string]*Node
	adj      map[string][]string
	inDegree map[string]int
	mu       sync.Mutex
}

func NewDAG() *DAG {
	return &DAG{
		nodes:    make(map[string]*Node),
		adj:      make(map[string][]string),
		inDegree: make(map[string]int),
	}
}

func (d *DAG) Add(name string, fn TaskFunc) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.nodes[name]; exists {
		return fmt.Errorf("task %q already exists", name)
	}

	d.nodes[name] = &Node{Name: name, Func: fn}
	if _, exists := d.inDegree[name]; !exists {
		d.inDegree[name] = 0
	}
	return nil
}

func (d *DAG) DependsOn(target string, dependencies ...string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, dep := range dependencies {
		d.adj[dep] = append(d.adj[dep], target)
		d.inDegree[target]++
	}
}

func (d *DAG) validate() error {
	for node := range d.adj {
		if _, ok := d.nodes[node]; !ok {
			return fmt.Errorf("task %q is referenced but not added", node)
		}
		for _, child := range d.adj[node] {
			if _, ok := d.nodes[child]; !ok {
				return fmt.Errorf("task %q is referenced but not added", child)
			}
		}
	}

	inDegreeCopy := make(map[string]int)
	for k, v := range d.inDegree {
		inDegreeCopy[k] = v
	}

	var queue []string
	for name, deg := range inDegreeCopy {
		if deg == 0 && d.nodes[name] != nil {
			queue = append(queue, name)
		}
	}

	visitedCount := 0
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		visitedCount++

		for _, child := range d.adj[curr] {
			inDegreeCopy[child]--
			if inDegreeCopy[child] == 0 {
				queue = append(queue, child)
			}
		}
	}

	if visitedCount != len(d.nodes) {
		return fmt.Errorf("dag validation failed: a cycle was detected or dependencies are missing")
	}

	return nil
}

type taskResult struct {
	name string
	err  error
}

func (d *DAG) Run() error {
	if err := d.validate(); err != nil {
		return err
	}

	inDegree := make(map[string]int)
	for k, v := range d.inDegree {
		inDegree[k] = v
	}

	results := make(chan taskResult)
	var wg sync.WaitGroup

	launch := func(name string) {
		wg.Add(1)
		go func(taskName string) {
			defer wg.Done()
			log.Printf("[dag] task %q: STARTED\n", taskName)

			err := d.nodes[taskName].Func()

			if err != nil {
				log.Printf("[dag] task %q: FAILED (%v)\n", taskName, err)
			} else {
				log.Printf("[dag] task %q: SUCCEEDED\n", taskName)
			}
			results <- taskResult{name: taskName, err: err}
		}(name)
	}

	for name, deg := range inDegree {
		if deg == 0 {
			launch(name)
		}
	}

	var executionErr error
	completed := 0
	totalTasks := len(d.nodes)

	for completed < totalTasks {
		res := <-results
		completed++

		if res.err != nil {
			executionErr = res.err
			break
		}

		for _, child := range d.adj[res.name] {
			inDegree[child]--
			if inDegree[child] == 0 {
				launch(child)
			}
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for range results {
	}

	return executionErr
}
