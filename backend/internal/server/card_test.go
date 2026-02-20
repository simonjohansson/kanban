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

func TestCardLifecyclePersistsMarkdownAndProjection(t *testing.T) {
	t.Parallel()

	dataDir, sqlitePath, httpServer := newTestServer(t)

	createProjectResp := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "Infra Board"})
	require.Equal(t, http.StatusCreated, createProjectResp.StatusCode)

	createCardResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards", http.MethodPost, map[string]string{
		"title":       "Set up CI",
		"description": "Wire up first CI pipeline",
		"status":      "Todo",
	})
	require.Equal(t, http.StatusCreated, createCardResp.StatusCode)
	createCardBody := decodeMap(t, createCardResp.Body)
	require.Equal(t, "infra-board/card-1", createCardBody["id"])
	require.NotContains(t, createCardBody, "column")

	cardPath := filepath.Join(dataDir, "projects", "infra-board", "card-1.md")
	cardMarkdown, err := os.ReadFile(cardPath)
	require.NoError(t, err)
	require.Contains(t, string(cardMarkdown), "# Description")
	require.Contains(t, string(cardMarkdown), "# Comments")
	require.Contains(t, string(cardMarkdown), "# History")
	require.Contains(t, string(cardMarkdown), "card.created")

	commentResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards/1/comments", http.MethodPost, map[string]string{"body": "Needs Linux and macOS jobs"})
	require.Equal(t, http.StatusOK, commentResp.StatusCode)

	descResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards/1/description", http.MethodPatch, map[string]string{"body": "Also add branch protection checks"})
	require.Equal(t, http.StatusOK, descResp.StatusCode)

	moveResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards/1/move", http.MethodPatch, map[string]string{"status": "Doing"})
	require.Equal(t, http.StatusOK, moveResp.StatusCode)

	getCardResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards/1", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, getCardResp.StatusCode)
	getCardBody := decodeMap(t, getCardResp.Body)
	require.Equal(t, "Doing", getCardBody["status"])
	require.NotContains(t, getCardBody, "column")
	require.Len(t, getCardBody["description"].([]any), 2)
	require.Len(t, getCardBody["comments"].([]any), 1)

	softDeleteResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards/1", http.MethodDelete, nil)
	require.Equal(t, http.StatusOK, softDeleteResp.StatusCode)

	listActiveResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, listActiveResp.StatusCode)
	listActiveBody := decodeMap(t, listActiveResp.Body)
	require.Len(t, listActiveBody["cards"].([]any), 0)

	listDeletedResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards?include_deleted=true", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, listDeletedResp.StatusCode)
	listDeletedBody := decodeMap(t, listDeletedResp.Body)
	require.Len(t, listDeletedBody["cards"].([]any), 1)

	db, err := sql.Open("sqlite", sqlitePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var deleted int
	err = db.QueryRow(`SELECT deleted FROM cards WHERE project_slug = 'infra-board' AND number = 1`).Scan(&deleted)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)

	hardDeleteResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards/1?hard=true", http.MethodDelete, nil)
	require.Equal(t, http.StatusOK, hardDeleteResp.StatusCode)

	_, err = os.Stat(cardPath)
	require.ErrorIs(t, err, os.ErrNotExist)

	listAfterHardResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards?include_deleted=true", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, listAfterHardResp.StatusCode)
	listAfterHardBody := decodeMap(t, listAfterHardResp.Body)
	require.Len(t, listAfterHardBody["cards"].([]any), 0)

	var count int
	err = db.QueryRow(`SELECT count(*) FROM cards WHERE project_slug = 'infra-board' AND number = 1`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	finalMarkdown := readFile(t, filepath.Join(dataDir, "projects", "infra-board", "project.md"))
	require.Contains(t, string(finalMarkdown), "next_card_seq: 2")
}

func TestCardEndpointsRejectColumnInRequests(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)

	createProjectResp := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "Infra Board"})
	require.Equal(t, http.StatusCreated, createProjectResp.StatusCode)

	createCardResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards", http.MethodPost, map[string]string{
		"title":  "Set up CI",
		"status": "Todo",
		"column": "Todo",
	})
	require.Equal(t, http.StatusUnprocessableEntity, createCardResp.StatusCode)

	createWithoutColumnResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards", http.MethodPost, map[string]string{
		"title":  "Set up CI",
		"status": "Todo",
	})
	require.Equal(t, http.StatusCreated, createWithoutColumnResp.StatusCode)

	moveResp := doJSON(t, httpServer.URL+"/projects/infra-board/cards/1/move", http.MethodPatch, map[string]string{
		"status": "Doing",
		"column": "Doing",
	})
	require.Equal(t, http.StatusUnprocessableEntity, moveResp.StatusCode)
}

func TestCardBranchCreateAndUpdate(t *testing.T) {
	t.Parallel()

	_, sqlitePath, httpServer := newTestServer(t)

	createProjectResp := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "Branch Board"})
	require.Equal(t, http.StatusCreated, createProjectResp.StatusCode)

	createCardResp := doJSON(t, httpServer.URL+"/projects/branch-board/cards", http.MethodPost, map[string]string{
		"title":  "Build branch support",
		"status": "Todo",
		"branch": "feature/card-branch",
	})
	require.Equal(t, http.StatusCreated, createCardResp.StatusCode)
	createCardBody := decodeMap(t, createCardResp.Body)
	require.Equal(t, "feature/card-branch", createCardBody["branch"])

	getCardResp := doJSON(t, httpServer.URL+"/projects/branch-board/cards/1", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, getCardResp.StatusCode)
	getCardBody := decodeMap(t, getCardResp.Body)
	require.Equal(t, "feature/card-branch", getCardBody["branch"])

	updateBranchResp := doJSON(t, httpServer.URL+"/projects/branch-board/cards/1/branch", http.MethodPatch, map[string]string{
		"branch": "feature/card-branch-v2",
	})
	require.Equal(t, http.StatusOK, updateBranchResp.StatusCode)
	updateBranchBody := decodeMap(t, updateBranchResp.Body)
	require.Equal(t, "feature/card-branch-v2", updateBranchBody["branch"])

	listCardsResp := doJSON(t, httpServer.URL+"/projects/branch-board/cards", http.MethodGet, nil)
	require.Equal(t, http.StatusOK, listCardsResp.StatusCode)
	listCardsBody := decodeMap(t, listCardsResp.Body)
	cards := listCardsBody["cards"].([]any)
	require.Len(t, cards, 1)
	cardSummary := cards[0].(map[string]any)
	require.Equal(t, "feature/card-branch-v2", cardSummary["branch"])

	invalidBranchResp := doJSON(t, httpServer.URL+"/projects/branch-board/cards/1/branch", http.MethodPatch, map[string]string{
		"branch": "bad branch name",
	})
	require.Equal(t, http.StatusBadRequest, invalidBranchResp.StatusCode)

	db, err := sql.Open("sqlite", sqlitePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var branch string
	err = db.QueryRow(`SELECT branch FROM cards WHERE project_slug = 'branch-board' AND number = 1`).Scan(&branch)
	require.NoError(t, err)
	require.Equal(t, "feature/card-branch-v2", branch)
}
