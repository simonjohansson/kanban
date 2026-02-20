package server_test

import (
	"database/sql"
	"net/http"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/stretchr/testify/require"
)

func TestRebuildProjectionFromMarkdown(t *testing.T) {
	t.Parallel()

	_, sqlitePath, httpServer := newTestServer(t)

	createProjectResp := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "Rebuild"})
	require.Equal(t, http.StatusCreated, createProjectResp.StatusCode)

	createCardResp := doJSON(t, httpServer.URL+"/projects/rebuild/cards", http.MethodPost, map[string]string{
		"title":       "Test rebuild",
		"description": "Projection should be recoverable",
		"status":      "Todo",
	})
	require.Equal(t, http.StatusCreated, createCardResp.StatusCode)

	db, err := sql.Open("sqlite", sqlitePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`DELETE FROM cards; DELETE FROM projects;`)
	require.NoError(t, err)

	rebuildResp := doJSON(t, httpServer.URL+"/admin/rebuild", http.MethodPost, nil)
	require.Equal(t, http.StatusOK, rebuildResp.StatusCode)

	listResp := doJSON(t, httpServer.URL+"/projects/rebuild/cards", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	listBody := decodeMap(t, listResp.Body)
	require.Len(t, listBody["cards"].([]any), 1)

	var projectCount, cardCount int
	err = db.QueryRow(`SELECT count(*) FROM projects`).Scan(&projectCount)
	require.NoError(t, err)
	err = db.QueryRow(`SELECT count(*) FROM cards`).Scan(&cardCount)
	require.NoError(t, err)
	require.Equal(t, 1, projectCount)
	require.Equal(t, 1, cardCount)
}
