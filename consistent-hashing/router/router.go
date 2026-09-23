package router

import (
	"database/sql"
	"fmt"
	"log"

	"consistent-hashing/ring"

	_ "github.com/lib/pq"
)

// router struct integrates the HashRing with actual database connections
type Router struct {
	Ring        *ring.HashRing
	Connections map[string]*sql.DB
}

func NewRouter(vnodes int) *Router {
	return &Router{
		Ring:        ring.New(vnodes),
		Connections: make(map[string]*sql.DB),
	}
}

// fn to add a new pg instance to the routing layer
func (r *Router) AddDatabase(shardID, connStr string) error {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	//creates table if it doesn't exist
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY, name TEXT)`)
	if err != nil {
		return fmt.Errorf("failed to initialize schema on %s: %v", shardID, err)
	}

	r.Connections[shardID] = db
	r.Ring.AddShard(shardID)
	log.Printf("added db shard: %s", shardID)
	return nil
}

// fn to route an insert operation to the correct shard
func (r *Router) InsertUser(id, name string) (string, error) {
	shardID := r.Ring.GetShard(id)
	if shardID == "" {
		return "", fmt.Errorf("no shards available")
	}

	db := r.Connections[shardID]
	_, err := db.Exec(`INSERT INTO users (id, name) VALUES ($1, $2) ON CONFLICT (id) DO UPDATE SET name = $2`, id, name)
	return shardID, err
}

// fn to route a select operation to the correct shard
func (r *Router) GetUser(id string) (string, string, error) {
	shardID := r.Ring.GetShard(id)
	if shardID == "" {
		return "", "", fmt.Errorf("no shards available")
	}

	db := r.Connections[shardID]
	var name string
	err := db.QueryRow(`SELECT name FROM users WHERE id = $1`, id).Scan(&name)
	return shardID, name, err
}
