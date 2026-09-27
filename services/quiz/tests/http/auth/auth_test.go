package auth_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/http/handlers/auth"

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kungrem23/quizgo/services/quiz/internal/authn"
	"github.com/kungrem23/quizgo/services/quiz/internal/domain"
	"github.com/kungrem23/quizgo/services/quiz/internal/security"
)

type authRepoStub struct {
	quiz.Repository
	getUser func(context.Context, string) (quiz.User, error)
	create  func(context.Context, string, string) error
}

func (r authRepoStub) GetUserByUsername(ctx context.Context, username string) (quiz.User, error) {
	return r.getUser(ctx, username)
}

func (r authRepoStub) CreateNewUser(ctx context.Context, username, passwordHash string) error {
	return r.create(ctx, username, passwordHash)
}

func newAuthHandler(t *testing.T, repo quiz.Repository) *AuthHandler {
	t.Helper()
	tokens, err := authn.NewManager(authn.Config{
		Secret: "0123456789abcdef0123456789abcdef", Issuer: "quizgo", Audience: "quizgo-test", TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewAuthHandler(quiz.NewServiceWithTokens(repo, nil, tokens))
}

func TestLoginReturnsGenericUnauthorized(t *testing.T) {
	handler := newAuthHandler(t, authRepoStub{
		getUser: func(context.Context, string) (quiz.User, error) { return quiz.User{}, sql.ErrNoRows },
	})
	request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"missing","password":"secret123"}`))
	response := httptest.NewRecorder()
	handler.Login(response, request)
	if response.Code != http.StatusUnauthorized || response.Body.String() != "{\"error\":\"unauthorized\"}\n" {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestLoginIssuesStrictAccessToken(t *testing.T) {
	hash, err := security.HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	handler := newAuthHandler(t, authRepoStub{
		getUser: func(context.Context, string) (quiz.User, error) {
			return quiz.User{Id: 42, Username: "alice", PasswordHash: hash}, nil
		},
	})
	request := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"alice","password":"secret123"}`))
	response := httptest.NewRecorder()
	handler.Login(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"token":"Bearer `) {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
}

func TestRegisterRejectsShortPasswordAndUnknownFields(t *testing.T) {
	handler := newAuthHandler(t, authRepoStub{
		getUser: func(context.Context, string) (quiz.User, error) { return quiz.User{}, sql.ErrNoRows },
		create:  func(context.Context, string, string) error { t.Fatal("user was created"); return nil },
	})
	for _, body := range []string{
		`{"username":"alice","password":"short"}`,
		`{"username":"alice","password":"secret123","admin":true}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
		response := httptest.NewRecorder()
		handler.Register(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body=%s status=%d response=%q", body, response.Code, response.Body.String())
		}
	}
}
