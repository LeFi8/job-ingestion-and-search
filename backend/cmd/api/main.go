package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/config"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/httpapi"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search"
)

const shutdownTimeout = 10 * time.Second

func main() {
	cfg, err := config.LoadAPI()
	if err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
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

	gin.SetMode(gin.ReleaseMode)
	router, err := httpapi.NewRouter(es, log)
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
