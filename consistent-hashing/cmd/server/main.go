package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"consistent-hashing/router"
)

func main() {

	dbRouter := router.NewRouter(100)

	// initialize the 3 pgsql shards defined in docker-compose.yml
	shards := map[string]string{
		"shard-0": "postgres://user:pass@localhost:5432/db?sslmode=disable",
		"shard-1": "postgres://user:pass@localhost:5433/db?sslmode=disable",
		"shard-2": "postgres://user:pass@localhost:5434/db?sslmode=disable",
	}

	for id, dsn := range shards {
		if err := dbRouter.AddDatabase(id, dsn); err != nil {
			log.Printf("Warning: Could not connect to %s. Ensure Docker containers are running.", id)
		}
	}

	http.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {

		var req  struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		shardID, err := dbRouter.InsertUser(req.ID, req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "success", "routed_to": shardID})
	})

	http.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {

		id := r.PathValue("id")
		shardID, name, err := dbRouter.GetUser(id)
		if err != nil {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"id": id, "name": name, "found_on_shard": shardID})
	})

	// endpoint to add a shard
	http.HandleFunc("POST /shard/add", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ShardID string `json:"shard_id"`
			DSN     string `json:"dsn"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := dbRouter.AddDatabase(req.ShardID, req.DSN); err != nil {
			http.Error(w, fmt.Sprintf("Failed to connect to new shard: %v", err), http.StatusInternalServerError)
			return
		}

		response, _ := json.Marshal(map[string]string{"status": "added and connected", "shard": req.ShardID})
		w.Write(append(response, '\r', '\n'))
	})

	// endpoint to del a shard
	http.HandleFunc("POST /shard/delete", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ShardID string `json:"shard_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		dbRouter.Ring.RemoveShard(req.ShardID)

		if db, exists := dbRouter.Connections[req.ShardID]; exists {
			db.Close()
			delete(dbRouter.Connections, req.ShardID)
		}

		response, _ := json.Marshal(map[string]string{"status": "removed and disconnected", "shard": req.ShardID})
		w.Write(append(response, '\r', '\n'))
	})

	fmt.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
