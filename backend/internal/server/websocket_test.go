package server_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestWebsocketReceivesProjectAndCardEvents(t *testing.T) {
	t.Parallel()

	_, _, httpServer := newTestServer(t)

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	createProjectResp := doJSON(t, httpServer.URL+"/projects", http.MethodPost, map[string]string{"name": "Realtime"})
	require.Equal(t, http.StatusCreated, createProjectResp.StatusCode)

	createCardResp := doJSON(t, httpServer.URL+"/projects/realtime/cards", http.MethodPost, map[string]string{
		"title":       "Implement events",
		"description": "Broadcast all operations",
		"status":      "Todo",
		"column":      "Todo",
	})
	require.Equal(t, http.StatusCreated, createCardResp.StatusCode)

	commentResp := doJSON(t, httpServer.URL+"/projects/realtime/cards/1/comments", http.MethodPost, map[string]string{"body": "Looks good"})
	require.Equal(t, http.StatusOK, commentResp.StatusCode)

	descResp := doJSON(t, httpServer.URL+"/projects/realtime/cards/1/description", http.MethodPatch, map[string]string{"body": "Include reconnect semantics later"})
	require.Equal(t, http.StatusOK, descResp.StatusCode)

	moveResp := doJSON(t, httpServer.URL+"/projects/realtime/cards/1/move", http.MethodPatch, map[string]string{"status": "Doing", "column": "Doing"})
	require.Equal(t, http.StatusOK, moveResp.StatusCode)

	softDeleteResp := doJSON(t, httpServer.URL+"/projects/realtime/cards/1", http.MethodDelete, nil)
	require.Equal(t, http.StatusOK, softDeleteResp.StatusCode)

	hardDeleteResp := doJSON(t, httpServer.URL+"/projects/realtime/cards/1?hard=true", http.MethodDelete, nil)
	require.Equal(t, http.StatusOK, hardDeleteResp.StatusCode)

	expected := []string{
		"project.created",
		"card.created",
		"card.commented",
		"card.updated",
		"card.moved",
		"card.deleted_soft",
		"card.deleted_hard",
	}
	actual := make([]string, 0, len(expected))

	for i := 0; i < len(expected); i++ {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
		var event map[string]any
		err := conn.ReadJSON(&event)
		require.NoErrorf(t, err, "failed on event index %d", i)
		actual = append(actual, fmt.Sprintf("%v", event["type"]))
	}

	require.Equal(t, expected, actual)
}
