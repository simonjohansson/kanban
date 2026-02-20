package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/simonjohansson/kanban/backend/internal/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var (
		addr       string
		dataDir    string
		sqlitePath string
	)

	defaultDataDir := filepath.Join(os.TempDir(), "kanban-data")
	flag.StringVar(&addr, "addr", "127.0.0.1:8080", "server listen address")
	flag.StringVar(&dataDir, "data-dir", defaultDataDir, "directory for markdown source-of-truth files")
	flag.StringVar(&sqlitePath, "sqlite-path", filepath.Join(defaultDataDir, "projection.db"), "sqlite projection database path")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		logger.Error("create sqlite parent dir failed", "error", err)
		os.Exit(1)
	}

	app, err := server.New(server.Options{DataDir: dataDir, SQLitePath: sqlitePath, Logger: logger})
	if err != nil {
		logger.Error("init server failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := app.Close(); err != nil {
			logger.Error("close server failed", "error", err)
		}
	}()

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("starting kanban backend", "addr", addr, "data_dir", dataDir, "sqlite_path", sqlitePath)

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen failed", "error", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("shutdown signal received", "signal", sig.String())

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		if err := httpServer.Close(); err != nil {
			logger.Error("http server close failed", "error", err)
		}
	}()
	<-shutdownDone
	logger.Info("server stopped")
}
