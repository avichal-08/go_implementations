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
