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
	binaryPath := filepath.Join(t.TempDir(), "kanban-backend")

	build := exec.Command("go", "build", "-o", binaryPath, "./cmd/kanban-backend")
	build.Dir = "/Users/simonjohansson/src/kanban/backend"
	buildOut, err := build.CombinedOutput()
	require.NoError(t, err, string(buildOut))

	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cmd := exec.CommandContext(ctx, binaryPath, "--addr", addr, "--data-dir", dataDir, "--sqlite-path", sqlitePath)
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
		"column":      "Todo",
	})
	require.Equal(t, http.StatusCreated, createCard.StatusCode)

	cards := doJSONRequest(t, baseURL+"/projects/blackbox/cards", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, cards.StatusCode)
	cardsMap := decodeBodyMap(t, cards.Body)
	require.Len(t, cardsMap["cards"].([]any), 1)

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
