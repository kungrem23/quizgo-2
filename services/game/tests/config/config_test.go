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
}

func TestLoadRejectsPartialMTLS(t *testing.T) {
	t.Setenv("QUIZ_GRPC_SERVICE_TOKEN", "0123456789abcdef0123456789abcdef")
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
	t.Setenv("GAME_WS_MAX_MESSAGE_BYTES", "100")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid websocket message limit to fail")
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
