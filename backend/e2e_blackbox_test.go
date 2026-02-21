package backend_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"
)

func TestE2EBlackBoxServerProcess(t *testing.T) {
	dataDir := t.TempDir()
	sqlitePath := filepath.Join(dataDir, "projection.db")
	binaryPath := filepath.Join(t.TempDir(), "kanban")

	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/kanban")
	build.Dir = "."
	buildOut, err := build.CombinedOutput()
	require.NoError(t, err, string(buildOut))

	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, binaryPath, "serve", "--addr", addr, "--data-dir", dataDir, "--sqlite-path", sqlitePath)
	stdoutPipe, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderrPipe, err := cmd.StderrPipe()
	require.NoError(t, err)
	var streamWG sync.WaitGroup
	streamReaderToTestLogs(t, "backend stdout", stdoutPipe, &streamWG)
	streamReaderToTestLogs(t, "backend stderr", stderrPipe, &streamWG)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
		streamWG.Wait()
	})

	baseURL := "http://" + addr
	require.Eventually(t, func() bool {
		resp, err := http.Get(baseURL + "/health")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 10*time.Second, 100*time.Millisecond)

	createProject := doJSONRequest(t, baseURL+"/projects", http.MethodPost, map[string]string{
		"name":       "Blackbox",
		"local_path": "/tmp/blackbox",
	})
	require.Equal(t, http.StatusCreated, createProject.StatusCode)

	createCard := doJSONRequest(t, baseURL+"/projects/blackbox/cards", http.MethodPost, map[string]string{
		"title":       "exercise process",
		"description": "test real server",
		"status":      "Todo",
	})
	require.Equal(t, http.StatusCreated, createCard.StatusCode)

	addTodoA := doJSONRequest(t, baseURL+"/projects/blackbox/cards/1/todos", http.MethodPost, map[string]string{
		"text": "write e2e test",
	})
	require.Equal(t, http.StatusCreated, addTodoA.StatusCode)

	addTodoB := doJSONRequest(t, baseURL+"/projects/blackbox/cards/1/todos", http.MethodPost, map[string]string{
		"text": "run e2e test",
	})
	require.Equal(t, http.StatusCreated, addTodoB.StatusCode)

	completeTodo := doJSONRequest(t, baseURL+"/projects/blackbox/cards/1/todos/2", http.MethodPatch, map[string]bool{
		"completed": true,
	})
	require.Equal(t, http.StatusOK, completeTodo.StatusCode)

	addAC1 := doJSONRequest(t, baseURL+"/projects/blackbox/cards/1/acceptance", http.MethodPost, map[string]string{
		"text": "meets requirement A",
	})
	require.Equal(t, http.StatusCreated, addAC1.StatusCode)

	addAC2 := doJSONRequest(t, baseURL+"/projects/blackbox/cards/1/acceptance", http.MethodPost, map[string]string{
		"text": "meets requirement B",
	})
	require.Equal(t, http.StatusCreated, addAC2.StatusCode)

	completeAC := doJSONRequest(t, baseURL+"/projects/blackbox/cards/1/acceptance/2", http.MethodPatch, map[string]bool{
		"completed": true,
	})
	require.Equal(t, http.StatusOK, completeAC.StatusCode)

	listAcceptance := doJSONRequest(t, baseURL+"/projects/blackbox/cards/1/acceptance", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, listAcceptance.StatusCode)
	acceptanceMap := decodeBodyMap(t, listAcceptance.Body)
	require.Len(t, acceptanceMap["acceptance_criteria"].([]any), 2)

	listTodos := doJSONRequest(t, baseURL+"/projects/blackbox/cards/1/todos", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, listTodos.StatusCode)
	todosMap := decodeBodyMap(t, listTodos.Body)
	require.Len(t, todosMap["todos"].([]any), 2)

	getCard := doJSONRequest(t, baseURL+"/projects/blackbox/cards/1", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, getCard.StatusCode)
	getCardMap := decodeBodyMap(t, getCard.Body)
	require.Len(t, getCardMap["todos"].([]any), 2)
	require.Len(t, getCardMap["acceptance_criteria"].([]any), 2)

	cards := doJSONRequest(t, baseURL+"/projects/blackbox/cards", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, cards.StatusCode)
	cardsMap := decodeBodyMap(t, cards.Body)
	require.Len(t, cardsMap["cards"].([]any), 1)
	summary := cardsMap["cards"].([]any)[0].(map[string]any)
	require.Equal(t, float64(2), summary["todos_count"])
	require.Equal(t, float64(1), summary["todos_completed_count"])
	require.Equal(t, float64(2), summary["acceptance_criteria_count"])
	require.Equal(t, float64(1), summary["acceptance_criteria_completed_count"])

	_, err = os.Stat(filepath.Join(dataDir, "projects", "blackbox", "card-1.md"))
	require.NoError(t, err)

	db, err := sql.Open("sqlite", sqlitePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var count int
	err = db.QueryRow(`SELECT count(*) FROM cards WHERE project_slug = 'blackbox'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

func doJSONRequest(t *testing.T, url, method string, payload any) *http.Response {
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

func decodeBodyMap(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(body)
	require.NoError(t, err)
	var out map[string]any
	err = json.Unmarshal(raw, &out)
	require.NoError(t, err, fmt.Sprintf("failed to decode: %s", string(raw)))
	return out
}
