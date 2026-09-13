package http

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// WithFrontend serves a Vite production build alongside the API. Client routes
// fall back to index.html; missing assets and unknown API routes remain 404s.
func WithFrontend(api http.Handler, directory string) (http.Handler, error) {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	index := filepath.Join(directory, "index.html")
	if _, err := os.Stat(index); err != nil {
		return nil, fmt.Errorf("frontend build: %w", err)
	}
	files := http.FileServer(http.Dir(directory))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, prefix := range []string{"/api", "/auth", "/swagger"} {
			if r.URL.Path == prefix || strings.HasPrefix(r.URL.Path, prefix+"/") {
				api.ServeHTTP(w, r)
				return
			}
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", 405)
			return
		}
		clean := path.Clean("/" + r.URL.Path)
		// Never expose dotfiles from a supplied static directory.
		for _, part := range strings.Split(clean, "/") {
			if strings.HasPrefix(part, ".") {
				http.NotFound(w, r)
				return
			}
		}
		filename := filepath.Join(directory, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
		info, err := os.Stat(filename)
		if err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(clean, "/assets/") || path.Ext(clean) != "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	}), nil
}
