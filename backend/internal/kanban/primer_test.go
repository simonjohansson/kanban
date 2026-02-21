package kanban

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
	require.Contains(t, raw, "LIST_TODOS:")
	require.Contains(t, raw, "ADD_TODO:")
	require.Contains(t, raw, "LIST_ACCEPTANCE_CRITERIA:")
	require.Contains(t, raw, "ADD_ACCEPTANCE_CRITERION:")
	require.Contains(t, raw, "LIST_CARDS => {\"cards\":[")
	require.Contains(t, raw, "kanban --output json")
}

func TestPrimerJSONIncludesContractSections(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	require.NoError(t, printPrimer(OutputJSON, &out))

	var payload map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(out.Bytes()), &payload))

	require.Equal(t, "kanban", payload["name"])

	commandTemplates, ok := payload["command_templates"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, commandTemplates, "get_card")
	require.Contains(t, commandTemplates, "delete_project")
	require.Contains(t, commandTemplates, "list_cards_include_deleted")
	require.Contains(t, commandTemplates, "list_todos")
	require.Contains(t, commandTemplates, "add_todo")
	require.Contains(t, commandTemplates, "list_acceptance_criteria")
	require.Contains(t, commandTemplates, "add_acceptance_criterion")

	responseShapes, ok := payload["response_shapes"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, responseShapes, "list_cards")
	require.Contains(t, responseShapes, "list_todos")
	require.Contains(t, responseShapes, "list_acceptance_criteria")

	idSemantics, ok := payload["id_semantics"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, idSemantics, "card_id")
	require.Contains(t, idSemantics, "card_number")
	require.Contains(t, idSemantics, "id_argument")

	errorShape, ok := payload["error_shape"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, errorShape, "backend_problem_json")
	require.Contains(t, errorShape, "cli_fallback_json")

	deleteSemantics, ok := payload["delete_semantics"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, deleteSemantics["soft_delete_default"])
	require.Equal(t, true, deleteSemantics["hard_delete_flag"])

	descSemantics, ok := payload["desc_semantics"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "append", descSemantics["mode"])
	require.Equal(t, true, descSemantics["not_a_get"])

	todoSemantics, ok := payload["todo_semantics"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, todoSemantics["use_description_for_todos"])

	acceptanceSemantics, ok := payload["acceptance_semantics"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, acceptanceSemantics["use_description_for_acceptance_criteria"])

	projectCommandSupport, ok := payload["project_command_support"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, projectCommandSupport["rename_supported"])
	require.Equal(t, false, projectCommandSupport["edit_supported"])

	watchEventShape, ok := payload["watch_event_shape"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, watchEventShape, "type")
	require.Contains(t, watchEventShape, "project")
	require.Contains(t, watchEventShape, "card_id")
	require.Contains(t, watchEventShape, "card_number")

	statusRules, ok := payload["status_rules"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, statusRules["can_create_in_any_allowed_status"])
	require.Equal(t, true, statusRules["status_required_for_create_command"])
}
