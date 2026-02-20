package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/simonjohansson/kanban/backend/internal/server"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) (dataDir string, sqlitePath string, httpServer *httptest.Server) {
	t.Helper()
	dataDir = t.TempDir()
	sqlitePath = filepath.Join(dataDir, "projection.db")
	app, err := server.New(server.Options{DataDir: dataDir, SQLitePath: sqlitePath})
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })
	httpServer = httptest.NewServer(app.Handler())
	t.Cleanup(httpServer.Close)
	return dataDir, sqlitePath, httpServer
}

func newHTTPTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)
	return httpServer
}

func doJSON(t *testing.T, url, method string, payload any) *http.Response {
	t.Helper()

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		require.NoError(t, err)
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, body)
	require.NoError(t, err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func doRaw(t *testing.T, url, method, payload, contentType string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, bytes.NewBufferString(payload))
	require.NoError(t, err)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func decodeMap(t *testing.T, reader io.Reader) map[string]any {
	t.Helper()
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	var out map[string]any
	err = json.Unmarshal(data, &out)
	require.NoError(t, err)
	return out
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func readBody(t *testing.T, reader io.Reader) []byte {
	t.Helper()
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	return data
}
