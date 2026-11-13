# http-server

A `net/http` server written the way you would run it in production, using only the standard library.

`http.ListenAndServe(":8080", nil)` works, but it has no timeouts, no shutdown handling, no panic safety and no limit on concurrent work. This module adds each of those.

## Run

```bash
go run .
```

```bash
go test ./...
```

Configuration is read from the environment:

| Variable | Default | Meaning |
|---|---|---|
| `ADDR` | `:8080` | listen address |
| `MAX_IN_FLIGHT` | `100` | max requests processed at once |
| `REQUEST_TIMEOUT` | `10s` | per-request deadline |
| `SHUTDOWN_TIMEOUT` | `15s` | how long to wait for in-flight requests on shutdown |

## How concurrency works

`net/http` starts **one goroutine per connection**, so handlers already run concurrently with no extra code. That means:

- anything shared between handlers must be safe for concurrent use (here: `atomic.Bool` for readiness, a channel for the limiter);
- nothing limits how many goroutines exist. Under overload the server keeps accepting work until it runs out of memory.

`limitInFlight` fixes the second point with a buffered channel used as a semaphore. If all slots are taken the request is rejected immediately with `503` and `Retry-After` (load shedding) rather than queued.

## What is in here

| Piece | File | Why |
|---|---|---|
| Explicit `http.Server` with `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes` | `main.go` | a slow client cannot hold a connection open forever |
| Own `ServeMux` instead of the global default | `handlers.go` | no package can register routes behind your back |
| Graceful shutdown on SIGINT/SIGTERM via `signal.NotifyContext` + `srv.Shutdown` | `main.go` | deploys do not cut off in-flight requests |
| `/healthz` (liveness) and `/readyz` (readiness, fails once shutdown starts) | `handlers.go` | the load balancer stops sending traffic before the server goes away |
| `requestID` | `middleware.go` | correlate log lines for one request |
| `logging` | `middleware.go` | one structured (JSON `slog`) line per request with status and duration |
| `recoverPanic` | `middleware.go` | a bug in one handler returns 500 instead of dropping the connection |
| `limitInFlight` | `middleware.go` | bounded concurrency, load shedding |
| `timeout` (`http.TimeoutHandler`) | `middleware.go` | every request has a deadline; the request context is cancelled |
| `http.MaxBytesReader` + `DisallowUnknownFields` | `handlers.go` | request bodies are size-capped and strictly parsed |
| Handlers watch `r.Context()` | `handlers.go` | work stops when the client leaves or the deadline passes |
| `run() error` and `newHandler(deps...)` | `main.go`, `handlers.go` | no globals, so the handler tree is testable with `httptest` |

Middleware order, outermost first:

```
requestID -> logging -> recoverPanic -> [ /healthz, /readyz ]
                                     -> limitInFlight -> timeout -> API routes
```

Health checks sit outside the limiter and timeout so they still answer when the server is saturated.

## Try it

```bash
curl -i localhost:8080/hello
```

```bash
curl -i -X POST -d '{"message":"hi"}' localhost:8080/echo
```

Panic is recovered, server stays up:

```bash
curl -i localhost:8080/panic
```

Request deadline (503 after `REQUEST_TIMEOUT`):

```bash
curl -i "localhost:8080/slow?d=30s"
```

Load shedding: start with `MAX_IN_FLIGHT=2`, then fire four slow requests at once; two get 200 and two get 503.

```bash
for i in 1 2 3 4; do curl -s -o /dev/null -w "%{http_code}\n" "localhost:8080/slow?d=1s" & done; wait
```

Graceful shutdown: start a slow request, then press Ctrl+C in the server terminal. The request still completes, then the server logs `server stopped cleanly`.

```bash
curl "localhost:8080/slow?d=5s"
```
