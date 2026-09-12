package config

import (
	"fmt"
	"os"
	"strconv"
)

type QuizService struct {
	HTTPPort string
	Postgres Postgres
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
		Postgres: Postgres{
			Host:     envOrDefault("POSTGRES_HOST", "localhost"),
			Port:     envOrDefault("POSTGRES_PORT", "5432"),
			User:     envOrDefault("POSTGRES_USER", "danilmitrosin"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			Database: envOrDefault("POSTGRES_DB", "quizgo"),
			SSLMode:  envOrDefault("POSTGRES_SSLMODE", "disable"),
		},
	}

	if err := validatePort("QUIZ_HTTP_PORT", config.HTTPPort); err != nil {
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
