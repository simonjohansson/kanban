package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAPIYamlEndpoint(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)

	resp := doJSON(t, httpServer.URL+"/openapi.yaml", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.True(t, strings.Contains(resp.Header.Get("Content-Type"), "yaml"))

	raw := readBody(t, resp.Body)
	require.Contains(t, string(raw), "openapi:")
	require.Contains(t, string(raw), "/projects:")
	require.Contains(t, string(raw), "/ws:")
}
