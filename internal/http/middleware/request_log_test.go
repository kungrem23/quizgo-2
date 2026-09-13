package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogger(t *testing.T) {
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	router := http.NewServeMux()
	router.HandleFunc("POST /api/quizzes", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	})
	handler := RequestLogger(router, logger)
	request := httptest.NewRequest(http.MethodPost, "/api/quizzes?draft=true", nil)
	request.Header.Set(requestIDHeader, "request-123")
	request.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.2")
	request.Header.Set("User-Agent", "quizgo-test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Header().Get(requestIDHeader) != "request-123" {
		t.Fatalf("%s = %q, want request-123", requestIDHeader, response.Header().Get(requestIDHeader))
	}
	for _, want := range []string{
		`http_request method="POST"`,
		`path="/api/quizzes"`,
		`route="POST /api/quizzes"`,
		`status=201`,
		`bytes=7`,
		`client_ip="203.0.113.10"`,
		`request_id="request-123"`,
		`user_agent="quizgo-test"`,
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("log %q does not contain %q", output.String(), want)
		}
	}
	if strings.Contains(output.String(), "draft=true") {
		t.Errorf("log contains query string: %q", output.String())
	}
}

func TestRequestLoggerRecordsImplicitOK(t *testing.T) {
	var output bytes.Buffer
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	}), log.New(&output, "", 0))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/quizzes", nil))

	if !strings.Contains(output.String(), "status=200 bytes=2") {
		t.Fatalf("log = %q, want implicit status and response size", output.String())
	}
}

func TestRequestLoggerSkipsHealthcheck(t *testing.T) {
	var output bytes.Buffer
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), log.New(&output, "", 0))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if output.Len() != 0 {
		t.Fatalf("healthcheck log = %q, want no output", output.String())
	}
}
