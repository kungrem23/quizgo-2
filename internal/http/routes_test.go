package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kungrem23/quizgo/internal/domain/quiz"
)

func TestNewQuizRouter_RegistersRoutes(t *testing.T) {
	router := NewQuizRouter(quiz.NewService(nil))
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{name: "login", method: stdhttp.MethodPost, path: "/auth/login", status: stdhttp.StatusBadRequest},
		{name: "liveness", method: stdhttp.MethodGet, path: "/healthz", status: stdhttp.StatusOK},
		{name: "readiness", method: stdhttp.MethodGet, path: "/readyz", status: stdhttp.StatusOK},
		{name: "register", method: stdhttp.MethodPost, path: "/auth/register", status: stdhttp.StatusBadRequest},
		{name: "create quiz protected", method: stdhttp.MethodPost, path: "/api/quizzes", status: stdhttp.StatusUnauthorized},
		{name: "list quizzes route", method: stdhttp.MethodPut, path: "/api/quizzes", status: stdhttp.StatusMethodNotAllowed},
		{name: "get quiz", method: stdhttp.MethodGet, path: "/api/quizzes/0", status: stdhttp.StatusBadRequest},
		{name: "list author quizzes", method: stdhttp.MethodGet, path: "/api/users/0/quizzes", status: stdhttp.StatusBadRequest},
		{name: "create question protected", method: stdhttp.MethodPost, path: "/api/questions", status: stdhttp.StatusUnauthorized},
		{name: "get question", method: stdhttp.MethodGet, path: "/api/questions/0", status: stdhttp.StatusBadRequest},
		{name: "delete question protected", method: stdhttp.MethodDelete, path: "/api/questions/1", status: stdhttp.StatusUnauthorized},
		{name: "change position protected", method: stdhttp.MethodPatch, path: "/api/questions/1/position", body: `{"position":1}`, status: stdhttp.StatusUnauthorized},
		{name: "create answer protected", method: stdhttp.MethodPost, path: "/api/answers", status: stdhttp.StatusUnauthorized},
		{name: "get public answer", method: stdhttp.MethodGet, path: "/api/answers/0", status: stdhttp.StatusBadRequest},
		{name: "get author answer protected", method: stdhttp.MethodGet, path: "/api/answers/1/author", status: stdhttp.StatusUnauthorized},
		{name: "delete answer protected", method: stdhttp.MethodDelete, path: "/api/answers/1", status: stdhttp.StatusUnauthorized},
		{name: "get user", method: stdhttp.MethodGet, path: "/api/users/0", status: stdhttp.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			router.ServeHTTP(recorder, request)
			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d; body = %q", recorder.Code, tt.status, recorder.Body.String())
			}
		})
	}
}

func TestNewQuizRouter_ReadinessFailure(t *testing.T) {
	router := NewQuizRouterWithReadiness(quiz.NewService(nil), func(context.Context) error {
		return errors.New("database unavailable")
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(stdhttp.MethodGet, "/readyz", nil))
	if recorder.Code != stdhttp.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, stdhttp.StatusServiceUnavailable)
	}
}

func TestNewQuizRouter_UnknownRoute(t *testing.T) {
	router := NewQuizRouter(quiz.NewService(nil))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/unknown", nil)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, stdhttp.StatusNotFound)
	}
}
