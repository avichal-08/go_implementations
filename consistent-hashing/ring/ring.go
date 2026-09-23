package ring

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"slices"
	"sort"
	"sync"
)

type HashRing struct {
	mu          sync.RWMutex
	vNodesCount int          // counts no of virtual nodes per shard
	ring        []uint64
	nodes       map[uint64]string // maps hash to physical shard id
	shards      map[string]bool  // tracks which physical shards are active
}

func New(vNodesCount int) *HashRing {
	return &HashRing{
		vNodesCount: vNodesCount,
		ring:        make([]uint64, 0),
		nodes:       make(map[uint64]string),
		shards:      make(map[string]bool),
	}
}

// generates a uint64 hash using SHA-256
func hash(key string) uint64 {
	h := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint64(h[:8])
}

// adds a physical shard to the ring along with its virtual nodes
func (r *HashRing) AddShard(shardID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.shards[shardID] {
		return
	}
	r.shards[shardID] = true

	// loop to assign virtual nodes to the ring upto vNodesCount per shard
	for i := 0; i < r.vNodesCount; i++ {
		vNodeKey := fmt.Sprintf("%s#%d", shardID, i)
		h := hash(vNodeKey)

		// collision handling offset
		collisionOffset := 0
		for {
			existingShard, exists := r.nodes[h]
			if !exists {
				break
			}
			if existingShard == shardID {
				break // already belongs to this shard (rare collision with itself case)
			}
			collisionOffset++
			h = hash(fmt.Sprintf("%s_%d", vNodeKey, collisionOffset))
		}

		r.ring = append(r.ring, h)
		r.nodes[h] = shardID
	}

	// sort the ring based on hash values to enable binary search
	slices.Sort(r.ring)
}

// removes a physical shard from the ring along with its virtual nodes
func (r *HashRing) RemoveShard(shardID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.shards[shardID] {
		return
	}
	delete(r.shards, shardID)

	var newRing []uint64
	for _, h := range r.ring {
		if r.nodes[h] == shardID {
			delete(r.nodes, h)
		} else {
			newRing = append(newRing, h)
		}
	}
	r.ring = newRing
}

// returns the shard responsible for the given key
func (r *HashRing) GetShard(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.ring) == 0 {
		return ""
	}

	h := hash(key)
	idx := sort.Search(len(r.ring), func(i int) bool {
		return r.ring[i] >= h
	})

	// wrap around clockwise if key hash is greater than the largest node hash
	if idx == len(r.ring) {
		idx = 0
	}

	return r.nodes[r.ring[idx]]
}

//returns stats about the ring
func (r *HashRing) Info() (activeShards int, ringSize int) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.shards), len(r.ring)
}
