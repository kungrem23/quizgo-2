package authn

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

const refreshSecretBytes = 32

var ErrInvalidRefreshToken = errors.New("invalid refresh token")

type RefreshCredential struct {
	SessionID string
	Raw       string
	Hash      []byte
	ExpiresAt time.Time
}

type RefreshManager struct {
	ttl    time.Duration
	now    func() time.Time
	random io.Reader
}

func NewRefreshManager(ttl time.Duration) (*RefreshManager, error) {
	if ttl <= 0 {
		return nil, errors.New("refresh token TTL must be positive")
	}
	return &RefreshManager{ttl: ttl, now: time.Now, random: rand.Reader}, nil
}

func (m *RefreshManager) Issue() (RefreshCredential, error) {
	if m == nil {
		return RefreshCredential{}, ErrInvalidRefreshToken
	}
	now := m.now().UTC()
	return m.issueForSession(uuid.NewString(), now.Add(m.ttl))
}

func (m *RefreshManager) Rotate(sessionID string) (RefreshCredential, error) {
	if m == nil {
		return RefreshCredential{}, ErrInvalidRefreshToken
	}
	return m.issueForSession(sessionID, time.Time{})
}

func (m *RefreshManager) Parse(raw string) (string, []byte, error) {
	if m == nil {
		return "", nil, ErrInvalidRefreshToken
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return "", nil, ErrInvalidRefreshToken
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return "", nil, ErrInvalidRefreshToken
	}
	secret, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(secret) != refreshSecretBytes {
		return "", nil, ErrInvalidRefreshToken
	}
	hash := sha256.Sum256(secret)
	return id.String(), hash[:], nil
}

func (m *RefreshManager) Now() time.Time {
	if m == nil {
		return time.Time{}
	}
	return m.now().UTC()
}

func (m *RefreshManager) issueForSession(sessionID string, expiresAt time.Time) (RefreshCredential, error) {
	id, err := uuid.Parse(sessionID)
	if err != nil {
		return RefreshCredential{}, ErrInvalidRefreshToken
	}
	secret := make([]byte, refreshSecretBytes)
	if _, err = io.ReadFull(m.random, secret); err != nil {
		return RefreshCredential{}, err
	}
	hash := sha256.Sum256(secret)
	encoded := base64.RawURLEncoding.EncodeToString(secret)
	return RefreshCredential{
		SessionID: id.String(),
		Raw:       id.String() + "." + encoded,
		Hash:      hash[:],
		ExpiresAt: expiresAt,
	}, nil
}
