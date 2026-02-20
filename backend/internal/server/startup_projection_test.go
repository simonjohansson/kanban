package server_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/simonjohansson/kanban/backend/internal/server"
	"github.com/simonjohansson/kanban/backend/internal/store"
	"github.com/stretchr/testify/require"
)

func TestServerStartupRecreatesProjectionFromMarkdown(t *testing.T) {
	dataDir := t.TempDir()
	sqlitePath := filepath.Join(dataDir, "projection.db")

	markdownStore, err := store.NewMarkdownStore(dataDir)
	require.NoError(t, err)

	_, err = markdownStore.CreateProject("Alpha", "", "")
	require.NoError(t, err)
	_, err = markdownStore.CreateCard("alpha", "Recovered card", "from markdown", "", "Todo")
	require.NoError(t, err)

	createLegacyProjectionDB(t, sqlitePath)

	app, err := server.New(server.Options{DataDir: dataDir, SQLitePath: sqlitePath})
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	httpServer := httptest.NewServer(app.Handler())
	t.Cleanup(httpServer.Close)

	resp := doJSON(t, httpServer.URL+"/projects/alpha/cards", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	payload := decodeMap(t, resp.Body)
	rawCards, ok := payload["cards"].([]any)
	require.True(t, ok)
	require.Len(t, rawCards, 1)

	card, ok := rawCards[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Recovered card", card["title"])

	db, err := sql.Open("sqlite", sqlitePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	rows, err := db.Query("PRAGMA table_info(cards)")
	require.NoError(t, err)
	defer rows.Close()

	var (
		colName        string
		foundStatus    bool
		foundColumnOld bool
	)
	for rows.Next() {
		var cid int
		var colType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		require.NoError(t, rows.Scan(&cid, &colName, &colType, &notNull, &defaultValue, &pk))
		if colName == "status" {
			foundStatus = true
		}
		if colName == "column_name" {
			foundColumnOld = true
		}
	}
	require.NoError(t, rows.Err())
	require.True(t, foundStatus)
	require.False(t, foundColumnOld)
}

func createLegacyProjectionDB(t *testing.T, sqlitePath string) {
	t.Helper()

	db, err := sql.Open("sqlite", sqlitePath)
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`
CREATE TABLE projects (
  slug TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  local_path TEXT,
  remote_url TEXT,
  next_card_seq INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
CREATE TABLE cards (
  id TEXT PRIMARY KEY,
  project_slug TEXT NOT NULL,
  number INTEGER NOT NULL,
  title TEXT NOT NULL,
  status TEXT NOT NULL,
  column_name TEXT NOT NULL,
  deleted INTEGER NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  comments_count INTEGER NOT NULL,
  history_count INTEGER NOT NULL,
  UNIQUE(project_slug, number)
);
INSERT INTO projects (slug, name, next_card_seq, created_at, updated_at) VALUES ('legacy', 'Legacy', 1, ?, ?);
INSERT INTO cards (id, project_slug, number, title, status, column_name, deleted, created_at, updated_at, comments_count, history_count)
VALUES ('legacy/card-1', 'legacy', 1, 'legacy card', 'Todo', 'Todo', 0, ?, ?, 0, 0);
`, now, now, now, now)
	require.NoError(t, err)
}
