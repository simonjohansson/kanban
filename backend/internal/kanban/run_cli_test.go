package kanban

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type commandRequest struct {
	method string
	path   string
	query  string
	body   string
}

func TestRunExecutesProjectAndCardCommands(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var (
		mu       sync.Mutex
		requests []commandRequest
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		mu.Lock()
		requests = append(requests, commandRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.RawQuery,
			body:   strings.TrimSpace(string(body)),
		})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/projects":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name":"Alpha","slug":"alpha","next_card_seq":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/projects":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"projects":[{"name":"Alpha","slug":"alpha"}]}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/projects/alpha":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"project":"alpha","deleted":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/projects/alpha/cards":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"alpha/card-1","project":"alpha","number":1,"title":"Task","status":"Todo"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/projects/alpha/cards":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"cards":[{"id":"alpha/card-1","project":"alpha","number":1,"title":"Task","status":"Todo","deleted":false,"comments_count":0,"history_count":1}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/projects/alpha/cards/1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"alpha/card-1","project":"alpha","number":1,"title":"Task","status":"Todo","description":[],"comments":[],"history":[]}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/projects/alpha/cards/1/move":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"alpha/card-1","project":"alpha","number":1,"title":"Task","status":"Doing"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/projects/alpha/cards/1/comments":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"alpha/card-1","project":"alpha","number":1,"title":"Task","status":"Doing"}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/projects/alpha/cards/1/description":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"alpha/card-1","project":"alpha","number":1,"title":"Task","status":"Doing"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/projects/alpha/cards/1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"alpha/card-1","project":"alpha","number":1,"deleted":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"not found"}`))
		}
	}))
	defer server.Close()

	env := []string{"KANBAN_SERVER_URL=" + server.URL, "KANBAN_OUTPUT=json"}

	cases := [][]string{
		{"project", "create", "--name", "Alpha"},
		{"project", "ls"},
		{"card", "create", "-p", "alpha", "-t", "Task", "-s", "Todo"},
		{"card", "ls", "-p", "alpha"},
		{"card", "get", "-p", "alpha", "-i", "1"},
		{"card", "move", "-p", "alpha", "-i", "1", "-s", "Doing"},
		{"card", "comment", "-p", "alpha", "-i", "1", "-b", "note"},
		{"card", "desc", "-p", "alpha", "-i", "1", "-b", "body"},
		{"card", "rm", "-p", "alpha", "-i", "1", "--hard"},
		{"project", "rm", "alpha"},
	}

	for _, args := range cases {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		exitCode := Run(args, &stdout, &stderr, env)
		require.Equal(t, 0, exitCode, strings.Join(args, " ")+" stderr="+stderr.String())
		require.NotEmpty(t, strings.TrimSpace(stdout.String()))
	}

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, requests)
}

func TestRunReturnsJSONErrorForBackendProblem(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"type":"about:blank","title":"Unprocessable Entity","status":422,"detail":"bad input"}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"project", "create", "--name", "Alpha"}, &stdout, &stderr, []string{"KANBAN_SERVER_URL=" + server.URL, "KANBAN_OUTPUT=json"})
	require.Equal(t, 1, exitCode)
	require.Contains(t, stderr.String(), `"detail":"bad input"`)
}
