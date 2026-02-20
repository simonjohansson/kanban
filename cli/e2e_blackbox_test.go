package cli_test

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

func TestKBShowsHelpByDefault(t *testing.T) {
	kbBin := buildKBBinary(t)

	result := runKB(t, kbBin)
	require.Equal(t, 0, result.exitCode, result.combined)
	require.Contains(t, result.stdout, "Usage:")
	require.Contains(t, result.stdout, "kb [command]")
	require.Contains(t, result.stdout, "project")
	require.Contains(t, result.stdout, "card")
	require.Contains(t, result.stdout, "watch")
	require.Contains(t, result.stdout, "primer")
}

func TestKBHasExtensiveHelp(t *testing.T) {
	kbBin := buildKBBinary(t)

	rootHelp := runKB(t, kbBin, "--help")
	require.Equal(t, 0, rootHelp.exitCode, rootHelp.combined)
	require.Contains(t, rootHelp.stdout, "Flags:")
	require.Contains(t, rootHelp.stdout, "--server-url")
	require.Contains(t, rootHelp.stdout, "--output")
	require.Contains(t, rootHelp.stdout, "Examples:")
	require.NotContains(t, rootHelp.stdout, "--sqlite-path")
	require.NotContains(t, rootHelp.stdout, "--cards-path")

	nestedHelp := runKB(t, kbBin, "help", "card", "create")
	require.Equal(t, 0, nestedHelp.exitCode, nestedHelp.combined)
	require.Contains(t, nestedHelp.stdout, "Usage:")
	require.Contains(t, nestedHelp.stdout, "kb card create")
	require.Contains(t, nestedHelp.stdout, "--project")
	require.Contains(t, nestedHelp.stdout, "--title")
	require.Contains(t, nestedHelp.stdout, "--status")
}

func TestKBHelpListsAliases(t *testing.T) {
	kbBin := buildKBBinary(t)

	projectHelp := runKB(t, kbBin, "help", "project")
	require.Equal(t, 0, projectHelp.exitCode, projectHelp.combined)
	require.Contains(t, projectHelp.stdout, "Aliases:")
	require.Contains(t, projectHelp.stdout, "projects")
	require.Contains(t, projectHelp.stdout, "proj")

	deleteHelp := runKB(t, kbBin, "help", "card", "delete")
	require.Equal(t, 0, deleteHelp.exitCode, deleteHelp.combined)
	require.Contains(t, deleteHelp.stdout, "Aliases:")
	require.Contains(t, deleteHelp.stdout, "rm")
}

func TestKBPrimerCommandSupportsJSON(t *testing.T) {
	kbBin := buildKBBinary(t)

	result := runKB(t, kbBin, "--output", "json", "primer")
	require.Equal(t, 0, result.exitCode, result.combined)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(result.stdout)), &payload), result.stdout)
	require.Equal(t, "kb", payload["name"])
	require.NotEmpty(t, payload["purpose"])
	require.Equal(t, "machine", payload["mode"])
	require.Equal(t, "json", payload["default_output"])
	require.NotEmpty(t, payload["agent_prompt"])

	usage, ok := payload["usage"].(map[string]any)
	require.True(t, ok, result.stdout)
	globalFlags, ok := usage["global_flags"].([]any)
	require.True(t, ok, result.stdout)
	require.ElementsMatch(t, []any{"--server-url", "--output"}, globalFlags)

	rules, ok := payload["execution_rules"].([]any)
	require.True(t, ok, result.stdout)
	require.NotEmpty(t, rules)

	templates, ok := payload["command_templates"].(map[string]any)
	require.True(t, ok, result.stdout)
	require.Contains(t, templates, "list_projects")
	require.Contains(t, templates, "create_card")

	responseShapes, ok := payload["response_shapes"].(map[string]any)
	require.True(t, ok, result.stdout)
	require.Contains(t, responseShapes, "create_project")
	require.Contains(t, responseShapes, "create_card")
	require.Contains(t, responseShapes, "get_card")

	idSemantics, ok := payload["id_semantics"].(map[string]any)
	require.True(t, ok, result.stdout)
	require.Equal(t, "card identifier string (<project-slug>/card-<number>)", idSemantics["card_id"])
	require.Equal(t, "project-scoped integer sequence (1,2,3...)", idSemantics["card_number"])
	require.Equal(t, "all --id/-i flags expect card_number, not card_id", idSemantics["id_argument"])

	errorShape, ok := payload["error_shape"].(map[string]any)
	require.True(t, ok, result.stdout)
	require.Contains(t, errorShape, "backend_problem_json")
	require.Contains(t, errorShape, "cli_fallback_json")

	deleteSemantics, ok := payload["delete_semantics"].(map[string]any)
	require.True(t, ok, result.stdout)
	require.Equal(t, true, deleteSemantics["soft_delete_default"])
	require.Equal(t, true, deleteSemantics["hard_delete_flag"])

	descSemantics, ok := payload["desc_semantics"].(map[string]any)
	require.True(t, ok, result.stdout)
	require.Equal(t, "append", descSemantics["mode"])
	require.Equal(t, "kb --output json card get -p \"$PROJECT\" -i \"$ID\"", descSemantics["read_via"])

	projectSupport, ok := payload["project_command_support"].(map[string]any)
	require.True(t, ok, result.stdout)
	require.Equal(t, false, projectSupport["rename_supported"])

	watchShape, ok := payload["watch_event_shape"].(map[string]any)
	require.True(t, ok, result.stdout)
	require.Contains(t, watchShape, "type")
	require.Contains(t, watchShape, "project")
	require.Contains(t, watchShape, "card_id")
	require.Contains(t, watchShape, "card_number")
	require.Contains(t, watchShape, "timestamp")

	statusRules, ok := payload["status_rules"].(map[string]any)
	require.True(t, ok, result.stdout)
	require.Equal(t, true, statusRules["can_create_in_any_allowed_status"])

	require.Contains(t, strings.TrimSpace(result.stdout), "\"card_statuses\"")
}

func TestKBPrimerTextIsMachineOrientedForAgentUse(t *testing.T) {
	kbBin := buildKBBinary(t)

	result := runKB(t, kbBin, "primer")
	require.Equal(t, 0, result.exitCode, result.combined)

	require.Contains(t, result.stdout, "KB AGENT PRIMER")
	require.Contains(t, result.stdout, "SYSTEM PROMPT")
	require.Contains(t, result.stdout, "EXECUTION RULES")
	require.Contains(t, result.stdout, "COMMAND TEMPLATES")
	require.Contains(t, result.stdout, "RESPONSE SHAPES")
	require.Contains(t, result.stdout, "CARD ID SEMANTICS")
	require.Contains(t, result.stdout, "ERROR SHAPE")
	require.Contains(t, result.stdout, "DELETE SEMANTICS")
	require.Contains(t, result.stdout, "DESC SEMANTICS")
	require.Contains(t, result.stdout, "PROJECT COMMAND SUPPORT")
	require.Contains(t, result.stdout, "WATCH EVENT SHAPE")
	require.Contains(t, result.stdout, "Card statuses: Todo | Doing | Review | Done")
	require.Contains(t, result.stdout, "LIST_PROJECTS:")
	require.Contains(t, result.stdout, "CREATE_CARD:")
	require.Contains(t, result.stdout, "Cards may be created directly in any allowed status.")
	require.Contains(t, result.stdout, "--hard => permanent delete")
	require.Contains(t, result.stdout, "--output json")
}

func TestKBProjectAndCardFlowCallsBackend(t *testing.T) {
	kbBin := buildKBBinary(t)

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

	createProject := runKB(t, kbBin,
		"--server-url", server.URL,
		"--output", "json",
		"project", "create",
		"--name", "Alpha",
	)
	require.Equal(t, 0, createProject.exitCode, createProject.combined)

	createCard := runKB(t, kbBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "create",
		"--project", "alpha",
		"--title", "Task",
		"--status", "Todo",
	)
	require.Equal(t, 0, createCard.exitCode, createCard.combined)

	moveCard := runKB(t, kbBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "move",
		"--project", "alpha",
		"--id", "1",
		"--status", "Doing",
	)
	require.Equal(t, 0, moveCard.exitCode, moveCard.combined)

	deleteCard := runKB(t, kbBin,
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

func TestKBAliasesAndShortFlagsCallBackend(t *testing.T) {
	kbBin := buildKBBinary(t)

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

	createProject := runKB(t, kbBin,
		"--server-url", server.URL,
		"--output", "json",
		"proj", "create",
		"-n", "Alpha",
	)
	require.Equal(t, 0, createProject.exitCode, createProject.combined)

	createCard := runKB(t, kbBin,
		"--server-url", server.URL,
		"--output", "json",
		"cards", "create",
		"-p", "alpha",
		"-t", "Task",
		"-s", "Todo",
	)
	require.Equal(t, 0, createCard.exitCode, createCard.combined)

	moveCard := runKB(t, kbBin,
		"--server-url", server.URL,
		"--output", "json",
		"card", "move",
		"-p", "alpha",
		"-i", "1",
		"-s", "Doing",
	)
	require.Equal(t, 0, moveCard.exitCode, moveCard.combined)

	deleteCard := runKB(t, kbBin,
		"--server-url", server.URL,
		"--output", "json",
		"cards", "rm",
		"-p", "alpha",
		"-i", "1",
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

func TestKBWatchExitsOnInterrupt(t *testing.T) {
	kbBin := buildKBBinary(t)

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

	cmd := exec.Command(kbBin, "--server-url", server.URL, "--output", "json", "watch")
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

type recordedRequest struct {
	method string
	path   string
	query  string
	body   string
}

type runResult struct {
	exitCode int
	stdout   string
	stderr   string
	combined string
}

func buildKBBinary(t *testing.T) string {
	t.Helper()

	binPath := filepath.Join(t.TempDir(), "kb")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/kb")
	cmd.Dir = "/Users/simonjohansson/src/kanban/cli"
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return binPath
}

func runKB(t *testing.T, kbBin string, args ...string) runResult {
	t.Helper()

	cmd := exec.Command(kbBin, args...)
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
