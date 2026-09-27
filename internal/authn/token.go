package authn

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const minimumSecretBytes = 32

var ErrInvalidToken = errors.New("invalid token")

type Config struct {
	Secret   string
	Issuer   string
	Audience string
	TTL      time.Duration
}

type AccessClaims struct {
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret   []byte
	issuer   string
	audience string
	ttl      time.Duration
	now      func() time.Time
}

func NewManager(config Config) (*Manager, error) {
	if len(config.Secret) < minimumSecretBytes {
		return nil, fmt.Errorf("JWT secret must contain at least %d bytes", minimumSecretBytes)
	}
	if strings.TrimSpace(config.Issuer) == "" {
		return nil, errors.New("JWT issuer is required")
	}
	if strings.TrimSpace(config.Audience) == "" {
		return nil, errors.New("JWT audience is required")
	}
	if config.TTL <= 0 {
		return nil, errors.New("JWT TTL must be positive")
	}
	return &Manager{
		secret:   []byte(config.Secret),
		issuer:   config.Issuer,
		audience: config.Audience,
		ttl:      config.TTL,
		now:      time.Now,
	}, nil
}

func (m *Manager) IssueAccessToken(userID int) (string, error) {
	if m == nil || userID < 1 {
		return "", ErrInvalidToken
	}
	now := m.now().UTC()
	claims := AccessClaims{
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.Itoa(userID),
			Audience:  jwt.ClaimStrings{m.audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *Manager) VerifyAccessToken(raw string) (int, error) {
	if m == nil {
		return 0, ErrInvalidToken
	}
	raw = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "Bearer "))
	if raw == "" {
		return 0, ErrInvalidToken
	}
	claims := &AccessClaims{}
	token, err := jwt.ParseWithClaims(
		raw,
		claims,
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(m.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || !token.Valid || claims.TokenType != "access" {
		return 0, ErrInvalidToken
	}
	userID, err := strconv.Atoi(claims.Subject)
	if err != nil || userID < 1 {
		return 0, ErrInvalidToken
	}
	return userID, nil
}
