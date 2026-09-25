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

//middleware for prom
func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// record the metric based on the requested URL
		requestCount.WithLabelValues(r.URL.Path).Inc()

		next.ServeHTTP(w, r)
	})
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("hello!"))
	slog.Info("Request handled", slog.String("path", r.URL.Path))
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	http.Handle("/hello", prometheusMiddleware(http.HandlerFunc(helloHandler)))

	// metrics endpoint for prom
	http.Handle("/metrics", promhttp.Handler())

	slog.Info("Starting server on :8080")
	http.ListenAndServe(":8080", nil)
}
