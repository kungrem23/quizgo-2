package authn

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager(Config{Secret: testSecret, Issuer: "quizgo", Audience: "quizgo-crud", TTL: 15 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestManagerIssueAndVerify(t *testing.T) {
	m := newTestManager(t)
	token, err := m.IssueAccessToken(42)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := m.VerifyAccessToken(token)
	if err != nil || userID != 42 {
		t.Fatalf("VerifyAccessToken = %d, %v", userID, err)
	}
}

func TestManagerRejectsWrongAudienceAndAlgorithm(t *testing.T) {
	m := newTestManager(t)
	now := time.Now()
	claims := AccessClaims{TokenType: "access", RegisteredClaims: jwt.RegisteredClaims{
		Issuer: "quizgo", Subject: "42", Audience: jwt.ClaimStrings{"another-service"},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)), IssuedAt: jwt.NewNumericDate(now),
	}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.VerifyAccessToken(token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong audience error = %v", err)
	}

	wrongAlgorithm, err := jwt.NewWithClaims(jwt.SigningMethodHS384, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.VerifyAccessToken(wrongAlgorithm); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("wrong algorithm error = %v", err)
	}
}

func TestNewManagerRejectsWeakConfiguration(t *testing.T) {
	if _, err := NewManager(Config{Secret: "short", Issuer: "quizgo", Audience: "quizgo", TTL: time.Minute}); err == nil {
		t.Fatal("expected weak secret to be rejected")
	}
}
