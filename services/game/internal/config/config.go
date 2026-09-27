package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort string
	Quiz     QuizGRPC
	Redis    Redis
	GameTTL  time.Duration
}

type QuizGRPC struct {
	Address      string
	ServiceToken string
	CAFile       string
	CertFile     string
	KeyFile      string
	ServerName   string
}

type Redis struct {
	Address  string
	Password string
	DB       int
}

func Load() (Config, error) {
	db, err := strconv.Atoi(envOrDefault("GAME_REDIS_DB", "0"))
	if err != nil || db < 0 {
		return Config{}, fmt.Errorf("GAME_REDIS_DB must be a non-negative integer")
	}
	ttl, err := time.ParseDuration(envOrDefault("GAME_TTL", "24h"))
	if err != nil || ttl < time.Minute || ttl > 7*24*time.Hour {
		return Config{}, fmt.Errorf("GAME_TTL must be between 1m and 168h")
	}
	config := Config{
		HTTPPort: envOrDefault("GAME_HTTP_PORT", "8081"),
		Quiz: QuizGRPC{
			Address:      envOrDefault("GAME_QUIZ_GRPC_ADDRESS", "localhost:9090"),
			ServiceToken: os.Getenv("QUIZ_GRPC_SERVICE_TOKEN"),
			CAFile:       os.Getenv("GAME_QUIZ_GRPC_CA_FILE"), CertFile: os.Getenv("GAME_QUIZ_GRPC_CERT_FILE"),
			KeyFile: os.Getenv("GAME_QUIZ_GRPC_KEY_FILE"), ServerName: os.Getenv("GAME_QUIZ_GRPC_SERVER_NAME"),
		},
		Redis:   Redis{Address: envOrDefault("GAME_REDIS_ADDRESS", "localhost:6379"), Password: os.Getenv("GAME_REDIS_PASSWORD"), DB: db},
		GameTTL: ttl,
	}
	if err := validatePort("GAME_HTTP_PORT", config.HTTPPort); err != nil {
		return Config{}, err
	}
	if len(config.Quiz.ServiceToken) < 32 {
		return Config{}, fmt.Errorf("QUIZ_GRPC_SERVICE_TOKEN must contain at least 32 bytes")
	}
	tlsValues := []string{config.Quiz.CAFile, config.Quiz.CertFile, config.Quiz.KeyFile}
	tlsConfigured := 0
	for _, value := range tlsValues {
		if value != "" {
			tlsConfigured++
		}
	}
	if tlsConfigured != 0 && tlsConfigured != len(tlsValues) {
		return Config{}, fmt.Errorf("quiz gRPC mTLS requires CA, certificate and key")
	}
	return config, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func validatePort(name, value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s must be a number between 1 and 65535", name)
	}
	return nil
}
