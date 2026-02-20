package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMarkdownStoreUsesRWMutex(t *testing.T) {
	s, err := NewMarkdownStore(t.TempDir())
	require.NoError(t, err)
	require.Equal(t, reflect.TypeOf(sync.RWMutex{}), reflect.TypeOf(s.mu))
}

func TestGetProjectBlocksWhileWriteLockHeld(t *testing.T) {
	s, err := NewMarkdownStore(t.TempDir())
	require.NoError(t, err)

	project, err := s.CreateProject("Alpha", "", "")
	require.NoError(t, err)

	done := make(chan struct{})
	s.mu.Lock()
	go func() {
		defer close(done)
		_, _ = s.GetProject(project.Slug)
	}()

	select {
	case <-done:
		t.Fatal("expected GetProject to block while write lock is held")
	case <-time.After(100 * time.Millisecond):
	}

	s.mu.Unlock()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("GetProject did not finish after releasing write lock")
	}
}

func TestWriteFileAtomicCleansTempOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.md")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o644))

	previousRename := renameFile
	renameFile = func(_, _ string) error { return errors.New("rename failed") }
	t.Cleanup(func() { renameFile = previousRename })

	err := writeFileAtomic(path, []byte("new"), 0o644)
	require.Error(t, err)

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, "old", string(data))

	leftovers, globErr := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	require.NoError(t, globErr)
	require.Empty(t, leftovers)
}

func TestWriteFileAtomicReplacesTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.md")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o644))

	require.NoError(t, writeFileAtomic(path, []byte("after"), 0o644))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "after", string(data))
}

func TestValidateBranchName(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateBranchName(""))
	require.NoError(t, validateBranchName("feature/card-branch"))
	require.NoError(t, validateBranchName("bugfix/123"))

	require.Error(t, validateBranchName("bad branch"))
	require.Error(t, validateBranchName("feature..x"))
	require.Error(t, validateBranchName("feature.lock"))
	require.Error(t, validateBranchName("@"))
	require.Error(t, validateBranchName("refs/heads/main"))
	require.Error(t, validateBranchName("HEAD"))
	require.Error(t, validateBranchName("foo/.bar"))
	require.Error(t, validateBranchName("foo/bar.lock/baz"))
}
