package backend_test

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

type testLogWriter struct {
	t      *testing.T
	prefix string
	mu     sync.Mutex
	buf    bytes.Buffer
}

func newTestLogger(t *testing.T, prefix string) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(&testLogWriter{t: t, prefix: prefix}, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func (w *testLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, _ := w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.WriteString(line)
			break
		}
		w.t.Logf("[%s] %s", w.prefix, strings.TrimSpace(line))
	}
	return n, nil
}

func streamReaderToTestLogs(t *testing.T, prefix string, r io.Reader, wg *sync.WaitGroup) {
	t.Helper()
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			t.Logf("[%s] %s", prefix, scanner.Text())
		}
	}()
}
