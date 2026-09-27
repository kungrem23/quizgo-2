package config_test

import . "github.com/kungrem23/quizgo/services/game/internal/config"

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("QUIZ_GRPC_SERVICE_TOKEN", "0123456789abcdef0123456789abcdef")
	t.Setenv("GAME_HTTP_PORT", "")
	t.Setenv("GAME_REDIS_DB", "")
	t.Setenv("GAME_TTL", "")
	t.Setenv("GAME_QUIZ_GRPC_CA_FILE", "")
	t.Setenv("GAME_QUIZ_GRPC_CERT_FILE", "")
	t.Setenv("GAME_QUIZ_GRPC_KEY_FILE", "")
	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.HTTPPort != "8081" || config.Redis.Address != "localhost:6379" || config.Quiz.Address != "localhost:9090" {
		t.Fatalf("unexpected defaults: %#v", config)
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
