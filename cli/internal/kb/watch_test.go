package kb

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatWatchLineJSON(t *testing.T) {
	t.Parallel()

	event := map[string]any{
		"type":    "card.created",
		"project": "alpha",
	}
	line, err := FormatWatchLine(OutputJSON, event)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &parsed))
	require.Equal(t, "alpha", parsed["project"])
}

func TestFormatWatchLineText(t *testing.T) {
	t.Parallel()

	event := map[string]any{
		"type":    "card.created",
		"project": "alpha",
		"card_id": "alpha/card-1",
	}
	line, err := FormatWatchLine(OutputText, event)
	require.NoError(t, err)
	require.Contains(t, line, "card.created")
	require.Contains(t, line, "alpha")
	require.Contains(t, line, "alpha/card-1")
}

func TestBuildWebsocketURL(t *testing.T) {
	t.Parallel()

	u, err := BuildWebsocketURL("http://127.0.0.1:8080", "")
	require.NoError(t, err)
	require.Equal(t, "ws://127.0.0.1:8080/ws", u)

	u, err = BuildWebsocketURL("https://kanban.local/api", "alpha")
	require.NoError(t, err)
	require.Equal(t, "wss://kanban.local/ws?project=alpha", u)
}
