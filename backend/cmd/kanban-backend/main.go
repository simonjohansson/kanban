package main

import (
	"flag"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/simonjohansson/kanban/backend/internal/server"
	"github.com/simonjohansson/kanban/backend/pkg/kanbanconfig"
)

const defaultListenAddr = "127.0.0.1:8080"

type runtimeDefaults struct {
	Addr       string
	CardsPath  string
	SQLitePath string
}

func loadRuntimeDefaults(home string) (runtimeDefaults, error) {
	cfg, err := kanbanconfig.LoadOrInit(home)
	if err != nil {
		return runtimeDefaults{}, err
	}
	return runtimeDefaults{
		Addr:       addrFromServerURL(cfg.ServerURL),
		CardsPath:  cfg.Backend.CardsPath,
		SQLitePath: cfg.Backend.SQLitePath,
	}, nil
}

func addrFromServerURL(serverURL string) string {
	raw := strings.TrimSpace(serverURL)
	if raw == "" {
		return defaultListenAddr
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return defaultListenAddr
	}

	host := u.Host
	if _, _, splitErr := net.SplitHostPort(host); splitErr == nil {
		return host
	}

	switch u.Scheme {
	case "https":
		return net.JoinHostPort(host, "443")
	case "http":
		return net.JoinHostPort(host, "80")
	default:
		return defaultListenAddr
	}
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var (
		addr       string
		cardsPath  string
		sqlitePath string
	)

	home, err := os.UserHomeDir()
	if err != nil {
		logger.Error("resolve user home failed", "error", err)
		os.Exit(1)
	}
	defaults, err := loadRuntimeDefaults(home)
	if err != nil {
		logger.Error("load shared config failed", "error", err)
		os.Exit(1)
	}

	flag.StringVar(&addr, "addr", defaults.Addr, "server listen address")
	flag.StringVar(&cardsPath, "cards-path", defaults.CardsPath, "directory for markdown source-of-truth files")
	flag.StringVar(&cardsPath, "data-dir", defaults.CardsPath, "deprecated alias for --cards-path")
	flag.StringVar(&sqlitePath, "sqlite-path", defaults.SQLitePath, "sqlite projection database path")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		logger.Error("create sqlite parent dir failed", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cardsPath, 0o755); err != nil {
		logger.Error("create cards dir failed", "error", err)
		os.Exit(1)
	}

	app, err := server.New(server.Options{DataDir: cardsPath, SQLitePath: sqlitePath, Logger: logger})
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
	logger.Info("starting kanban backend", "addr", addr, "cards_path", cardsPath, "sqlite_path", sqlitePath)

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
