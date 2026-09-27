package postgres_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/platform/postgres"

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type migrationStore struct {
	objects map[string][]byte
	failID  string
}

func (s *migrationStore) Put(_ context.Context, id, contentType string, data []byte) error {
	if id == s.failID {
		return errors.New("storage unavailable")
	}
	if contentType != "image/png" {
		return errors.New("wrong content type")
	}
	s.objects[id] = bytes.Clone(data)
	return nil
}

func TestLegacyImagesMigrateWithoutLosingSourceOnFailure(t *testing.T) {
	dsn := os.Getenv("QUIZ_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("QUIZ_TEST_POSTGRES_DSN is unset")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "quizgo_image_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{"0001_init.sql", "0002_editor.sql"} {
		data, err := os.ReadFile("../../internal/platform/postgres/migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(string(data)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO users (id,username,password_hash) VALUES (1,'author','hash'); INSERT INTO quizzes (id,title,author_id) VALUES (1,'Legacy',1)`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"image-a", "image-b"} {
		if _, err = db.Exec(`INSERT INTO images (id,author_id,image_url,content_type,content) VALUES ($1,1,$2,'image/png',$3)`, id, "/api/images/"+id, []byte("bytes-"+id)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO questions (quiz_id,position,text_content,image_id) VALUES (1,1,'Legacy question','image-a')`); err != nil {
		t.Fatal(err)
	}
	if err = ApplyMigrations(db); err == nil {
		t.Fatal("migration without S3 silently dropped old images")
	}
	store := &migrationStore{objects: map[string][]byte{}, failID: "image-b"}
	if err = ApplyMigrations(db, store); err == nil {
		t.Fatal("failed upload did not stop migration")
	}
	var retained int
	if err = db.QueryRow(`SELECT count(*) FROM images WHERE content IS NOT NULL`).Scan(&retained); err != nil || retained != 2 {
		t.Fatalf("source data was lost: %d, %v", retained, err)
	}
	store.failID = ""
	if err = ApplyMigrations(db, store); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"image-a", "image-b"} {
		if string(store.objects[id]) != "bytes-"+id {
			t.Fatal("object bytes changed")
		}
	}
	var cols int
	if err = db.QueryRow(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='images' AND column_name NOT IN ('id','author_id')`).Scan(&cols); err != nil || cols != 0 {
		t.Fatalf("non-metadata columns remain: %d %v", cols, err)
	}
	var imageID string
	if err = db.QueryRow(`SELECT image_id FROM questions WHERE quiz_id=1`).Scan(&imageID); err != nil || imageID != "image-a" {
		t.Fatal("question reference changed")
	}
	if err = ApplyMigrations(db, store); err != nil {
		t.Fatalf("restart failed: %v", err)
	}
}
