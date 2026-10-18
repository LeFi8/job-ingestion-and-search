package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/config"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/httpapi"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/storage"
)

const (
	shutdownTimeout = 10 * time.Second
	requestTimeout  = 5 * time.Second
	connectTimeout  = 30 * time.Second
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe /healthz on API_ADDR and exit (container healthcheck)")
	flag.Parse()

	cfg, err := config.LoadAPI()
	if err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}

	if *healthcheck {
		if err := probe(cfg.Addr); err != nil {
			slog.Error("healthcheck failed", "error", err)
			os.Exit(1)
		}
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err = run(ctx, cfg)
	stop()
	if err != nil {
		slog.Error("api failed", "error", err)
		os.Exit(1)
	}
	slog.Info("api stopped")
}

func run(ctx context.Context, cfg config.API) error {
	log := slog.Default()

	es, err := search.New(cfg.ElasticsearchURL)
	if err != nil {
		return err
	}

	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	store, err := storage.Open(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	gin.SetMode(gin.ReleaseMode)
	router, err := httpapi.NewRouter(es, store, requestTimeout, log)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Info("api listening", "addr", cfg.Addr)

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

func probe(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("parse API_ADDR: %w", err)
	}
	if host == "" {
		host = "localhost"
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
