package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/kungrem23/quizgo/services/quiz/internal/http/middleware"
)

type optionalVerifier struct {
	userID int
	err    error
}

func (v optionalVerifier) VerifyAccessToken(string) (int, error) { return v.userID, v.err }

func TestOptionalAuthAllowsAnonymousAndIdentifiesBearer(t *testing.T) {
	handler := OptionalAuth(optionalVerifier{userID: 42}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if r.Header.Get("Authorization") == "" {
			if ok {
				t.Fatalf("anonymous request has user %d", userID)
			}
		} else if !ok || userID != 42 {
			t.Fatalf("viewer = %d, %v", userID, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, authorization := range []string{"", "Bearer token"} {
		request := httptest.NewRequest(http.MethodGet, "/resource", nil)
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("authorization %q: status = %d", authorization, response.Code)
		}
	}
}

func TestOptionalAuthRejectsInvalidBearer(t *testing.T) {
	handler := OptionalAuth(optionalVerifier{err: errors.New("invalid")}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next handler was called")
	}))
	request := httptest.NewRequest(http.MethodGet, "/resource", nil)
	request.Header.Set("Authorization", "Bearer invalid")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}
