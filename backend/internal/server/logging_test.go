package server_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/simonjohansson/kanban/backend/internal/server"
	"github.com/stretchr/testify/require"
)

func TestRequestLoggingMiddleware(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dataDir := t.TempDir()
	sqlitePath := filepath.Join(dataDir, "projection.db")
	app, err := server.New(server.Options{DataDir: dataDir, SQLitePath: sqlitePath, Logger: logger})
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	httpServer := newHTTPTestServer(t, app.Handler())

	resp := doJSON(t, httpServer.URL+"/health", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	out := logs.String()
	require.Contains(t, out, "http request")
	require.Contains(t, out, "method=GET")
	require.Contains(t, out, "path=/health")
	require.Contains(t, out, "status=200")
}

func TestOperationLoggingForProjectAndCardLifecycle(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dataDir := t.TempDir()
	sqlitePath := filepath.Join(dataDir, "projection.db")
	app, err := server.New(server.Options{DataDir: dataDir, SQLitePath: sqlitePath, Logger: logger})
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	httpServer := newHTTPTestServer(t, app.Handler())

	createProjectResp := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "Log Me"})
	require.Equal(t, http.StatusCreated, createProjectResp.StatusCode)

	createCardResp := doJSON(t, httpServer.URL+"/projects/log-me/cards", http.MethodPost, map[string]string{
		"title":  "Observe logs",
		"status": "Todo",
	})
	require.Equal(t, http.StatusCreated, createCardResp.StatusCode)

	out := logs.String()
	require.Contains(t, out, "project created")
	require.Contains(t, out, "project=log-me")
	require.Contains(t, out, "card created")
	require.Contains(t, out, "card_id=log-me/card-1")
}
