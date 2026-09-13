package postgres

import (
	"net/url"
	"testing"
)

func TestConnectionStringPreservesEmptyAndSpecialPasswords(t *testing.T) {
	for _, password := range []string{"", "spaces and 'quotes' \\ backslash @:/?#="} {
		cfg := Config{Host: "::1", Port: "5432", User: "quiz user", Password: password, Database: "quizgo", SSLMode: "disable"}
		parsed, err := url.Parse(connectionString(cfg))
		if err != nil {
			t.Fatal(err)
		}
		gotPassword, ok := parsed.User.Password()
		if !ok || gotPassword != password || parsed.User.Username() != cfg.User || parsed.Path != "/quizgo" || parsed.Host != "[::1]:5432" || parsed.Query().Get("sslmode") != "disable" {
			t.Fatal("connection string changed configuration values")
		}
	}
}
