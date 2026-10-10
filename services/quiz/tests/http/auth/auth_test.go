package auth_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/http/handlers/auth"

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

type refreshRecord struct {
	userID    int
	hash      []byte
	expiresAt time.Time
	revoked   bool
}

type refreshStoreStub struct {
	mu      sync.Mutex
	records map[string]refreshRecord
}

func (s *refreshStoreStub) CreateRefreshSession(_ context.Context, session quiz.RefreshSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[session.ID] = refreshRecord{
		userID: session.UserID, hash: append([]byte(nil), session.SecretHash...), expiresAt: session.ExpiresAt,
	}
	return nil
}

func (s *refreshStoreStub) RotateRefreshSession(
	_ context.Context, id string, presentedHash, nextHash []byte, now time.Time,
) (int, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[id]
	if !ok || record.revoked || !record.expiresAt.After(now) || !bytes.Equal(record.hash, presentedHash) {
		if ok {
			record.revoked = true
			s.records[id] = record
		}
		return 0, time.Time{}, authn.ErrInvalidRefreshToken
	}
	record.hash = append([]byte(nil), nextHash...)
	s.records[id] = record
	return record.userID, record.expiresAt, nil
}

func (s *refreshStoreStub) RevokeRefreshSession(
	_ context.Context, id string, _ []byte, _ time.Time,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[id]
	if !ok {
		return authn.ErrInvalidRefreshToken
	}
	record.revoked = true
	s.records[id] = record
	return nil
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
	refresh, err := authn.NewRefreshManager(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	store := &refreshStoreStub{records: make(map[string]refreshRecord)}
	service := quiz.NewServiceWithAuth(repo, nil, tokens, refresh, store)
	return NewAuthHandler(service, CookieConfig{Secure: true})
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
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "quizgo_refresh" || !cookies[0].HttpOnly ||
		!cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Path != "/auth" {
		t.Fatalf("unexpected refresh cookie: %#v", cookies)
	}
}

func TestRefreshRotatesTokenAndRejectsReplay(t *testing.T) {
	hash, err := security.HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	handler := newAuthHandler(t, authRepoStub{
		getUser: func(context.Context, string) (quiz.User, error) {
			return quiz.User{Id: 42, Username: "alice", PasswordHash: hash}, nil
		},
	})
	loginRequest := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"alice","password":"secret123"}`))
	loginResponse := httptest.NewRecorder()
	handler.Login(loginResponse, loginRequest)
	original := loginResponse.Result().Cookies()[0]

	refreshRequest := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	refreshRequest.AddCookie(original)
	refreshResponse := httptest.NewRecorder()
	handler.Refresh(refreshResponse, refreshRequest)
	if refreshResponse.Code != http.StatusOK || !strings.Contains(refreshResponse.Body.String(), `"token":"Bearer `) {
		t.Fatalf("refresh status=%d body=%q", refreshResponse.Code, refreshResponse.Body.String())
	}
	rotated := refreshResponse.Result().Cookies()[0]
	if rotated.Value == original.Value {
		t.Fatal("refresh token was not rotated")
	}

	replayRequest := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	replayRequest.AddCookie(original)
	replayResponse := httptest.NewRecorder()
	handler.Refresh(replayResponse, replayRequest)
	if replayResponse.Code != http.StatusUnauthorized {
		t.Fatalf("replay status=%d body=%q", replayResponse.Code, replayResponse.Body.String())
	}
}

func TestLogoutRevokesSessionAndRejectsCrossOrigin(t *testing.T) {
	hash, err := security.HashPassword("secret123")
	if err != nil {
		t.Fatal(err)
	}
	handler := newAuthHandler(t, authRepoStub{
		getUser: func(context.Context, string) (quiz.User, error) {
			return quiz.User{Id: 42, Username: "alice", PasswordHash: hash}, nil
		},
	})
	loginRequest := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"alice","password":"secret123"}`))
	loginResponse := httptest.NewRecorder()
	handler.Login(loginResponse, loginRequest)
	cookie := loginResponse.Result().Cookies()[0]

	crossOrigin := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	crossOrigin.Host = "quiz.example.com"
	crossOrigin.Header.Set("Origin", "https://evil.example")
	crossOrigin.AddCookie(cookie)
	crossResponse := httptest.NewRecorder()
	handler.Logout(crossResponse, crossOrigin)
	if crossResponse.Code != http.StatusForbidden {
		t.Fatalf("cross-origin logout status=%d", crossResponse.Code)
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutRequest.AddCookie(cookie)
	logoutResponse := httptest.NewRecorder()
	handler.Logout(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusNoContent || logoutResponse.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatalf("logout status=%d cookies=%#v", logoutResponse.Code, logoutResponse.Result().Cookies())
	}

	refreshRequest := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	refreshRequest.AddCookie(cookie)
	refreshResponse := httptest.NewRecorder()
	handler.Refresh(refreshResponse, refreshRequest)
	if refreshResponse.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout status=%d", refreshResponse.Code)
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
