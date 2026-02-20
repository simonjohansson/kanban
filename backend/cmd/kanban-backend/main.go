package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/simonjohansson/kanban/backend/internal/server"
)

func main() {
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
		log.Fatalf("create sqlite parent dir: %v", err)
	}

	app, err := server.New(server.Options{DataDir: dataDir, SQLitePath: sqlitePath})
	if err != nil {
		log.Fatalf("init server: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			log.Printf("close server: %v", err)
		}
	}()

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		_ = httpServer.Close()
	}()
	<-shutdownDone
}
