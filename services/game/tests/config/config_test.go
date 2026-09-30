package config_test

import . "github.com/kungrem23/quizgo/services/game/internal/config"

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("QUIZ_GRPC_SERVICE_TOKEN", "0123456789abcdef0123456789abcdef")
	t.Setenv("GAME_HTTP_PORT", "")
	t.Setenv("GAME_REDIS_DB", "")
	t.Setenv("GAME_TTL", "")
	t.Setenv("GAME_QUIZ_GRPC_CA_FILE", "")
	t.Setenv("GAME_QUIZ_GRPC_CERT_FILE", "")
	t.Setenv("GAME_QUIZ_GRPC_KEY_FILE", "")
	clearWebSocketEnv(t)
	clearOwnershipEnv(t)
	t.Setenv("GAME_INSTANCE_ID", "replica-test")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.HTTPPort != "8081" || config.Redis.Address != "localhost:6379" || config.Quiz.Address != "localhost:9090" {
		t.Fatalf("unexpected defaults: %#v", config)
	}
	if config.WebSocket.MaxMessageBytes != 16<<10 || config.WebSocket.PlayerCommandRate != 5 || config.WebSocket.PlayerCommandBurst != 10 ||
		config.WebSocket.HostCommandRate != 10 || config.WebSocket.HostCommandBurst != 20 || config.WebSocket.InvalidMessageLimit != 5 ||
		config.WebSocket.HandshakeTimeout != 10*time.Second || config.WebSocket.CommandTimeout != 5*time.Second ||
		config.WebSocket.WriteTimeout != 5*time.Second || config.WebSocket.PingInterval != 30*time.Second || config.WebSocket.PongTimeout != 10*time.Second {
		t.Fatalf("unexpected websocket defaults: %#v", config.WebSocket)
	}
	if config.Ownership.InstanceID != "replica-test" || config.Ownership.InternalURL != "http://127.0.0.1:8081" ||
		config.Ownership.LeaseTTL != 15*time.Second || config.Ownership.RenewInterval != 5*time.Second || config.Ownership.SafetyMargin != time.Second {
		t.Fatalf("unexpected ownership defaults: %#v", config.Ownership)
	}
}

func TestLoadRejectsPartialMTLS(t *testing.T) {
	t.Setenv("QUIZ_GRPC_SERVICE_TOKEN", "0123456789abcdef0123456789abcdef")
	clearOwnershipEnv(t)
	t.Setenv("GAME_QUIZ_GRPC_CA_FILE", "ca.pem")
	t.Setenv("GAME_QUIZ_GRPC_CERT_FILE", "")
	t.Setenv("GAME_QUIZ_GRPC_KEY_FILE", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected partial mTLS configuration to fail")
	}
}

func TestLoadRejectsInvalidWebSocketLimit(t *testing.T) {
	t.Setenv("QUIZ_GRPC_SERVICE_TOKEN", "0123456789abcdef0123456789abcdef")
	clearWebSocketEnv(t)
	clearOwnershipEnv(t)
	t.Setenv("GAME_WS_MAX_MESSAGE_BYTES", "100")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid websocket message limit to fail")
	}
}

func TestLoadRejectsUnsafeOwnershipTiming(t *testing.T) {
	t.Setenv("QUIZ_GRPC_SERVICE_TOKEN", "0123456789abcdef0123456789abcdef")
	clearWebSocketEnv(t)
	clearOwnershipEnv(t)
	t.Setenv("GAME_ROOM_LEASE_TTL", "5s")
	t.Setenv("GAME_ROOM_LEASE_RENEW_INTERVAL", "4s")
	t.Setenv("GAME_ROOM_LEASE_SAFETY_MARGIN", "1s")
	if _, err := Load(); err == nil {
		t.Fatal("expected unsafe ownership timing to fail")
	}
}

func clearWebSocketEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"GAME_WS_MAX_MESSAGE_BYTES", "GAME_WS_HANDSHAKE_TIMEOUT", "GAME_WS_COMMAND_TIMEOUT", "GAME_WS_WRITE_TIMEOUT",
		"GAME_WS_PING_INTERVAL", "GAME_WS_PONG_TIMEOUT", "GAME_WS_PLAYER_COMMAND_RATE", "GAME_WS_PLAYER_COMMAND_BURST",
		"GAME_WS_HOST_COMMAND_RATE", "GAME_WS_HOST_COMMAND_BURST", "GAME_WS_INVALID_MESSAGE_LIMIT",
	} {
		t.Setenv(name, "")
	}
}

func clearOwnershipEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"GAME_INSTANCE_ID", "GAME_INTERNAL_URL", "GAME_ROOM_LEASE_TTL",
		"GAME_ROOM_LEASE_RENEW_INTERVAL", "GAME_ROOM_LEASE_SAFETY_MARGIN",
	} {
		t.Setenv(name, "")
	}
}
