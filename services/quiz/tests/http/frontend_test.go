package http_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/http"

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrontendPreservesAPIRoutesAndSupportsDeepLinks(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>QuizGo</html>"), 0600)
	os.WriteFile(filepath.Join(dir, ".env"), []byte("private"), 0600)
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "API response", 404) })
	h, err := WithFrontend(api, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		path   string
		status int
		body   string
	}{
		{"/quizzes/42/edit", 200, "QuizGo"}, {"/api/missing", 404, "API response"}, {"/auth/missing", 404, "API response"}, {"/assets/missing.js", 404, "404"}, {"/.env", 404, "404"},
	} {
		req := httptest.NewRequest("GET", tt.path, nil)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		if res.Code != tt.status || !strings.Contains(res.Body.String(), tt.body) {
			t.Errorf("%s: %d %s", tt.path, res.Code, res.Body.String())
		}
	}
}
