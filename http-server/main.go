package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

type config struct {
	addr            string
	maxInFlight     int
	requestTimeout  time.Duration
	shutdownTimeout time.Duration
}

func loadConfig() (config, error) {
	cfg := config{
		addr:            ":8080",
		maxInFlight:     100,
		requestTimeout:  10 * time.Second,
		shutdownTimeout: 15 * time.Second,
	}

	if v := os.Getenv("ADDR"); v != "" {
		cfg.addr = v
	}
	if v := os.Getenv("MAX_IN_FLIGHT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return cfg, fmt.Errorf("MAX_IN_FLIGHT must be a positive integer, got %q", v)
		}
		cfg.maxInFlight = n
	}
	for name, dst := range map[string]*time.Duration{
		"REQUEST_TIMEOUT":  &cfg.requestTimeout,
		"SHUTDOWN_TIMEOUT": &cfg.shutdownTimeout,
	} {
		if v := os.Getenv(name); v != "" {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return cfg, fmt.Errorf("%s must be a positive duration like 10s, got %q", name, v)
			}
			*dst = d
		}
	}
	return cfg, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(context.Background(), logger); err != nil {
		logger.Error("server exited", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// ctx is cancelled on ctrl+c or SIGTERM (what docker/k8s send on stop)
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	var ready atomic.Bool
	ready.Store(true)

	srv := &http.Server{
		Addr:    cfg.addr,
		Handler: newHandler(cfg, &ready, logger),

		// without these a slow client can hold a connection (and its goroutine) open forever
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      cfg.requestTimeout + 5*time.Second, // must outlast the request timeout
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,

		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	// ListenAndServe blocks, so run it in a goroutine and get the error back on a channel
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", slog.String("addr", cfg.addr), slog.Int("max_in_flight", cfg.maxInFlight))
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		// failed to start (port in use etc)
		return err
	case <-ctx.Done():
		stop() // second ctrl+c now kills the process right away
	}

	logger.Info("shutdown signal received, draining in-flight requests", slog.Duration("timeout", cfg.shutdownTimeout))
	ready.Store(false) // /readyz starts returning 503 so the load balancer stops sending traffic

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.shutdownTimeout)
	defer cancel()

	// stops accepting new connections and waits for active requests
	if err := srv.Shutdown(shutdownCtx); err != nil {
		srv.Close() // drain timed out, drop whatever is left
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}
	if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	logger.Info("server stopped cleanly")
	return nil
}
