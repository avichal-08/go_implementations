package ring

import (
	"fmt"
	"sync"
	"testing"
)

func TestHashRing_Empty(t *testing.T) {
	r := New(100)
	if shard := r.GetShard("key1"); shard != "" {
		t.Errorf("expected empty string for empty ring, got %s", shard)
	}
}

func TestHashRing_SingleShard(t *testing.T) {
	r := New(100)
	r.AddShard("shard-A")
	if shard := r.GetShard("key1"); shard != "shard-A" {
		t.Errorf("expected shard-A, got %s", shard)
	}
}

func TestHashRing_Deterministic(t *testing.T) {
	r := New(100)
	r.AddShard("shard-A")
	r.AddShard("shard-B")
	r.AddShard("shard-C")

	key := "user:123"
	shard1 := r.GetShard(key)
	shard2 := r.GetShard(key)

	if shard1 != shard2 {
		t.Errorf("expected deterministic routing, got %s then %s", shard1, shard2)
	}
}

func TestHashRing_RemoveShard(t *testing.T) {
	r := New(100)
	r.AddShard("shard-A")
	r.AddShard("shard-B")

	key := "user:999"
	initialShard := r.GetShard(key)

	r.RemoveShard(initialShard)
	newShard := r.GetShard(key)

	if initialShard == newShard {
		t.Errorf("shard should have changed after removing %s", initialShard)
	}
	if newShard == "" {
		t.Error("should have routed to the remaining shard")
	}
}

func TestHashRing_Concurrent(t *testing.T) {
	r := New(10)
	r.AddShard("shard-A")

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.GetShard(fmt.Sprintf("key:%d", i))
		}(i)
	}

	go r.AddShard("shard-B")
	go r.RemoveShard("shard-A")

	wg.Wait()
}
