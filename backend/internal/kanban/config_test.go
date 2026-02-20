package kanban

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeConfigPrecedence(t *testing.T) {
	t.Parallel()

	defaults := Config{
		ServerURL:  "http://127.0.0.1:8080",
		Output:     OutputText,
		CardsPath:  "/tmp/default-cards",
		SQLitePath: "/tmp/default.db",
	}
	fileCfg := Config{
		ServerURL:  "http://from-file:8080",
		Output:     OutputText,
		CardsPath:  "/tmp/file-cards",
		SQLitePath: "/tmp/file.db",
	}
	envCfg := Config{
		ServerURL:  "http://from-env:8080",
		Output:     OutputJSON,
		CardsPath:  "/tmp/env-cards",
		SQLitePath: "/tmp/env.db",
	}
	flagCfg := Config{
		ServerURL:  "http://from-flag:8080",
		Output:     OutputText,
		CardsPath:  "/tmp/flag-cards",
		SQLitePath: "/tmp/flag.db",
	}

	got := MergeConfig(defaults, fileCfg, envCfg, flagCfg)
	require.Equal(t, "http://from-flag:8080", got.ServerURL)
	require.Equal(t, OutputText, got.Output)
	require.Equal(t, "/tmp/flag-cards", got.CardsPath)
	require.Equal(t, "/tmp/flag.db", got.SQLitePath)
}

func TestLoadOrInitConfigWritesMissingFields(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	cfgDir := filepath.Join(home, ".config", "kanban")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(`
server_url: http://seed
cli:
  output: json
`), 0o644))

	got, err := LoadOrInitConfig(home)
	require.NoError(t, err)
	require.Equal(t, "http://seed", got.ServerURL)
	require.Equal(t, OutputJSON, got.Output)
	require.Equal(t, filepath.Join(home, ".config", "kanban", "config.yaml"), ConfigPath(home))

	roundTrip, err := LoadConfigFile(filepath.Join(cfgDir, "config.yaml"))
	require.NoError(t, err)
	require.Equal(t, got, roundTrip)
}

func TestParseEnvConfig(t *testing.T) {
	t.Parallel()

	env := []string{
		"KANBAN_SERVER_URL=http://env:9999",
		"KANBAN_OUTPUT=json",
		"KANBAN_CARDS_PATH=/tmp/env-cards",
		"KANBAN_SQLITE_PATH=/tmp/env.db",
	}

	got := ParseEnvConfig(env)
	require.Equal(t, "http://env:9999", got.ServerURL)
	require.Equal(t, OutputJSON, got.Output)
	require.Equal(t, "/tmp/env-cards", got.CardsPath)
	require.Equal(t, "/tmp/env.db", got.SQLitePath)
}

func TestFormatErrorJSON(t *testing.T) {
	t.Parallel()

	raw := FormatError(OutputJSON, 400, "bad request")
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	require.Equal(t, float64(400), body["status"])
	require.Equal(t, "bad request", body["error"])
}

func TestSaveConfigFileWritesScopedFields(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	err := SaveConfigFile(path, Config{
		ServerURL:  "http://127.0.0.1:9999",
		Output:     OutputJSON,
		CardsPath:  "/tmp/cards",
		SQLitePath: "/tmp/projection.db",
	})
	require.NoError(t, err)

	loaded, err := LoadConfigFile(path)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:9999", loaded.ServerURL)
	require.Equal(t, OutputJSON, loaded.Output)
	require.Equal(t, "/tmp/cards", loaded.CardsPath)
	require.Equal(t, "/tmp/projection.db", loaded.SQLitePath)
}
