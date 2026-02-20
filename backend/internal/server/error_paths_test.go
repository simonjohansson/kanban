package server_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestProjectAndCardErrorPaths(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)

	badJSONProject := doRaw(t, httpServer.URL+"/projects", http.MethodPost, "{", "application/json")
	require.Equal(t, http.StatusBadRequest, badJSONProject.StatusCode)
	require.Contains(t, string(readBody(t, badJSONProject.Body)), "unexpected end of JSON input")

	createProject := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "Err Board"})
	require.Equal(t, http.StatusCreated, createProject.StatusCode)

	duplicateProject := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "Err Board"})
	require.Equal(t, http.StatusConflict, duplicateProject.StatusCode)

	badJSONCard := doRaw(t, httpServer.URL+"/projects/err-board/cards", http.MethodPost, "{", "application/json")
	require.Equal(t, http.StatusBadRequest, badJSONCard.StatusCode)

	missingStatus := doJSON(t, httpServer.URL+"/projects/err-board/cards", http.MethodPost, map[string]string{"title": "No status"})
	require.Equal(t, http.StatusUnprocessableEntity, missingStatus.StatusCode)

	createCard := doJSON(t, httpServer.URL+"/projects/err-board/cards", http.MethodPost, map[string]string{
		"title":  "Happy path",
		"status": "Todo",
	})
	require.Equal(t, http.StatusCreated, createCard.StatusCode)

	invalidCardNumber := doJSON(t, httpServer.URL+"/projects/err-board/cards/abc", http.MethodGet, nil)
	require.Equal(t, http.StatusUnprocessableEntity, invalidCardNumber.StatusCode)
	zeroCardNumber := doJSON(t, httpServer.URL+"/projects/err-board/cards/0", http.MethodGet, nil)
	require.Equal(t, http.StatusBadRequest, zeroCardNumber.StatusCode)

	missingCard := doJSON(t, httpServer.URL+"/projects/err-board/cards/999", http.MethodGet, nil)
	require.Equal(t, http.StatusNotFound, missingCard.StatusCode)

	badMoveJSON := doRaw(t, httpServer.URL+"/projects/err-board/cards/1/move", http.MethodPatch, "{", "application/json")
	require.Equal(t, http.StatusBadRequest, badMoveJSON.StatusCode)

	invalidMoveStatus := doJSON(t, httpServer.URL+"/projects/err-board/cards/1/move", http.MethodPatch, map[string]string{"status": "Blocked"})
	require.Equal(t, http.StatusBadRequest, invalidMoveStatus.StatusCode)
	zeroMoveNumber := doJSON(t, httpServer.URL+"/projects/err-board/cards/0/move", http.MethodPatch, map[string]string{"status": "Todo"})
	require.Equal(t, http.StatusBadRequest, zeroMoveNumber.StatusCode)

	emptyComment := doJSON(t, httpServer.URL+"/projects/err-board/cards/1/comments", http.MethodPost, map[string]string{"body": "   "})
	require.Equal(t, http.StatusBadRequest, emptyComment.StatusCode)
	zeroCommentNumber := doJSON(t, httpServer.URL+"/projects/err-board/cards/0/comments", http.MethodPost, map[string]string{"body": "hi"})
	require.Equal(t, http.StatusBadRequest, zeroCommentNumber.StatusCode)

	emptyDescription := doJSON(t, httpServer.URL+"/projects/err-board/cards/1/description", http.MethodPatch, map[string]string{"body": ""})
	require.Equal(t, http.StatusBadRequest, emptyDescription.StatusCode)
	zeroDescriptionNumber := doJSON(t, httpServer.URL+"/projects/err-board/cards/0/description", http.MethodPatch, map[string]string{"body": "desc"})
	require.Equal(t, http.StatusBadRequest, zeroDescriptionNumber.StatusCode)

	missingDelete := doJSON(t, httpServer.URL+"/projects/err-board/cards/999", http.MethodDelete, nil)
	require.Equal(t, http.StatusNotFound, missingDelete.StatusCode)
	zeroDeleteNumber := doJSON(t, httpServer.URL+"/projects/err-board/cards/0", http.MethodDelete, nil)
	require.Equal(t, http.StatusBadRequest, zeroDeleteNumber.StatusCode)
}

func TestWebsocketProjectFilter(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)
	mustCreateProject(t, httpServer.URL, "Alpha")
	mustCreateProject(t, httpServer.URL, "Beta")

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws?project=alpha"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	eventCh := make(chan map[string]any, 1)
	errCh := make(chan error, 1)
	go func() {
		var event map[string]any
		if err := conn.ReadJSON(&event); err != nil {
			errCh <- err
			return
		}
		eventCh <- event
	}()

	betaCard := doJSON(t, httpServer.URL+"/projects/beta/cards", http.MethodPost, map[string]string{
		"title":  "beta card",
		"status": "Todo",
	})
	require.Equal(t, http.StatusCreated, betaCard.StatusCode)

	select {
	case event := <-eventCh:
		require.Failf(t, "unexpected event for filtered project", "event: %#v", event)
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(250 * time.Millisecond):
	}

	alphaCard := doJSON(t, httpServer.URL+"/projects/alpha/cards", http.MethodPost, map[string]string{
		"title":  "alpha card",
		"status": "Todo",
	})
	require.Equal(t, http.StatusCreated, alphaCard.StatusCode)

	select {
	case event := <-eventCh:
		require.Equal(t, "card.created", event["type"])
		require.Equal(t, "alpha", event["project"])
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for alpha event")
	}
}

func mustCreateProject(t *testing.T, baseURL, name string) {
	t.Helper()
	resp := doJSON(t, baseURL+"/projects", http.MethodPost, map[string]string{"name": name})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}
