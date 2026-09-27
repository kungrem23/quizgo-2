package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	imagestore "github.com/kungrem23/quizgo/internal/store/s3"
)

type QuizService struct {
	HTTPPort string
	GRPC     GRPC
	JWT      JWT
	Postgres Postgres
	S3       imagestore.Config
}

type JWT struct {
	Secret   string
	Issuer   string
	Audience string
	TTL      time.Duration
}

type GRPC struct {
	Port             string
	ServiceToken     string
	TLSCertFile      string
	TLSKeyFile       string
	TLSClientCAFile  string
	AllowedClientURI string
}

type Postgres struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

func LoadQuizService() (QuizService, error) {
	config := QuizService{
		HTTPPort: envOrDefault("QUIZ_HTTP_PORT", "8080"),
		GRPC: GRPC{
			Port:             envOrDefault("QUIZ_GRPC_PORT", "9090"),
			ServiceToken:     os.Getenv("QUIZ_GRPC_SERVICE_TOKEN"),
			TLSCertFile:      os.Getenv("QUIZ_GRPC_TLS_CERT_FILE"),
			TLSKeyFile:       os.Getenv("QUIZ_GRPC_TLS_KEY_FILE"),
			TLSClientCAFile:  os.Getenv("QUIZ_GRPC_TLS_CLIENT_CA_FILE"),
			AllowedClientURI: os.Getenv("QUIZ_GRPC_ALLOWED_CLIENT_URI"),
		},
		JWT: JWT{
			Secret:   os.Getenv("JWT_SECRET"),
			Issuer:   envOrDefault("JWT_ISSUER", "quizgo-auth"),
			Audience: envOrDefault("JWT_AUDIENCE", "quizgo-crud"),
		},
		Postgres: Postgres{
			Host:     envOrDefault("POSTGRES_HOST", "localhost"),
			Port:     envOrDefault("POSTGRES_PORT", "5432"),
			User:     envOrDefault("POSTGRES_USER", "danilmitrosin"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			Database: envOrDefault("POSTGRES_DB", "quizgo"),
			SSLMode:  envOrDefault("POSTGRES_SSLMODE", "disable"),
		},
	}
	var err error
	config.JWT.TTL, err = time.ParseDuration(envOrDefault("JWT_TTL", "15m"))
	if err != nil || config.JWT.TTL < time.Minute || config.JWT.TTL > 24*time.Hour {
		return QuizService{}, fmt.Errorf("JWT_TTL must be between 1m and 24h")
	}
	if len(config.JWT.Secret) < 32 {
		return QuizService{}, fmt.Errorf("JWT_SECRET must contain at least 32 bytes")
	}
	if len(config.GRPC.ServiceToken) < 32 {
		return QuizService{}, fmt.Errorf("QUIZ_GRPC_SERVICE_TOKEN must contain at least 32 bytes")
	}
	tlsValues := []string{config.GRPC.TLSCertFile, config.GRPC.TLSKeyFile, config.GRPC.TLSClientCAFile, config.GRPC.AllowedClientURI}
	tlsConfigured := 0
	for _, value := range tlsValues {
		if value != "" {
			tlsConfigured++
		}
	}
	if tlsConfigured != 0 && tlsConfigured != len(tlsValues) {
		return QuizService{}, fmt.Errorf("gRPC mTLS requires certificate, key, client CA and allowed client URI")
	}

	config.S3 = imagestore.Config{
		Bucket: os.Getenv("S3_BUCKET"), Region: envOrDefault("S3_REGION", "us-east-1"),
		Endpoint: os.Getenv("S3_ENDPOINT"), BrowserEndpoint: os.Getenv("S3_BROWSER_ENDPOINT"),
	}
	config.S3.URLTTL, err = time.ParseDuration(envOrDefault("S3_URL_TTL", "1h"))
	if err != nil || config.S3.URLTTL < time.Minute || config.S3.URLTTL > 7*24*time.Hour {
		return QuizService{}, fmt.Errorf("S3_URL_TTL must be between 1m and 168h")
	}
	config.S3.ForcePathStyle, err = strconv.ParseBool(envOrDefault("S3_FORCE_PATH_STYLE", "false"))
	if err != nil {
		return QuizService{}, fmt.Errorf("S3_FORCE_PATH_STYLE must be true or false")
	}

	if err := validatePort("QUIZ_HTTP_PORT", config.HTTPPort); err != nil {
		return QuizService{}, err
	}
	if err := validatePort("QUIZ_GRPC_PORT", config.GRPC.Port); err != nil {
		return QuizService{}, err
	}
	if err := validatePort("POSTGRES_PORT", config.Postgres.Port); err != nil {
		return QuizService{}, err
	}
	return config, nil
}

func envOrDefault(name, fallback string) string {
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return fallback
	}
	return value
}

func validatePort(name, value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%s must be a number between 1 and 65535", name)
	}
	return nil
}
