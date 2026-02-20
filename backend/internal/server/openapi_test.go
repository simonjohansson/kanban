package server_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAPIYamlEndpoint(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)

	resp := doJSON(t, httpServer.URL+"/openapi.yaml", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/yaml", resp.Header.Get("Content-Type"))

	raw := readBody(t, resp.Body)
	require.Contains(t, string(raw), "openapi: 3.1.0")
	require.Contains(t, string(raw), "/projects:")
	require.Contains(t, string(raw), "/ws:")
}
