package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/kungrem23/quizgo/services/game/internal/application"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
)

const (
	maxMessageTypeLength = 32
	maxRequestIDLength   = 128
)

type tokenBucket struct {
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

func newTokenBucket(rate, burst int, now time.Time) tokenBucket {
	return tokenBucket{rate: float64(rate), burst: float64(burst), tokens: float64(burst), last: now}
}

func (b *tokenBucket) allow(now time.Time) bool {
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens = min(b.burst, b.tokens+elapsed*b.rate)
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

type commandGuard struct {
	limiter        tokenBucket
	violations     int
	violationLimit int
}

func newCommandGuard(role application.Role, handler *Handler) *commandGuard {
	rate, burst := handler.playerCommandRate, handler.playerCommandBurst
	if role == application.RoleHost {
		rate, burst = handler.hostCommandRate, handler.hostCommandBurst
	}
	return &commandGuard{limiter: newTokenBucket(rate, burst, time.Now()), violationLimit: handler.invalidMessageLimit}
}

func (g *commandGuard) allow(now time.Time) bool {
	return g.limiter.allow(now)
}

func (g *commandGuard) violation() bool {
	g.violations++
	return g.violations >= g.violationLimit
}

func (g *commandGuard) success() {
	g.violations = 0
}

func readClientMessage(ctx context.Context, connection *websocket.Conn) (ClientMessage, error) {
	_, raw, err := connection.Read(ctx)
	if err != nil {
		return ClientMessage{}, err
	}
	var message ClientMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		return ClientMessage{}, errors.Join(errInvalidPayload, err)
	}
	if message.Type == "" || len(message.Type) > maxMessageTypeLength || len(message.RequestID) > maxRequestIDLength {
		return message, errInvalidPayload
	}
	return message, nil
}

func validateEmptyPayload(raw json.RawMessage) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var payload struct{}
	return decodePayload(raw, &payload)
}

func validCredentialField(value string, maxLength int) bool {
	return strings.TrimSpace(value) != "" && len(value) <= maxLength
}

func isKnownMessageType(messageType string) bool {
	switch messageType {
	case "join", "host_auth", "player_auth", "start", "next", "finish", "leave", "remove_player", "answer":
		return true
	default:
		return false
	}
}

func isCommandType(messageType string) bool {
	switch messageType {
	case "start", "next", "finish", "leave", "remove_player", "answer":
		return true
	default:
		return false
	}
}

func commandAllowed(role application.Role, messageType string) bool {
	switch role {
	case application.RoleHost:
		return messageType == "start" || messageType == "next" || messageType == "finish" || messageType == "remove_player"
	case application.RolePlayer:
		return messageType == "answer" || messageType == "leave"
	default:
		return false
	}
}

func isClientViolation(err error) bool {
	return errors.Is(err, errInvalidPayload) || errors.Is(err, errUnknownMessageType) || errors.Is(err, errRateLimited) ||
		errors.Is(err, game.ErrUnauthorized) || errors.Is(err, game.ErrInvalidPlayer) || errors.Is(err, game.ErrInvalidAnswer) ||
		errors.Is(err, game.ErrInvalidRequestID)
}
