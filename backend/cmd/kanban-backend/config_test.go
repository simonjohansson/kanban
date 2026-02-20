package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaultsFromSharedConfig(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	configPath := filepath.Join(home, ".config", "kanban", "config.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`
server_url: http://127.0.0.1:9010
backend:
  sqlite_path: /tmp/from-config.db
  cards_path: /tmp/from-config-cards
cli:
  output: text
`), 0o644))

	defaults, err := loadRuntimeDefaults(home)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:9010", defaults.Addr)
	require.Equal(t, "/tmp/from-config.db", defaults.SQLitePath)
	require.Equal(t, "/tmp/from-config-cards", defaults.CardsPath)
}
