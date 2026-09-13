package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"time"

	// "github.com/google/uuid"
	// "github.com/kungrem23/quizgo/internal/store/models"
	// "github.com/kungrem23/quizgo/internal/store/repos"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

func connectionString(config Config) string {
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.User, config.Password),
		Host:   net.JoinHostPort(config.Host, config.Port),
		Path:   "/" + config.Database,
	}
	query := url.Values{}
	query.Set("sslmode", config.SSLMode)
	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func ConnectPG(config Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", connectionString(config))
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return db, nil
}

func ApplyMigrations(db *sql.DB, images ...ImageUploader) error {
	files, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("migration files not found")
	}
	for _, file := range files {
		if file == "migrations/0003_s3_images.sql" && len(images) > 0 && images[0] != nil {
			if err := migrateLegacyImages(db, images[0]); err != nil {
				return err
			}
		}
		content, err := migrationFiles.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		_, err = db.Exec(string(content))
		if err != nil {
			return fmt.Errorf("apply migration %s: %w", file, err)
		}
	}
	return nil
}

func NewDBConnection(config Config, images ...ImageUploader) (*sql.DB, error) {
	db, err := ConnectPG(config)
	if err != nil {
		return nil, err
	}
	if err := ApplyMigrations(db, images...); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
