package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

type WALEntry struct {
	ID    int
	Query string
}

type ReplicationEngine struct {
	Primary  *sql.DB
	Replica1 *sql.DB
	Replica2 *sql.DB

	walMutex sync.RWMutex
	wal      []WALEntry
	walIndex int

	r1Offset    int
	r2Offset    int
	readCounter *ReadCounter

	stopChan chan struct{}
	wg       sync.WaitGroup
}

type ReadCounter struct {
	Mutex sync.RWMutex
	Count int
}

func main() {
	primary, _ := sql.Open("postgres", "postgres://postgres:pass@localhost:5432/mydb?sslmode=disable")
	replica1, _ := sql.Open("postgres", "postgres://postgres:pass@localhost:5433/mydb?sslmode=disable")
	replica2, _ := sql.Open("postgres", "postgres://postgres:pass@localhost:5434/mydb?sslmode=disable")

	engine := &ReplicationEngine{
		Primary:     primary,
		Replica1:    replica1,
		Replica2:    replica2,
		wal:         make([]WALEntry, 0),
		stopChan:    make(chan struct{}),
		readCounter: &ReadCounter{},
	}

	engine.wg.Add(1)
	go engine.StartReplicator()

	mux := http.NewServeMux()
	mux.HandleFunc("/write", engine.HandleWrite)
	mux.HandleFunc("/read", engine.HandleRead)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		fmt.Println("Server listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\nshutting down gracefully...")

	srv.Shutdown(context.Background())

	close(engine.stopChan)

	engine.wg.Wait()
	fmt.Println("all wal entries flushed")
}

func (e *ReplicationEngine) HandleWrite(w http.ResponseWriter, r *http.Request) {
	msg := r.URL.Query().Get("msg")
	query := fmt.Sprintf("INSERT INTO test_table (message) VALUES ('%s')", msg)

	e.walMutex.Lock()
	e.walIndex++
	entry := WALEntry{ID: e.walIndex, Query: query}
	e.wal = append(e.wal, entry)
	e.walMutex.Unlock()

	_, err := e.Primary.Exec(query)
	if err != nil {
		http.Error(w, "failed to write to primary", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "wrote to primary, appended to wal (id: %d)\n", entry.ID)
}

func (e *ReplicationEngine) HandleRead(w http.ResponseWriter, r *http.Request) {
	e.readCounter.Mutex.Lock()
	e.readCounter.Count++
	replicaNum := (e.readCounter.Count % 2) + 1
	e.readCounter.Mutex.Unlock()

	replica := e.Replica1
	if replicaNum == 2 {
		replica = e.Replica2
	}

	rows, err := replica.Query("SELECT id, message FROM test_table")
	if err != nil || rows.Err() != nil {
		http.Error(w, "failed to read from replica", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var message string
		rows.Scan(&id, &message)
		results = append(results, map[string]interface{}{"id": id, "message": message})
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "Read from Replica %d\n", replicaNum)
	json.NewEncoder(w).Encode(results)
}

func (e *ReplicationEngine) StartReplicator() {
	defer e.wg.Done()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.flush()
		case <-e.stopChan:
			fmt.Println("flushing remaining wal")
			e.flush()
			return
		}
	}
}

func (e *ReplicationEngine) flush() {
	e.walMutex.RLock()
	defer e.walMutex.RUnlock()

	for e.r1Offset < len(e.wal) {
		entry := e.wal[e.r1Offset]
		_, err := e.Replica1.Exec(entry.Query)
		if err == nil {
			e.r1Offset++
			log.Printf("replicated WAL ID %d to replica 1", entry.ID)
		} else {
			log.Printf("replica 1 failed on WAL ID %d: %v", entry.ID, err)
			break
		}
	}

	for e.r2Offset < len(e.wal) {
		entry := e.wal[e.r2Offset]
		_, err := e.Replica2.Exec(entry.Query)
		if err == nil {
			e.r2Offset++
			log.Printf("replicated WAL ID %d to replica 2", entry.ID)
		} else {
			log.Printf("replica 2 failed on WAL ID %d: %v", entry.ID, err)
			break
		}
	}
}
