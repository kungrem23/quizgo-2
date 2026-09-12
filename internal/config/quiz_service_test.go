package config

import "testing"

func TestLoadQuizService_FromEnvironment(t *testing.T) {
	t.Setenv("QUIZ_HTTP_PORT", "9090")
	t.Setenv("POSTGRES_HOST", "postgres")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "quiz")
	t.Setenv("POSTGRES_PASSWORD", "secret")
	t.Setenv("POSTGRES_DB", "quiz_test")
	t.Setenv("POSTGRES_SSLMODE", "require")

	got, err := LoadQuizService()
	if err != nil {
		t.Fatalf("LoadQuizService error: %v", err)
	}
	if got.HTTPPort != "9090" || got.Postgres.Host != "postgres" ||
		got.Postgres.Port != "5433" || got.Postgres.User != "quiz" ||
		got.Postgres.Password != "secret" || got.Postgres.Database != "quiz_test" ||
		got.Postgres.SSLMode != "require" {
		t.Fatalf("unexpected config: %#v", got)
	}
}

func TestLoadQuizService_InvalidPort(t *testing.T) {
	t.Setenv("QUIZ_HTTP_PORT", "invalid")

	if _, err := LoadQuizService(); err == nil {
		t.Fatal("LoadQuizService expected an error")
	}
}
