package server

import (
	"bytes"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed webui/dist/*
var frontendAssets embed.FS

var apiPathPrefixes = []string{
	"/health",
	"/openapi",
	"/schemas/",
	"/projects",
	"/admin",
	"/ws",
}

func (s *Server) frontendHandler() http.HandlerFunc {
	sub, err := fs.Sub(frontendAssets, "webui/dist")
	if err != nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "frontend assets unavailable", http.StatusInternalServerError)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if isAPIPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		requestPath := normalizeRequestPath(r.URL.Path)
		if requestPath == "index.html" {
			serveEmbeddedFile(w, r, sub, "index.html")
			return
		}

		if fileExists(sub, requestPath) {
			serveEmbeddedFile(w, r, sub, requestPath)
			return
		}

		// SPA fallback for non-file routes.
		if strings.Contains(path.Base(requestPath), ".") {
			http.NotFound(w, r)
			return
		}
		serveEmbeddedFile(w, r, sub, "index.html")
	}
}

func normalizeRequestPath(raw string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(raw))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return "index.html"
	}
	return cleaned
}

func fileExists(root fs.FS, name string) bool {
	info, err := fs.Stat(root, name)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func isAPIPath(path string) bool {
	for _, prefix := range apiPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func serveEmbeddedFile(w http.ResponseWriter, r *http.Request, root fs.FS, name string) {
	data, err := fs.ReadFile(root, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if ctype := mime.TypeByExtension(path.Ext(name)); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
