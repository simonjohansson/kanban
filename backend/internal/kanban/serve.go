package kanban

import (
	"errors"
	"fmt"
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
	"github.com/spf13/cobra"
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

func newServeCommand(cfg *Config) *cobra.Command {
	addr := addrFromServerURL(cfg.ServerURL)
	cardsPath := cfg.CardsPath
	sqlitePath := cfg.SQLitePath

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Kanban backend API server.",
		Long:  "Runs the backend API server with markdown storage and sqlite projection.",
		Example: strings.TrimSpace(`kanban serve
kanban serve --addr 127.0.0.1:8090
kanban --server-url http://127.0.0.1:9010 serve
kanban serve --cards-path /tmp/kanban/cards --sqlite-path /tmp/kanban/projection.db`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			serveAddr := strings.TrimSpace(addr)
			serveCards := strings.TrimSpace(cardsPath)
			serveSQLite := strings.TrimSpace(sqlitePath)

			if !cmd.Flags().Changed("addr") {
				serveAddr = addrFromServerURL(cfg.ServerURL)
			}
			if !cmd.Flags().Changed("cards-path") && !cmd.Flags().Changed("data-dir") {
				serveCards = strings.TrimSpace(cfg.CardsPath)
			}
			if !cmd.Flags().Changed("sqlite-path") {
				serveSQLite = strings.TrimSpace(cfg.SQLitePath)
			}

			if serveAddr == "" {
				return errors.New("--addr cannot be empty")
			}
			if serveCards == "" {
				return errors.New("--cards-path cannot be empty")
			}
			if serveSQLite == "" {
				return errors.New("--sqlite-path cannot be empty")
			}

			return runServe(serveAddr, serveCards, serveSQLite)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", addr, "server listen address")
	cmd.Flags().StringVar(&cardsPath, "cards-path", cardsPath, "directory for markdown source-of-truth files")
	cmd.Flags().StringVar(&cardsPath, "data-dir", cardsPath, "deprecated alias for --cards-path")
	cmd.Flags().StringVar(&sqlitePath, "sqlite-path", sqlitePath, "sqlite projection database path")
	return cmd
}

func runServe(addr, cardsPath, sqlitePath string) error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		return fmt.Errorf("create sqlite parent dir failed: %w", err)
	}
	if err := os.MkdirAll(cardsPath, 0o755); err != nil {
		return fmt.Errorf("create cards dir failed: %w", err)
	}

	app, err := server.New(server.Options{DataDir: cardsPath, SQLitePath: sqlitePath, Logger: logger})
	if err != nil {
		return fmt.Errorf("init server failed: %w", err)
	}
	defer func() {
		if closeErr := app.Close(); closeErr != nil {
			logger.Error("close server failed", "error", closeErr)
		}
	}()

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("starting kanban backend", "addr", addr, "cards_path", cardsPath, "sqlite_path", sqlitePath)

	serverErrCh := make(chan error, 1)
	go func() {
		if listenErr := httpServer.ListenAndServe(); listenErr != nil && listenErr != http.ErrServerClosed {
			serverErrCh <- listenErr
			return
		}
		serverErrCh <- nil
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case listenErr := <-serverErrCh:
		if listenErr != nil {
			return fmt.Errorf("listen failed: %w", listenErr)
		}
		return nil
	case sig := <-sigCh:
		logger.Info("shutdown signal received", "signal", sig.String())
	}

	if err := httpServer.Close(); err != nil {
		return fmt.Errorf("http server close failed: %w", err)
	}
	if listenErr := <-serverErrCh; listenErr != nil {
		return fmt.Errorf("listen failed after shutdown: %w", listenErr)
	}
	logger.Info("server stopped")
	return nil
}
