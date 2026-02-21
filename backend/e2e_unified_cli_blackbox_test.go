package backend_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestKanbanShowsHelpByDefault(t *testing.T) {
	kanbanBin := buildKanbanBinary(t)

	result := runKanban(t, kanbanBin)
	require.Equal(t, 0, result.exitCode, result.combined)
	require.Contains(t, result.stdout, "Usage:")
	require.Contains(t, result.stdout, "kanban [command]")
	require.Contains(t, result.stdout, "serve")
	require.Contains(t, result.stdout, "project")
	require.Contains(t, result.stdout, "card")
	require.Contains(t, result.stdout, "watch")
	require.Contains(t, result.stdout, "primer")
}

func TestKanbanPrimerCommandSupportsJSON(t *testing.T) {
	kanbanBin := buildKanbanBinary(t)

	result := runKanban(t, kanbanBin, "--output", "json", "primer")
	require.Equal(t, 0, result.exitCode, result.combined)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(result.stdout)), &payload), result.stdout)
	require.Equal(t, "kanban", payload["name"])
	require.Equal(t, "machine", payload["mode"])
	require.Equal(t, "json", payload["default_output"])
	require.NotEmpty(t, payload["agent_prompt"])
}

func TestKanbanProjectAndCardFlowCallsBackend(t *testing.T) {
	kanbanBin := buildKanbanBinary(t)

	var mu sync.Mutex
	var requests []recordedRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		mu.Lock()
		requests = append(requests, recordedRequest{
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
			_, _ = w.Write([]byte(`{"name":"Alpha","slug":"alpha"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/projects/alpha/cards":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"alpha/card-1","number":1}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/projects/alpha/cards/1/move":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/projects/alpha/cards/1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"not found"}`))
		}
	}))
	defer server.Close()

	createProject := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"project", "create",
		"--name", "Alpha",
	)
	require.Equal(t, 0, createProject.exitCode, createProject.combined)

	createCard := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "create",
		"--project", "alpha",
		"--title", "Task",
		"--status", "Todo",
	)
	require.Equal(t, 0, createCard.exitCode, createCard.combined)

	moveCard := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "move",
		"--project", "alpha",
		"--id", "1",
		"--status", "Doing",
	)
	require.Equal(t, 0, moveCard.exitCode, moveCard.combined)

	deleteCard := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "delete",
		"--project", "alpha",
		"--id", "1",
		"--hard",
	)
	require.Equal(t, 0, deleteCard.exitCode, deleteCard.combined)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, requests, 4)
	require.Equal(t, recordedRequest{method: "POST", path: "/projects", query: "", body: `{"name":"Alpha"}`}, requests[0])
	require.Equal(t, "POST", requests[1].method)
	require.Equal(t, "/projects/alpha/cards", requests[1].path)
	require.Contains(t, requests[1].body, `"title":"Task"`)
	require.Contains(t, requests[1].body, `"status":"Todo"`)
	require.Equal(t, "PATCH", requests[2].method)
	require.Equal(t, "/projects/alpha/cards/1/move", requests[2].path)
	require.Contains(t, requests[2].body, `"status":"Doing"`)
	require.Equal(t, "DELETE", requests[3].method)
	require.Equal(t, "/projects/alpha/cards/1", requests[3].path)
	require.Equal(t, "hard=true", requests[3].query)
}

func TestKanbanCardTodoFlowCallsBackend(t *testing.T) {
	kanbanBin := buildKanbanBinary(t)

	var mu sync.Mutex
	var requests []recordedRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		mu.Lock()
		requests = append(requests, recordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.RawQuery,
			body:   strings.TrimSpace(string(body)),
		})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/projects/alpha/cards/1/todos":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":1,"text":"Write tests","completed":false}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/projects/alpha/cards/1/todos/1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"text":"Write tests","completed":true}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/projects/alpha/cards/1/todos/2":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":2,"text":"Review","completed":false}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/projects/alpha/cards/1/todos/1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"text":"Write tests","completed":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/projects/alpha/cards/1/todos":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"todos":[{"id":2,"text":"Review","completed":false}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"not found"}`))
		}
	}))
	defer server.Close()

	addTodo := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "todo", "add",
		"--project", "alpha",
		"--id", "1",
		"--body", "Write tests",
	)
	require.Equal(t, 0, addTodo.exitCode, addTodo.combined)

	doneTodo := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "todo", "done",
		"--project", "alpha",
		"--id", "1",
		"--todo-id", "1",
	)
	require.Equal(t, 0, doneTodo.exitCode, doneTodo.combined)

	undoTodo := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "todo", "undo",
		"--project", "alpha",
		"--id", "1",
		"--todo-id", "2",
	)
	require.Equal(t, 0, undoTodo.exitCode, undoTodo.combined)

	deleteTodo := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "todo", "rm",
		"--project", "alpha",
		"--id", "1",
		"--todo-id", "1",
	)
	require.Equal(t, 0, deleteTodo.exitCode, deleteTodo.combined)

	listTodos := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "todo", "ls",
		"--project", "alpha",
		"--id", "1",
	)
	require.Equal(t, 0, listTodos.exitCode, listTodos.combined)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, requests, 5)
	require.Equal(t, recordedRequest{
		method: "POST",
		path:   "/projects/alpha/cards/1/todos",
		query:  "",
		body:   `{"text":"Write tests"}`,
	}, requests[0])
	require.Equal(t, recordedRequest{
		method: "PATCH",
		path:   "/projects/alpha/cards/1/todos/1",
		query:  "",
		body:   `{"completed":true}`,
	}, requests[1])
	require.Equal(t, recordedRequest{
		method: "PATCH",
		path:   "/projects/alpha/cards/1/todos/2",
		query:  "",
		body:   `{"completed":false}`,
	}, requests[2])
	require.Equal(t, recordedRequest{
		method: "DELETE",
		path:   "/projects/alpha/cards/1/todos/1",
		query:  "",
		body:   "",
	}, requests[3])
	require.Equal(t, recordedRequest{
		method: "GET",
		path:   "/projects/alpha/cards/1/todos",
		query:  "",
		body:   "",
	}, requests[4])
}

func TestKanbanCardAcceptanceFlowCallsBackend(t *testing.T) {
	kanbanBin := buildKanbanBinary(t)

	var mu sync.Mutex
	var requests []recordedRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()

		mu.Lock()
		requests = append(requests, recordedRequest{
			method: r.Method,
			path:   r.URL.Path,
			query:  r.URL.RawQuery,
			body:   strings.TrimSpace(string(body)),
		})
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/projects/alpha/cards/1/acceptance":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":1,"text":"Requirement met","completed":false}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/projects/alpha/cards/1/acceptance/1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"text":"Requirement met","completed":true}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/projects/alpha/cards/1/acceptance/2":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":2,"text":"Verified","completed":false}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/projects/alpha/cards/1/acceptance/1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":1,"text":"Requirement met","completed":true}`))
		case r.Method == http.MethodGet && r.URL.Path == "/projects/alpha/cards/1/acceptance":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"acceptance_criteria":[{"id":2,"text":"Verified","completed":false}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"detail":"not found"}`))
		}
	}))
	defer server.Close()

	add := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "acceptance", "add",
		"--project", "alpha",
		"--id", "1",
		"--body", "Requirement met",
	)
	require.Equal(t, 0, add.exitCode, add.combined)

	done := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "ac", "done",
		"--project", "alpha",
		"--id", "1",
		"--criterion-id", "1",
	)
	require.Equal(t, 0, done.exitCode, done.combined)

	undo := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "acceptance", "undo",
		"--project", "alpha",
		"--id", "1",
		"--criterion-id", "2",
	)
	require.Equal(t, 0, undo.exitCode, undo.combined)

	rm := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "ac", "rm",
		"--project", "alpha",
		"--id", "1",
		"--criterion-id", "1",
	)
	require.Equal(t, 0, rm.exitCode, rm.combined)

	ls := runKanban(t, kanbanBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "acceptance", "ls",
		"--project", "alpha",
		"--id", "1",
	)
	require.Equal(t, 0, ls.exitCode, ls.combined)

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, requests, 5)
	require.Equal(t, recordedRequest{
		method: "POST",
		path:   "/projects/alpha/cards/1/acceptance",
		query:  "",
		body:   `{"text":"Requirement met"}`,
	}, requests[0])
	require.Equal(t, recordedRequest{
		method: "PATCH",
		path:   "/projects/alpha/cards/1/acceptance/1",
		query:  "",
		body:   `{"completed":true}`,
	}, requests[1])
	require.Equal(t, recordedRequest{
		method: "PATCH",
		path:   "/projects/alpha/cards/1/acceptance/2",
		query:  "",
		body:   `{"completed":false}`,
	}, requests[2])
	require.Equal(t, recordedRequest{
		method: "DELETE",
		path:   "/projects/alpha/cards/1/acceptance/1",
		query:  "",
		body:   "",
	}, requests[3])
	require.Equal(t, recordedRequest{
		method: "GET",
		path:   "/projects/alpha/cards/1/acceptance",
		query:  "",
		body:   "",
	}, requests[4])
}

func TestKanbanWatchExitsOnInterrupt(t *testing.T) {
	kanbanBin := buildKanbanBinary(t)

	connected := make(chan struct{}, 1)
	upgrader := websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool { return true },
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			http.NotFound(w, r)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		connected <- struct{}{}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	cmd := exec.Command(kanbanBin, "--server-url", server.URL, "--output", "json", "watch")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-waitCh
		require.Fail(t, "watch did not connect")
	}

	require.NoError(t, cmd.Process.Signal(os.Interrupt))

	select {
	case err := <-waitCh:
		require.NoError(t, err, stdout.String()+stderr.String())
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-waitCh
		require.Fail(t, "watch did not exit after interrupt")
	}
}

type runResult struct {
	exitCode int
	stdout   string
	stderr   string
	combined string
}

type recordedRequest struct {
	method string
	path   string
	query  string
	body   string
}

func buildKanbanBinary(t *testing.T) string {
	t.Helper()

	binPath := filepath.Join(t.TempDir(), "kanban")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/kanban")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return binPath
}

func runKanban(t *testing.T, kanbanBin string, args ...string) runResult {
	t.Helper()

	cmd := exec.Command(kanbanBin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		require.True(t, ok, err)
		code = exitErr.ExitCode()
	}

	return runResult{
		exitCode: code,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		combined: stdout.String() + stderr.String(),
	}
}
