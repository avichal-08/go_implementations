package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

const maxBodyBytes = 1 << 20 // 1 MB

// builds the full handler tree
// deps come in as args (no globals) so tests can build it directly
func newHandler(cfg config, ready *atomic.Bool, logger *slog.Logger) http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /hello", handleHello)
	api.HandleFunc("GET /slow", handleSlow)
	api.HandleFunc("POST /echo", handleEcho)
	api.HandleFunc("GET /panic", handlePanic)

	root := http.NewServeMux()
	// health checks skip the limiter and timeout, they must answer even when the server is full
	root.HandleFunc("GET /healthz", handleHealthz)
	root.HandleFunc("GET /readyz", handleReadyz(ready))
	root.Handle("/", chain(api, limitInFlight(cfg.maxInFlight), timeout(cfg.requestTimeout)))

	return chain(root, requestID, logging(logger), recoverPanic(logger))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// liveness probe: the process is up
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readiness probe: fails once shutdown has begun so the load balancer stops routing here
func handleReadyz(ready *atomic.Bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			writeError(w, http.StatusServiceUnavailable, "shutting down")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func handleHello(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message":    "hello!",
		"request_id": requestIDFrom(r.Context()),
	})
}

// fakes work for ?d=<duration> (default 2s)
// watches the request ctx so it stops when the client leaves or the timeout fires
func handleSlow(w http.ResponseWriter, r *http.Request) {
	d := 2 * time.Second
	if v := r.URL.Query().Get("d"); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "d must be a duration like 3s")
			return
		}
		d = parsed
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		writeJSON(w, http.StatusOK, map[string]string{"slept": d.String()})
	case <-r.Context().Done():
		// nobody is listening anymore, just stop
	}
}

type echoRequest struct {
	Message string `json:"message"`
}

// safe body handling: size cap and strict json decoding
func handleEcho(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req echoRequest
	if err := dec.Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "body larger than 1 MB")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func handlePanic(w http.ResponseWriter, r *http.Request) {
	panic("something went wrong in a handler")
}
