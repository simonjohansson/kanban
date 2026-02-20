package server_test

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"
)

func TestProjectLifecyclePersistsMarkdownAndProjection(t *testing.T) {
	t.Parallel()

	dataDir, sqlitePath, httpServer := newTestServer(t)

	createResp := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{
		"name":       "Project One",
		"local_path": "/tmp/project-one",
		"remote_url": "https://example.com/project-one.git",
	})
	require.Equal(t, http.StatusCreated, createResp.StatusCode)
	createBody := decodeMap(t, createResp.Body)
	require.Equal(t, "project-one", createBody["slug"])

	projectMarkdownPath := filepath.Join(dataDir, "projects", "project-one", "project.md")
	rawProjectMarkdown := readFile(t, projectMarkdownPath)
	require.Contains(t, string(rawProjectMarkdown), "slug: project-one")
	require.Contains(t, string(rawProjectMarkdown), "local_path: /tmp/project-one")

	listResp := doJSON(t, httpServer.URL+"/projects", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	listBody := decodeMap(t, listResp.Body)
	projects := listBody["projects"].([]any)
	require.Len(t, projects, 1)

	db, err := sql.Open("sqlite", sqlitePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var count int
	err = db.QueryRow(`SELECT count(*) FROM projects WHERE slug = 'project-one'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
