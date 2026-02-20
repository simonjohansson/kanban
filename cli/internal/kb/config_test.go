package kb

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
		ServerURL: "http://127.0.0.1:8080",
		Output:    OutputText,
	}
	fileCfg := Config{
		ServerURL: "http://from-file:8080",
		Output:    OutputText,
	}
	envCfg := Config{
		ServerURL: "http://from-env:8080",
		Output:    OutputJSON,
	}
	flagCfg := Config{
		ServerURL: "http://from-flag:8080",
		Output:    OutputText,
	}

	got := MergeConfig(defaults, fileCfg, envCfg, flagCfg)
	require.Equal(t, "http://from-flag:8080", got.ServerURL)
	require.Equal(t, OutputText, got.Output)
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
		"KB_SERVER_URL=http://env:9999",
		"KB_OUTPUT=json",
	}

	got := ParseEnvConfig(env)
	require.Equal(t, "http://env:9999", got.ServerURL)
	require.Equal(t, OutputJSON, got.Output)
}

func TestFormatErrorJSON(t *testing.T) {
	t.Parallel()

	raw := FormatError(OutputJSON, 400, "bad request")
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	require.Equal(t, float64(400), body["status"])
	require.Equal(t, "bad request", body["error"])
}
