# Consistent Hashing Router

This project demonstrates how consistent hashing works under the hood to route keys (users) to distributed database shards.

## The Problem with `hash(key) % N`
Naive modulo hashing relies heavily on `N` (the total number of nodes). If you have 3 nodes and scale up to 4, the denominator changes. This forces roughly 75% of your existing keys to point to entirely new servers, requiring massive, network-saturating data migrations.

## How Consistent Hashing Solves This
Consistent hashing projects both the data keys and the server nodes onto the same circular space (a Hash Ring). 
A key maps to the first node encountered by moving clockwise around the ring. 

Because nodes have fixed positions independent of `N`, adding a new node only steals keys from its immediate clockwise neighbor. Instead of 75% data movement, scaling from 3 to 4 nodes results in roughly 25% data movement—the absolute mathematical minimum required for a balanced cluster.

## Why Virtual Nodes?
If you place exactly 1 point on the ring for `shard-A` and 1 for `shard-B`, they might randomly hash very close to each other. `shard-A` might own 80% of the ring circumference, causing severe hot-spotting. 

Virtual nodes solve this by hashing many replicas (e.g., `shard-A#0`, `shard-A#1`... `shard-A#99`) onto the ring for each physical server. This interleaved spacing statistically guarantees an even load distribution.

## Limitations & Real-World Context (Replication vs. Sharding)
This project focuses purely on **sharding** (partitioning data). Real-world databases like Cassandra or DynamoDB combine this ring with **replication** (storing a key on the next 3 clockwise nodes). 

Additionally, this HTTP API updates the routing ring instantly on a membership change. In a real system, changing the ring routing without migrating the underlying rows will result in cache misses and missing rows. Data migration requires a background process to move rows before the ring officially updates.

## Performance Journey & Optimizations

This router was incrementally optimized for extreme high-throughput, low-latency environments using Go's built-in benchmarking and escape analysis tools. 

### 1. Baseline Implementation
The initial implementation used standard `crypto/sha256` hashing and a `sync.RWMutex` to protect the routing state.

| Operation | Time/op | Allocations | Memory/op |
| :--- | :--- | :--- | :--- |
| `GetShard` (Read) | ~500 ns | 0 allocs | 0 B |
| `AddShard` (Write) | ~11.1 ms | 202 allocs | ~18.4 KB |

### 2. Zero-Allocation Hot Path (Escape Analysis)
By running Go's escape analysis (`go build -gcflags="-m"`), we identified that using `fmt.Sprintf("%s#%d", shardID, i)` for generating virtual nodes was forcing memory onto the heap due to interface boxing. We replaced this with native string concatenation and `strconv.Itoa`.

| Operation | Time/op | Allocations | Memory/op | Impact |
| :--- | :--- | :--- | :--- | :--- |
| `GetShard` (Read) | ~484 ns | 0 allocs | 0 B | Maintained zero-allocation |
| `AddShard` (Write) | ~12.0 ms | **2 allocs** | ~14.6 KB | **99% reduction in heap allocations** |

### 3. CPU-Bound Optimization (`xxhash`)
SHA-256 is cryptographically secure but computationally heavy. We replaced it with `github.com/cespare/xxhash/v2`, an extremely fast non-cryptographic hash algorithm designed specifically for hash tables and rings.

| Operation | Time/op | Allocations | Memory/op | Impact |
| :--- | :--- | :--- | :--- | :--- |
| `GetShard` (Read) | **~87-122 ns** | 0 allocs | 0 B | **~4x speedup in CPU throughput** |
| `AddShard` (Write) | ~12.3 ms | 2 allocs | ~14.3 KB | Minimal impact |

### 4. Lock-Free Reads (Copy-on-Write)
To eliminate `sync.RWMutex` contention across CPU cores under high load, we implemented a Copy-on-Write (CoW) architecture using `atomic.Pointer`. 
*   **Reads** are now completely lock-free. Threads read an immutable state, allowing perfectly linear multi-core scaling.
*   **Writes** perform a deep-copy of the ring before atomically swapping the pointer. This increases write allocations but guarantees zero disruption to active readers.

| Operation | Time/op | Allocations | Memory/op | Impact |
| :--- | :--- | :--- | :--- | :--- |
| `GetShard` (Read) | **~67-107 ns** | 0 allocs | 0 B | **100% Lock-free, max multi-core scaling** |
| `AddShard` (Write) | ~13.0 ms | 143 allocs | ~2.1 MB | Expected tradeoff for CoW state isolation |

At ~70ns per operation, a single logical CPU core can route approximately **14.2 million keys per second**.

---

## Setup & Running
Start the PostgreSQL shards and run the Go server:

```bash
docker compose up -d
go run cmd/server/main.go
```

## API Usage
The router exposes endpoints for managing data and the shard ring dynamically. You can interact with these using your preferred REST client (Postman, Insomnia) or via the command line using standard curl.

### User Operations
1. Create/Update a User
Routes the user to the correct shard based on the consistent hash of their ID.
- Endpoint: POST http://localhost:8080/users
- Headers: Content-Type: application/json
- Payload:
```json
{
  "id": "user123", 
  "name": "Alice"
}
```

2. Retrieve a User
Finds the shard responsible for the ID and fetches the user data.
- Endpoint: GET http://localhost:8080/users/{id}

### Shard Management
These endpoints allow you to dynamically add or remove database shards from the hash ring while the application is running.

3. Add a New Shard
Adds a new shard to the hash ring and establishes a database connection.
- Endpoint: POST http://localhost:8080/shard/add
- Headers: Content-Type: application/json
- Payload:
```json
{
  "shard_id": "shard-3", 
  "dsn": "postgres://user:pass@localhost:5435/db?sslmode=disable"
}
```
4. Delete a Shard
Removes a shard's virtual nodes from the hash ring and closes its database connection.
- Endpoint: POST http://localhost:8080/shard/delete
- Headers: Content-Type: application/json
- Payload:
```json
{
  "shard_id": "shard-3"
}
```
