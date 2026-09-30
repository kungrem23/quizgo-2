package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPPort  string
	Quiz      QuizGRPC
	Redis     Redis
	GameTTL   time.Duration
	WebSocket WebSocket
	Ownership RoomOwnership
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

type WebSocket struct {
	MaxMessageBytes     int64
	HandshakeTimeout    time.Duration
	CommandTimeout      time.Duration
	WriteTimeout        time.Duration
	PingInterval        time.Duration
	PongTimeout         time.Duration
	PlayerCommandRate   int
	PlayerCommandBurst  int
	HostCommandRate     int
	HostCommandBurst    int
	InvalidMessageLimit int
}

type RoomOwnership struct {
	InstanceID    string
	InternalURL   string
	LeaseTTL      time.Duration
	RenewInterval time.Duration
	SafetyMargin  time.Duration
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
	httpPort := envOrDefault("GAME_HTTP_PORT", "8081")
	websocketConfig, err := loadWebSocket()
	if err != nil {
		return Config{}, err
	}
	ownershipConfig, err := loadRoomOwnership(httpPort)
	if err != nil {
		return Config{}, err
	}
	config := Config{
		HTTPPort: httpPort,
		Quiz: QuizGRPC{
			Address:      envOrDefault("GAME_QUIZ_GRPC_ADDRESS", "localhost:9090"),
			ServiceToken: os.Getenv("QUIZ_GRPC_SERVICE_TOKEN"),
			CAFile:       os.Getenv("GAME_QUIZ_GRPC_CA_FILE"), CertFile: os.Getenv("GAME_QUIZ_GRPC_CERT_FILE"),
			KeyFile: os.Getenv("GAME_QUIZ_GRPC_KEY_FILE"), ServerName: os.Getenv("GAME_QUIZ_GRPC_SERVER_NAME"),
		},
		Redis:     Redis{Address: envOrDefault("GAME_REDIS_ADDRESS", "localhost:6379"), Password: os.Getenv("GAME_REDIS_PASSWORD"), DB: db},
		GameTTL:   ttl,
		WebSocket: websocketConfig,
		Ownership: ownershipConfig,
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

func loadRoomOwnership(httpPort string) (RoomOwnership, error) {
	instanceID := strings.TrimSpace(os.Getenv("GAME_INSTANCE_ID"))
	if instanceID == "" {
		instanceID, _ = os.Hostname()
	}
	if instanceID == "" || len(instanceID) > 128 {
		return RoomOwnership{}, fmt.Errorf("GAME_INSTANCE_ID must contain between 1 and 128 bytes")
	}
	internalURL := envOrDefault("GAME_INTERNAL_URL", "http://127.0.0.1:"+httpPort)
	parsedURL, err := url.Parse(internalURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" ||
		(parsedURL.Path != "" && parsedURL.Path != "/") || parsedURL.RawQuery != "" || parsedURL.Fragment != "" || parsedURL.User != nil {
		return RoomOwnership{}, fmt.Errorf("GAME_INTERNAL_URL must be an HTTP(S) origin without a path")
	}
	leaseTTL, err := duration("GAME_ROOM_LEASE_TTL", "15s", 3*time.Second, time.Minute)
	if err != nil {
		return RoomOwnership{}, err
	}
	renewInterval, err := duration("GAME_ROOM_LEASE_RENEW_INTERVAL", "5s", time.Second, 30*time.Second)
	if err != nil {
		return RoomOwnership{}, err
	}
	safetyMargin, err := duration("GAME_ROOM_LEASE_SAFETY_MARGIN", "1s", 100*time.Millisecond, 10*time.Second)
	if err != nil {
		return RoomOwnership{}, err
	}
	if renewInterval >= leaseTTL-safetyMargin {
		return RoomOwnership{}, fmt.Errorf("GAME_ROOM_LEASE_RENEW_INTERVAL must be shorter than lease TTL minus safety margin")
	}
	return RoomOwnership{
		InstanceID: instanceID, InternalURL: strings.TrimRight(internalURL, "/"),
		LeaseTTL: leaseTTL, RenewInterval: renewInterval, SafetyMargin: safetyMargin,
	}, nil
}

func loadWebSocket() (WebSocket, error) {
	maxMessageBytes, err := positiveInt("GAME_WS_MAX_MESSAGE_BYTES", "16384", 1024, 1<<20)
	if err != nil {
		return WebSocket{}, err
	}
	handshakeTimeout, err := duration("GAME_WS_HANDSHAKE_TIMEOUT", "10s", time.Second, time.Minute)
	if err != nil {
		return WebSocket{}, err
	}
	commandTimeout, err := duration("GAME_WS_COMMAND_TIMEOUT", "5s", 100*time.Millisecond, time.Minute)
	if err != nil {
		return WebSocket{}, err
	}
	writeTimeout, err := duration("GAME_WS_WRITE_TIMEOUT", "5s", 100*time.Millisecond, time.Minute)
	if err != nil {
		return WebSocket{}, err
	}
	pingInterval, err := duration("GAME_WS_PING_INTERVAL", "30s", time.Second, 5*time.Minute)
	if err != nil {
		return WebSocket{}, err
	}
	pongTimeout, err := duration("GAME_WS_PONG_TIMEOUT", "10s", 100*time.Millisecond, time.Minute)
	if err != nil {
		return WebSocket{}, err
	}
	playerRate, err := positiveInt("GAME_WS_PLAYER_COMMAND_RATE", "5", 1, 1000)
	if err != nil {
		return WebSocket{}, err
	}
	playerBurst, err := positiveInt("GAME_WS_PLAYER_COMMAND_BURST", "10", 1, 10000)
	if err != nil {
		return WebSocket{}, err
	}
	hostRate, err := positiveInt("GAME_WS_HOST_COMMAND_RATE", "10", 1, 1000)
	if err != nil {
		return WebSocket{}, err
	}
	hostBurst, err := positiveInt("GAME_WS_HOST_COMMAND_BURST", "20", 1, 10000)
	if err != nil {
		return WebSocket{}, err
	}
	invalidLimit, err := positiveInt("GAME_WS_INVALID_MESSAGE_LIMIT", "5", 1, 100)
	if err != nil {
		return WebSocket{}, err
	}
	return WebSocket{
		MaxMessageBytes: int64(maxMessageBytes), HandshakeTimeout: handshakeTimeout,
		CommandTimeout: commandTimeout, WriteTimeout: writeTimeout,
		PingInterval: pingInterval, PongTimeout: pongTimeout,
		PlayerCommandRate: playerRate, PlayerCommandBurst: playerBurst,
		HostCommandRate: hostRate, HostCommandBurst: hostBurst,
		InvalidMessageLimit: invalidLimit,
	}, nil
}

func positiveInt(name, fallback string, minimum, maximum int) (int, error) {
	value, err := strconv.Atoi(envOrDefault(name, fallback))
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", name, minimum, maximum)
	}
	return value, nil
}

func duration(name, fallback string, minimum, maximum time.Duration) (time.Duration, error) {
	value, err := time.ParseDuration(envOrDefault(name, fallback))
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %s and %s", name, minimum, maximum)
	}
	return value, nil
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
