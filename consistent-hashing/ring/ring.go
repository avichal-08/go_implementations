package ring

import (
	"slices"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/cespare/xxhash/v2"
)

type ringState struct {
	ring   []uint64
	nodes  map[uint64]string // maps hash to physical shard id
	shards map[string]bool   // tracks which physical shards are active
}

type HashRing struct {
	writeMu     sync.Mutex
	state       atomic.Pointer[ringState] // atomic pointer to the immutable active state
	vNodesCount int // counts no of virtual nodes per shard
}

// fn to create a new hashring with the specified number of virtual nodes per shard
func New(vNodesCount int) *HashRing {
	r := &HashRing{
		vNodesCount: vNodesCount,
	}

	initialState := &ringState{
		ring:   make([]uint64, 0),
		nodes:  make(map[uint64]string),
		shards: make(map[string]bool),
	}
	r.state.Store(initialState)

	return r
}

// generates a uint64 hash
func hash(key string) uint64 {
	return xxhash.Sum64String(key)
}

// adds a physical shard to the ring along with its virtual nodes
func (r *HashRing) AddShard(shardID string) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	currentState := r.state.Load()
	if currentState.shards[shardID] {
		return
	}

	newState := &ringState{
		ring:   make([]uint64, len(currentState.ring), len(currentState.ring)+r.vNodesCount),
		nodes:  make(map[uint64]string, len(currentState.nodes)+r.vNodesCount),
		shards: make(map[string]bool, len(currentState.shards)+1),
	}

	copy(newState.ring, currentState.ring)
	for k, v := range currentState.nodes {
		newState.nodes[k] = v
	}
	for k, v := range currentState.shards {
		newState.shards[k] = v
	}

	newState.shards[shardID] = true

	// loop to assign virtual nodes to the ring upto vNodesCount per shard
	for i := 0; i < r.vNodesCount; i++ {
		vNodeKey := shardID + "#" + strconv.Itoa(i)
		h := hash(vNodeKey)

		// collision handling offset
		collisionOffset := 0
		for {
			existingShard, exists := newState.nodes[h]
			if !exists {
				break
			}
			if existingShard == shardID {
				break // already belongs to this shard (rare collision with itself case)
			}
			collisionOffset++
			h = hash(vNodeKey + "_" + strconv.Itoa(collisionOffset))
		}

		newState.ring = append(newState.ring, h)
		newState.nodes[h] = shardID
	}

	// sort the ring based on hash values to enable binary search
	slices.Sort(newState.ring)

	// this atomically swaps the active state pointer
	r.state.Store(newState)
}

// removes a physical shard from the ring along with its virtual nodes
func (r *HashRing) RemoveShard(shardID string) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	currentState := r.state.Load()
	if !currentState.shards[shardID] {
		return
	}

	newState := &ringState{
		ring:   make([]uint64, 0, len(currentState.ring)-r.vNodesCount),
		nodes:  make(map[uint64]string, len(currentState.nodes)-r.vNodesCount),
		shards: make(map[string]bool, len(currentState.shards)-1),
	}

	for k, v := range currentState.shards {
		if k != shardID {
			newState.shards[k] = v
		}
	}

	for _, h := range currentState.ring {
		if currentState.nodes[h] != shardID {
			newState.ring = append(newState.ring, h)
			newState.nodes[h] = currentState.nodes[h]
		}
	}

	r.state.Store(newState)
}

// returns the shard responsible for the given key
func (r *HashRing) GetShard(key string) string {
	state := r.state.Load()

	if len(state.ring) == 0 {
		return ""
	}

	h := hash(key)
	idx := sort.Search(len(state.ring), func(i int) bool {
		return state.ring[i] >= h
	})

	if idx == len(state.ring) {
		idx = 0
	}

	return state.nodes[state.ring[idx]]
}
