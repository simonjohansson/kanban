package kanban

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

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

func TestAddrFromServerURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "127.0.0.1:9010", addrFromServerURL("http://127.0.0.1:9010"))
	require.Equal(t, "example.com:443", addrFromServerURL("https://example.com"))
	require.Equal(t, defaultListenAddr, addrFromServerURL("not-a-url"))
}

func TestServeCommandUsesConfigDefaultsWhenFlagsUnset(t *testing.T) {
	var got runtimeDefaults
	restore := setRunServeForTest(func(addr, cardsPath, sqlitePath string) error {
		got = runtimeDefaults{
			Addr:       addr,
			CardsPath:  cardsPath,
			SQLitePath: sqlitePath,
		}
		return nil
	})
	defer restore()

	cfg := Config{
		ServerURL:  "http://127.0.0.1:19190",
		CardsPath:  "/tmp/cards-default",
		SQLitePath: "/tmp/projection-default.db",
	}
	cmd := newServeCommand(&cfg)
	cmd.SetArgs(nil)
	require.NoError(t, cmd.Execute())

	require.Equal(t, "127.0.0.1:19190", got.Addr)
	require.Equal(t, "/tmp/cards-default", got.CardsPath)
	require.Equal(t, "/tmp/projection-default.db", got.SQLitePath)
}

func TestServeCommandAcceptsDeprecatedDataDirAlias(t *testing.T) {
	var got runtimeDefaults
	restore := setRunServeForTest(func(addr, cardsPath, sqlitePath string) error {
		got = runtimeDefaults{
			Addr:       addr,
			CardsPath:  cardsPath,
			SQLitePath: sqlitePath,
		}
		return nil
	})
	defer restore()

	cfg := Config{
		ServerURL:  "http://127.0.0.1:19191",
		CardsPath:  "/tmp/cards-default",
		SQLitePath: "/tmp/projection-default.db",
	}
	cmd := newServeCommand(&cfg)
	cmd.SetArgs([]string{
		"--data-dir", "/tmp/cards-alias",
		"--sqlite-path", "/tmp/projection-alias.db",
		"--addr", "127.0.0.1:18081",
	})
	require.NoError(t, cmd.Execute())

	require.Equal(t, "127.0.0.1:18081", got.Addr)
	require.Equal(t, "/tmp/cards-alias", got.CardsPath)
	require.Equal(t, "/tmp/projection-alias.db", got.SQLitePath)
}

func TestServeCommandRequiresStoragePaths(t *testing.T) {
	cfg := Config{
		ServerURL: "http://127.0.0.1:19192",
	}
	cmd := newServeCommand(&cfg)
	cmd.SetArgs(nil)
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "--cards-path cannot be empty")
}

func setRunServeForTest(fn func(addr, cardsPath, sqlitePath string) error) func() {
	previous := runServeFunc
	runServeFunc = fn
	return func() {
		runServeFunc = previous
	}
}

func TestRunServeWithSignalsStopsCleanlyAndCreatesStoragePaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cardsPath := filepath.Join(root, "cards")
	sqlitePath := filepath.Join(root, "db", "projection.db")
	addr := freeAddr(t)

	sigCh := make(chan os.Signal, 1)
	go func() {
		time.Sleep(150 * time.Millisecond)
		sigCh <- syscall.SIGTERM
	}()

	err := runServeWithSignals(addr, cardsPath, sqlitePath, sigCh)
	require.NoError(t, err)
	require.DirExists(t, cardsPath)
	require.DirExists(t, filepath.Dir(sqlitePath))
}

func TestRunServeWithSignalsReturnsListenError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cardsPath := filepath.Join(root, "cards")
	sqlitePath := filepath.Join(root, "db", "projection.db")
	addr := freeAddr(t)

	listener, err := net.Listen("tcp", addr)
	require.NoError(t, err)
	defer listener.Close()

	err = runServeWithSignals(addr, cardsPath, sqlitePath, make(chan os.Signal))
	require.Error(t, err)
	require.Contains(t, err.Error(), "listen failed")
}

func freeAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}
