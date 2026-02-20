package server_test

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"
)

func TestDeleteProjectRemovesMarkdownAndProjection(t *testing.T) {
	t.Parallel()

	dataDir, sqlitePath, httpServer := newTestServer(t)

	createProject := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "To Remove"})
	require.Equal(t, http.StatusCreated, createProject.StatusCode)

	createCard := doJSON(t, httpServer.URL+"/projects/to-remove/cards", http.MethodPost, map[string]string{
		"title":  "card in project",
		"status": "Todo",
	})
	require.Equal(t, http.StatusCreated, createCard.StatusCode)

	db, err := sql.Open("sqlite", sqlitePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	deleteProject := doJSON(t, httpServer.URL+"/projects/to-remove", http.MethodDelete, nil)
	require.Equal(t, http.StatusOK, deleteProject.StatusCode)

	_, err = os.Stat(filepath.Join(dataDir, "projects", "to-remove"))
	require.ErrorIs(t, err, os.ErrNotExist)

	listProjects := doJSON(t, httpServer.URL+"/projects", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, listProjects.StatusCode)
	projectsBody := decodeMap(t, listProjects.Body)
	require.Len(t, projectsBody["projects"].([]any), 0)

	var projectCount, cardCount int
	err = db.QueryRow(`SELECT count(*) FROM projects WHERE slug = 'to-remove'`).Scan(&projectCount)
	require.NoError(t, err)
	err = db.QueryRow(`SELECT count(*) FROM cards WHERE project_slug = 'to-remove'`).Scan(&cardCount)
	require.NoError(t, err)
	require.Equal(t, 0, projectCount)
	require.Equal(t, 0, cardCount)

	deleteMissing := doJSON(t, httpServer.URL+"/projects/to-remove", http.MethodDelete, nil)
	require.Equal(t, http.StatusNotFound, deleteMissing.StatusCode)
}
