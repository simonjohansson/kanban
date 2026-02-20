package kanban

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Output string

const (
	OutputText Output = "text"
	OutputJSON Output = "json"
)

type cliError struct {
	status  int
	message string
	rawJSON []byte
}

func (e *cliError) Error() string {
	return e.message
}

func isValidOutput(v string) bool {
	return v == string(OutputText) || v == string(OutputJSON)
}

func FormatError(output Output, status int, message string) string {
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = http.StatusText(status)
	}

	if output == OutputJSON {
		payload := map[string]any{
			"status": status,
			"error":  msg,
		}
		raw, _ := json.Marshal(payload)
		return string(raw)
	}

	return fmt.Sprintf("error (%d): %s", status, msg)
}

func handleResponse(output Output, stdout io.Writer, resp *http.Response, reqErr error) error {
	if reqErr != nil {
		return &cliError{status: http.StatusBadGateway, message: reqErr.Error()}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return &cliError{status: http.StatusInternalServerError, message: err.Error()}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(extractErrorMessage(raw))
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		if output == OutputJSON && json.Valid(raw) {
			return &cliError{status: resp.StatusCode, message: msg, rawJSON: compactJSON(raw)}
		}
		return &cliError{status: resp.StatusCode, message: msg}
	}

	if output == OutputJSON {
		trimmed := strings.TrimSpace(string(raw))
		if trimmed == "" {
			_, _ = fmt.Fprintln(stdout, "{}")
			return nil
		}
		if json.Valid(raw) {
			_, _ = fmt.Fprintln(stdout, string(compactJSON(raw)))
			return nil
		}

		encoded, _ := json.Marshal(map[string]any{"result": trimmed})
		_, _ = fmt.Fprintln(stdout, string(encoded))
		return nil
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		_, _ = fmt.Fprintln(stdout, "ok")
		return nil
	}

	_, _ = fmt.Fprintln(stdout, trimmed)
	return nil
}

func extractErrorMessage(raw []byte) string {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}

	if value, ok := obj["detail"].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	if value, ok := obj["title"].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	if value, ok := obj["error"].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	return ""
}

func compactJSON(raw []byte) []byte {
	var out bytes.Buffer
	if err := json.Compact(&out, raw); err != nil {
		return raw
	}
	return out.Bytes()
}

func asCLIError(err error, target **cliError) bool {
	e, ok := err.(*cliError)
	if !ok {
		return false
	}
	*target = e
	return true
}

func FormatWatchLine(output Output, event map[string]any) (string, error) {
	if output == OutputJSON {
		raw, err := json.Marshal(event)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}

	parts := make([]string, 0, 4)
	if value, ok := event["type"]; ok {
		parts = append(parts, fmt.Sprintf("type=%v", value))
	}
	if value, ok := event["project"]; ok {
		parts = append(parts, fmt.Sprintf("project=%v", value))
	}
	if value, ok := event["card_id"]; ok && fmt.Sprintf("%v", value) != "" {
		parts = append(parts, fmt.Sprintf("card_id=%v", value))
	}
	if value, ok := event["card_number"]; ok {
		parts = append(parts, fmt.Sprintf("card_number=%v", value))
	}
	if len(parts) == 0 {
		return "(event)", nil
	}

	return strings.Join(parts, " "), nil
}

func printPrimer(output Output, stdout io.Writer) error {
	executionRules := []string{
		"Prefer `--output json` for any command whose output will be parsed.",
		"Card operations are project-scoped and must include `--project` (`-p`).",
		"Single-card operations must include `--id` (`-i`).",
		"Use project slug (for example `alpha`) in command arguments.",
		"`watch` is long-running and must be explicitly stopped by the caller.",
	}

	commandTemplates := map[string]string{
		"list_projects":              "kanban --output json project ls",
		"create_project":             "kanban --output json project create --name \"$NAME\"",
		"delete_project":             "kanban --output json project rm \"$PROJECT\"",
		"list_cards":                 "kanban --output json card ls -p \"$PROJECT\"",
		"list_cards_include_deleted": "kanban --output json card ls -p \"$PROJECT\" --include-deleted",
		"create_card":                "kanban --output json card create -p \"$PROJECT\" -t \"$TITLE\" -s \"$STATUS\"",
		"get_card":                   "kanban --output json card get -p \"$PROJECT\" -i \"$ID\"",
		"move_card":                  "kanban --output json card move -p \"$PROJECT\" -i \"$ID\" -s \"$STATUS\"",
		"comment_card":               "kanban --output json card comment -p \"$PROJECT\" -i \"$ID\" -b \"$BODY\"",
		"describe_card":              "kanban --output json card desc -p \"$PROJECT\" -i \"$ID\" -b \"$BODY\"",
		"delete_card":                "kanban --output json card rm -p \"$PROJECT\" -i \"$ID\" [--hard]",
		"watch_events":               "kanban --output json watch -p \"$PROJECT\"",
	}

	responseShapes := map[string]any{
		"create_project": map[string]any{
			"name":          "Alpha",
			"slug":          "alpha",
			"next_card_seq": 1,
		},
		"create_card": map[string]any{
			"id":      "alpha/card-1",
			"project": "alpha",
			"number":  1,
			"status":  "Todo",
			"title":   "Task",
		},
		"get_card": map[string]any{
			"id":      "alpha/card-1",
			"project": "alpha",
			"number":  1,
			"title":   "Task",
			"status":  "Doing",
			"description": []any{
				map[string]any{"timestamp": "2026-02-20T12:00:00Z", "body": "Initial context"},
			},
			"comments": []any{
				map[string]any{"timestamp": "2026-02-20T12:05:00Z", "body": "Looks good"},
			},
			"history": []any{
				map[string]any{"timestamp": "2026-02-20T12:00:00Z", "type": "card.created", "details": "status=Todo column=Todo"},
				map[string]any{"timestamp": "2026-02-20T12:10:00Z", "type": "card.moved", "details": "status=Doing column=Doing"},
			},
		},
		"list_cards": map[string]any{
			"cards": []any{
				map[string]any{
					"id":             "alpha/card-1",
					"project":        "alpha",
					"number":         1,
					"title":          "Task",
					"status":         "Todo",
					"column":         "Todo",
					"deleted":        false,
					"comments_count": 1,
					"history_count":  2,
				},
			},
		},
	}

	errorShape := map[string]any{
		"backend_problem_json": map[string]any{
			"type":   "about:blank",
			"title":  "Unprocessable Entity",
			"status": 422,
			"detail": "validation error detail",
		},
		"cli_fallback_json": map[string]any{
			"status": 502,
			"error":  "gateway or CLI processing error",
		},
	}

	deleteSemantics := map[string]any{
		"soft_delete_default": true,
		"hard_delete_flag":    true,
		"soft_delete_effect":  "card remains queryable when --include-deleted is enabled",
		"hard_delete_effect":  "card is permanently removed",
	}

	descSemantics := map[string]any{
		"mode":      "append",
		"read_via":  "kanban --output json card get -p \"$PROJECT\" -i \"$ID\"",
		"not_a_get": true,
	}

	projectCommandSupport := map[string]any{
		"supported":        []string{"project create", "project ls", "project rm"},
		"rename_supported": false,
		"edit_supported":   false,
	}

	watchEventShape := map[string]any{
		"type":        "card.created",
		"project":     "alpha",
		"card_id":     "alpha/card-1",
		"card_number": 1,
		"timestamp":   "2026-02-20T12:34:56Z",
	}

	statusRules := map[string]any{
		"allowed":                            []string{"Todo", "Doing", "Review", "Done"},
		"can_create_in_any_allowed_status":   true,
		"status_required_for_create_command": true,
	}

	idSemantics := map[string]any{
		"card_id":     "card identifier string (<project-slug>/card-<number>)",
		"card_number": "project-scoped integer sequence (1,2,3...)",
		"id_argument": "all --id/-i flags expect card_number, not card_id",
	}

	if output == OutputJSON {
		payload := map[string]any{
			"name":           "kanban",
			"mode":           "machine",
			"purpose":        "HTTP-only kanban automation client.",
			"default_output": "json",
			"card_statuses":  []string{"Todo", "Doing", "Review", "Done"},
			"usage": map[string]any{
				"global_flags": []string{"--server-url", "--output"},
				"commands": []string{
					"project create|list|delete",
					"card create|get|list|move|comment|describe|delete",
					"watch [--project <slug>]",
					"primer",
				},
			},
			"execution_rules":         executionRules,
			"command_templates":       commandTemplates,
			"response_shapes":         responseShapes,
			"id_semantics":            idSemantics,
			"error_shape":             errorShape,
			"delete_semantics":        deleteSemantics,
			"desc_semantics":          descSemantics,
			"project_command_support": projectCommandSupport,
			"watch_event_shape":       watchEventShape,
			"status_rules":            statusRules,
			"agent_prompt": strings.Join([]string{
				"You are an automation agent controlling Kanban through the `kanban` CLI.",
				"Prefer deterministic, scriptable invocations and parse JSON output.",
				"Use `kanban --output json project ls` to discover project slugs before card operations.",
				"Use only valid card statuses: Todo, Doing, Review, Done.",
			}, "\n"),
		}
		raw, _ := json.Marshal(payload)
		_, _ = fmt.Fprintln(stdout, string(raw))
		return nil
	}

	text := strings.Join([]string{
		"KANBAN AGENT PRIMER (MACHINE MODE)",
		"",
		"SYSTEM PROMPT",
		"You are an automation agent controlling the `kanban` CLI.",
		"Produce deterministic commands and prefer machine-readable output.",
		"",
		"EXECUTION RULES",
		"1. Always prefer `--output json` when output is parsed by tools.",
		"2. Card commands require `--project` (`-p`).",
		"3. Single-card commands require `--id` (`-i`).",
		"4. Use project slug, not display name.",
		"5. Card statuses: Todo | Doing | Review | Done",
		"6. `watch` is long-running and must be interrupted by caller.",
		"",
		"COMMAND TEMPLATES",
		"LIST_PROJECTS: kanban --output json project ls",
		"CREATE_PROJECT: kanban --output json project create --name \"$NAME\"",
		"DELETE_PROJECT: kanban --output json project rm \"$PROJECT\"",
		"LIST_CARDS: kanban --output json card ls -p \"$PROJECT\"",
		"LIST_CARDS_WITH_DELETED: kanban --output json card ls -p \"$PROJECT\" --include-deleted",
		"CREATE_CARD: kanban --output json card create -p \"$PROJECT\" -t \"$TITLE\" -s \"$STATUS\"",
		"GET_CARD: kanban --output json card get -p \"$PROJECT\" -i \"$ID\"",
		"MOVE_CARD: kanban --output json card move -p \"$PROJECT\" -i \"$ID\" -s \"$STATUS\"",
		"COMMENT_CARD: kanban --output json card comment -p \"$PROJECT\" -i \"$ID\" -b \"$BODY\"",
		"DESCRIBE_CARD: kanban --output json card desc -p \"$PROJECT\" -i \"$ID\" -b \"$BODY\"",
		"DELETE_CARD: kanban --output json card rm -p \"$PROJECT\" -i \"$ID\" [--hard]",
		"WATCH_EVENTS: kanban --output json watch -p \"$PROJECT\"",
		"",
		"RESPONSE SHAPES",
		"CREATE_PROJECT => {\"name\":\"Alpha\",\"slug\":\"alpha\",\"next_card_seq\":1}",
		"CREATE_CARD => {\"id\":\"alpha/card-1\",\"project\":\"alpha\",\"number\":1,\"status\":\"Todo\",\"title\":\"Task\"}",
		"LIST_CARDS => {\"cards\":[{\"id\":\"alpha/card-1\",\"project\":\"alpha\",\"number\":1,\"status\":\"Todo\",\"deleted\":false,\"comments_count\":1,\"history_count\":2}]}",
		"GET_CARD => {\"id\":\"alpha/card-1\",\"project\":\"alpha\",\"number\":1,\"status\":\"Doing\",\"description\":[{\"timestamp\":\"...\",\"body\":\"...\"}],\"comments\":[{\"timestamp\":\"...\",\"body\":\"...\"}],\"history\":[{\"timestamp\":\"...\",\"type\":\"card.moved\",\"details\":\"...\"}]}",
		"",
		"CARD ID SEMANTICS",
		"- card_id = <project-slug>/card-<number> (string).",
		"- card_number = per-project integer sequence.",
		"- all --id/-i arguments use card_number, not card_id.",
		"",
		"ERROR SHAPE",
		"- backend problem JSON typically includes: type, title, status, detail.",
		"- CLI fallback JSON shape: {\"status\":<int>,\"error\":\"<message>\"}.",
		"",
		"DELETE SEMANTICS",
		"- default delete is soft delete (card can still be listed with --include-deleted).",
		"- --hard => permanent delete",
		"",
		"DESC SEMANTICS",
		"- `card desc` appends description text; it does not fetch current description.",
		"- read full card details via `kanban --output json card get -p \"$PROJECT\" -i \"$ID\"`.",
		"",
		"PROJECT COMMAND SUPPORT",
		"- supported: create, ls, rm",
		"- unsupported: rename/edit (not implemented)",
		"",
		"WATCH EVENT SHAPE",
		"- {\"type\":\"card.created\",\"project\":\"alpha\",\"card_id\":\"alpha/card-1\",\"card_number\":1,\"timestamp\":\"...\"}",
		"",
		"STATUS RULE",
		"- Cards may be created directly in any allowed status.",
	}, "\n")
	_, _ = fmt.Fprintln(stdout, text)
	return nil
}
