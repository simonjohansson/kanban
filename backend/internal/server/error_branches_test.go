package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServerHandlesCorruptProjectMarkdown(t *testing.T) {
	t.Parallel()

	dataDir, _, httpServer := newTestServer(t)

	created := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "Corruptible"})
	require.Equal(t, http.StatusCreated, created.StatusCode)

	projectPath := filepath.Join(dataDir, "projects", "corruptible", "project.md")
	require.NoError(t, os.WriteFile(projectPath, []byte("not-frontmatter"), 0o644))

	list := doJSON(t, httpServer.URL+"/projects", http.MethodGet, nil)
	require.Equal(t, http.StatusInternalServerError, list.StatusCode)

	rebuild := doJSON(t, httpServer.URL+"/admin/rebuild", http.MethodPost, nil)
	require.Equal(t, http.StatusInternalServerError, rebuild.StatusCode)
}

func TestAdditionalHandlerBranches(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)

	emptyProjectName := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "   "})
	require.Equal(t, http.StatusBadRequest, emptyProjectName.StatusCode)

	createProject := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "Branches"})
	require.Equal(t, http.StatusCreated, createProject.StatusCode)

	createCardMissingProject := doJSON(t, httpServer.URL+"/projects/does-not-exist/cards", http.MethodPost, map[string]string{
		"title":  "x",
		"status": "Todo",
	})
	require.Equal(t, http.StatusBadRequest, createCardMissingProject.StatusCode)

	createCard := doJSON(t, httpServer.URL+"/projects/branches/cards", http.MethodPost, map[string]string{
		"title":  "card",
		"status": "Todo",
	})
	require.Equal(t, http.StatusCreated, createCard.StatusCode)

	commentBadJSON := doRaw(t, httpServer.URL+"/projects/branches/cards/1/comments", http.MethodPost, "{", "application/json")
	require.Equal(t, http.StatusBadRequest, commentBadJSON.StatusCode)

	commentMissingCard := doJSON(t, httpServer.URL+"/projects/branches/cards/999/comments", http.MethodPost, map[string]string{"body": "x"})
	require.Equal(t, http.StatusNotFound, commentMissingCard.StatusCode)

	descriptionBadJSON := doRaw(t, httpServer.URL+"/projects/branches/cards/1/description", http.MethodPatch, "{", "application/json")
	require.Equal(t, http.StatusBadRequest, descriptionBadJSON.StatusCode)

	descriptionMissingCard := doJSON(t, httpServer.URL+"/projects/branches/cards/999/description", http.MethodPatch, map[string]string{"body": "x"})
	require.Equal(t, http.StatusNotFound, descriptionMissingCard.StatusCode)

	moveMissingCard := doJSON(t, httpServer.URL+"/projects/branches/cards/999/move", http.MethodPatch, map[string]string{"status": "Todo"})
	require.Equal(t, http.StatusNotFound, moveMissingCard.StatusCode)

	deleteInvalidNumber := doJSON(t, httpServer.URL+"/projects/branches/cards/not-a-number", http.MethodDelete, nil)
	require.Equal(t, http.StatusUnprocessableEntity, deleteInvalidNumber.StatusCode)
}
