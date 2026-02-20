package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientConfigEndpoint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_, _, httpServer := newTestServer(t)

	missingConfig := doJSON(t, httpServer.URL+"/client-config", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, missingConfig.StatusCode)
	missingPayload := decodeMap(t, missingConfig.Body)
	require.Equal(t, "", missingPayload["server_url"])

	configPath := filepath.Join(home, ".config", "kanban", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte("server_url: http://127.0.0.1:9999\n"), 0o644))

	withConfig := doJSON(t, httpServer.URL+"/client-config", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, withConfig.StatusCode)
	withConfigPayload := decodeMap(t, withConfig.Body)
	require.Equal(t, "http://127.0.0.1:9999", withConfigPayload["server_url"])
}
