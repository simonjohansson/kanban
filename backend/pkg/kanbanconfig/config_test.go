package kanbanconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadOrInitCreatesDefaults(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	cfg, err := LoadOrInit(home)
	require.NoError(t, err)

	require.Equal(t, DefaultServerURL, cfg.ServerURL)
	require.NotEmpty(t, cfg.Backend.SQLitePath)
	require.NotEmpty(t, cfg.Backend.CardsPath)
	require.Equal(t, DefaultOutput, cfg.CLI.Output)
	require.Equal(t, filepath.Join(home, ".config", "kanban", "config.yaml"), ConfigPath(home))

	_, err = os.Stat(ConfigPath(home))
	require.NoError(t, err)
}

func TestLoadOrInitMergesMissingFields(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	path := ConfigPath(home)

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`
server_url: http://seed:8080
backend:
  sqlite_path: /seed/projection.db
cli:
  output: json
`), 0o644))

	cfg, err := LoadOrInit(home)
	require.NoError(t, err)

	require.Equal(t, "http://seed:8080", cfg.ServerURL)
	require.Equal(t, "/seed/projection.db", cfg.Backend.SQLitePath)
	require.NotEmpty(t, cfg.Backend.CardsPath)
	require.Equal(t, "json", cfg.CLI.Output)

	roundTrip, err := LoadFile(path)
	require.NoError(t, err)
	require.Equal(t, cfg, roundTrip)
}
