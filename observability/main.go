package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var requestCount = promauto.NewCounterVec(
	prometheus.CounterOpts{Name: "http_requests_total"},
	[]string{"path"},
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	http.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		requestCount.WithLabelValues("/hello").Inc()
		w.WriteHeader(200)
		w.Write([]byte("hello!"))
		slog.Info("Request handled", slog.String("path", "/hello"))
	})

	http.Handle("/metrics", promhttp.Handler())

	slog.Info("Starting server on :8080")
	http.ListenAndServe(":8080", nil)
}
