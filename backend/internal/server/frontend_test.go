package server_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFrontendIndexServedAtRoot(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)
	resp, err := http.Get(httpServer.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	body := string(readBody(t, resp.Body))
	require.Contains(t, body, "<div id=\"app\"></div>")
	require.Contains(t, body, "<title>Kanban Web</title>")
}

func TestFrontendSPAFallbackForUnknownNonAPIRoute(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)
	resp, err := http.Get(httpServer.URL + "/app/projects")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	body := string(readBody(t, resp.Body))
	require.Contains(t, body, "<div id=\"app\"></div>")
}

func TestFrontendFallbackDoesNotOverrideHealthAPI(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)
	resp, err := http.Get(httpServer.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	body := string(readBody(t, resp.Body))
	require.True(t, strings.Contains(body, "\"ok\":true") || strings.Contains(body, "\"ok\": true"))
}

func TestFrontendFallbackDoesNotOverrideClientConfigAPI(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)
	resp, err := http.Get(httpServer.URL + "/client-config")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	body := string(readBody(t, resp.Body))
	require.Contains(t, body, "\"server_url\"")
}
