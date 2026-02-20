package kb

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrimerTextIncludesCoreTemplates(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, printPrimer(OutputText, &out))
	raw := out.String()

	require.Contains(t, raw, "GET_CARD:")
	require.Contains(t, raw, "DELETE_PROJECT:")
	require.Contains(t, raw, "LIST_CARDS_WITH_DELETED:")
	require.Contains(t, raw, "LIST_CARDS => {\"cards\":[")
}

func TestPrimerJSONIncludesContractSections(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, printPrimer(OutputJSON, &out))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(out.Bytes()), &payload))

	commandTemplates, ok := payload["command_templates"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, commandTemplates, "get_card")
	require.Contains(t, commandTemplates, "delete_project")
	require.Contains(t, commandTemplates, "list_cards_include_deleted")

	responseShapes, ok := payload["response_shapes"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, responseShapes, "list_cards")
}
