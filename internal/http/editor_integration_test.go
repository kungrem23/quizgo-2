package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kungrem23/quizgo/internal/authn"
	"github.com/kungrem23/quizgo/internal/domain/quiz"
	"github.com/kungrem23/quizgo/internal/platform/postgres"
)

// Set QUIZ_TEST_POSTGRES_DSN to a disposable PostgreSQL database. Each test uses
// an isolated schema and drops only that schema after completion.
func TestEditorPostgresLifecycle(t *testing.T) {
	dsn := os.Getenv("QUIZ_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("QUIZ_TEST_POSTGRES_DSN is not set")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "quizgo_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	params := u.Query()
	params.Set("search_path", schema)
	u.RawQuery = params.Encode()
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = postgres.ApplyMigrations(db); err != nil {
		t.Fatal(err)
	}
	// Migrations must also be safe when the service is restarted.
	if err = postgres.ApplyMigrations(db); err != nil {
		t.Fatal(err)
	}
	images := &testImageStore{objects: map[string][]byte{}}
	tokens, err := authn.NewManager(authn.Config{
		Secret: "0123456789abcdef0123456789abcdef", Issuer: "quizgo", Audience: "quizgo-test", TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	router := NewQuizRouter(quiz.NewServiceWithTokens(quiz.NewPostgresRepository(db), images, tokens))
	call := func(method, path, token string, body any, status int) []byte {
		t.Helper()
		var data []byte
		if body != nil {
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, res.Code, status, res.Body.String())
		}
		return res.Body.Bytes()
	}
	login := func(name string) string {
		t.Helper()
		body := map[string]string{"username": name, "password": "secret123"}
		call("POST", "/auth/register", "", body, 201)
		var v struct{ Token string }
		json.Unmarshal(call("POST", "/auth/login", "", body, 200), &v)
		return v.Token
	}
	owner, other := login("author"), login("other")
	call("GET", "/api/me", owner, nil, 200)
	call("GET", "/api/me", "", nil, 401)
	var created struct{ ID int }
	json.Unmarshal(call("POST", "/api/quizzes", owner, map[string]string{"title": "География"}, 201), &created)
	if created.ID < 1 {
		t.Fatal("create must return ID")
	}
	path := fmt.Sprintf("/api/quizzes/%d", created.ID)
	read := func() quiz.QuizContent {
		t.Helper()
		var v quiz.QuizContent
		if err := json.Unmarshal(call("GET", path+"/author", owner, nil, 200), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	empty := read()
	if empty.Revision < 1 || empty.Questions == nil || len(empty.Questions) != 0 {
		t.Fatalf("invalid empty content: %+v", empty)
	}
	question := func(text string) quiz.ContentQuestion {
		return quiz.ContentQuestion{TextContent: text, TimeLimit: 20, Answers: []quiz.ContentAnswer{{TextContent: "Париж", IsCorrect: true}, {TextContent: "Лондон"}}}
	}
	input := quiz.SaveQuizContent{Title: empty.Title, Revision: empty.Revision, Questions: []quiz.ContentQuestion{question("Столица Франции?"), question("Другой вопрос")}}
	call("PUT", path+"/content", other, input, 403)
	call("PUT", path+"/content", owner, input, 200)
	saved := read()
	if len(saved.Questions) != 2 || saved.Questions[0].ID == 0 || saved.Questions[0].Answers[0].ID == 0 {
		t.Fatal("missing persistent IDs")
	}
	// Public endpoints must never expose author flags.
	for _, suffix := range []string{"", "/content"} {
		if bytes.Contains(call("GET", path+suffix, "", nil, 200), []byte("is_correct")) {
			t.Fatal("public response leaked correct answers")
		}
	}
	call("GET", path+"/author", other, nil, 403)
	call("DELETE", path, other, nil, 403)
	call("PUT", path+"/content", owner, input, 409)
	originalFirstID := saved.Questions[0].ID
	originalAnswerID := saved.Questions[0].Answers[0].ID
	input.Revision = saved.Revision
	input.Title = "Обновлённый квиз"
	input.Questions = []quiz.ContentQuestion{saved.Questions[1], saved.Questions[0]}
	input.Questions[1].Answers = []quiz.ContentAnswer{saved.Questions[0].Answers[1], saved.Questions[0].Answers[0]}
	call("PUT", path+"/content", owner, input, 200)
	saved = read()
	if saved.Questions[1].ID != originalFirstID || saved.Questions[1].Answers[1].ID != originalAnswerID {
		t.Fatal("reordering replaced IDs")
	}
	if saved.Title != input.Title {
		t.Fatal("title was not saved")
	}
	// Invalid references after earlier writes must roll back the entire change.
	input.Revision = saved.Revision
	input.Title = "Must roll back"
	input.Questions = saved.Questions
	input.Questions[1].Answers[0].ID = 987654321
	call("PUT", path+"/content", owner, input, 400)
	rolledBack := read()
	if rolledBack.Title != saved.Title || rolledBack.Revision != saved.Revision {
		t.Fatal("partial write escaped transaction")
	}
	// Competing saves of the same revision cannot silently overwrite each other.
	concurrentInput := quiz.SaveQuizContent{Title: rolledBack.Title, Revision: rolledBack.Revision, Questions: rolledBack.Questions}
	concurrentBody, err := json.Marshal(concurrentInput)
	if err != nil {
		t.Fatal(err)
	}
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() {
			req := httptest.NewRequest("PUT", path+"/content", bytes.NewReader(concurrentBody))
			req.Header.Set("Authorization", owner)
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			statuses <- res.Code
		}()
	}
	first, second := <-statuses, <-statuses
	if !((first == 200 && second == 409) || (first == 409 && second == 200)) {
		t.Fatalf("competing save statuses: %d, %d", first, second)
	}
	rolledBack = read()
	// Correctness is validated at the service boundary.
	invalid := quiz.SaveQuizContent{Title: rolledBack.Title, Revision: rolledBack.Revision, Questions: []quiz.ContentQuestion{question("Некорректный вопрос")}}
	invalid.Questions[0].Answers[1].IsCorrect = true
	call("PUT", path+"/content", owner, invalid, 400)
	call("PUT", path+"/content", owner, map[string]any{"title": "Quiz", "revision": rolledBack.Revision, "questions": []any{}, "extra": true}, 400)
	// New images can be attached only by their owner; removing them while used is rejected.
	upload := func(token string, data []byte, status int) []byte {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "question.png")
		part.Write(data)
		writer.Close()
		req := httptest.NewRequest("POST", "/api/images", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", token)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("upload status %d: %s", res.Code, res.Body.String())
		}
		return res.Body.Bytes()
	}
	var pngData bytes.Buffer
	png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	var img quiz.UploadedImage
	json.Unmarshal(upload(owner, pngData.Bytes(), 201), &img)
	upload(owner, []byte("not an image"), 400)
	upload(owner, make([]byte, (5<<20)+1), 413)
	input = quiz.SaveQuizContent{Title: rolledBack.Title, Revision: rolledBack.Revision, Questions: rolledBack.Questions}
	input.Questions[0].ImageID = img.ID
	call("PUT", path+"/content", owner, input, 200)
	call("DELETE", "/api/images/"+img.ID, owner, nil, 409)
	req := httptest.NewRequest(http.MethodGet, "/api/images/"+img.ID, nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 307 || res.Header().Get("Location") != img.URL || res.Header().Get("Cache-Control") != "no-store" || !bytes.Equal(images.objects[img.ID], pngData.Bytes()) {
		t.Fatal("S3 object or redirect differs")
	}
	var link quiz.UploadedImage
	json.Unmarshal(call("GET", "/api/images/"+img.ID+"/url", "", nil, 200), &link)
	if link.URL != img.URL {
		t.Fatal("fresh image URL differs")
	}
	saved = read()
	if saved.Questions[0].ImageURL != img.URL {
		t.Fatal("author response lacks S3 link")
	}
	var public struct {
		Questions []struct {
			ImageURL string `json:"image_url"`
		} `json:"questions"`
	}
	json.Unmarshal(call("GET", path+"/content", "", nil, 200), &public)
	if public.Questions[0].ImageURL != img.URL {
		t.Fatal("public response lacks S3 link")
	}
	var binaryColumns int
	if err := db.QueryRow(`SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='images' AND column_name IN ('content', 'content_type', 'image_url')`).Scan(&binaryColumns); err != nil || binaryColumns != 0 {
		t.Fatalf("legacy columns remain: %d, %v", binaryColumns, err)
	}
	var foreign quiz.UploadedImage
	json.Unmarshal(upload(other, pngData.Bytes(), 201), &foreign)
	saved = read()
	input = quiz.SaveQuizContent{Title: saved.Title, Revision: saved.Revision, Questions: saved.Questions}
	input.Questions[0].ImageID = foreign.ID
	call("PUT", path+"/content", owner, input, 400)
	// Legacy reorder/delete routes must work with nullable images and cascade answers.
	saved = read()
	qid := saved.Questions[0].ID
	call("PATCH", fmt.Sprintf("/api/questions/%d/position", qid), owner, map[string]int{"position": 2}, 204)
	call("DELETE", fmt.Sprintf("/api/questions/%d", qid), owner, nil, 204)
	saved = read()
	if len(saved.Questions) != 1 {
		t.Fatal("legacy delete failed")
	}
	input = quiz.SaveQuizContent{Title: saved.Title, Revision: saved.Revision, Questions: []quiz.ContentQuestion{}}
	call("PUT", path+"/content", owner, input, 200)
	call("DELETE", "/api/images/"+img.ID, owner, nil, 204)
	if _, exists := images.objects[img.ID]; exists {
		t.Fatal("deleted object remains in storage")
	}
	call("GET", "/api/me/quizzes", owner, nil, 200)
	call("DELETE", path, owner, nil, 204)
	call("GET", path+"/content", "", nil, 404)
	var total int
	db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM answers`).Scan(&total)
	if total != 0 {
		t.Fatal("orphan answers")
	}
}

// The HTTP/DB lifecycle test keeps storage in memory; SDK wire behavior is
// covered separately by internal/store/s3 tests.
type testImageStore struct{ objects map[string][]byte }

func (s *testImageStore) Put(_ context.Context, id, _ string, data []byte) error {
	s.objects[id] = bytes.Clone(data)
	return nil
}
func (s *testImageStore) URL(_ context.Context, id string) (string, error) {
	return "https://storage.example.test/bucket/images/" + id + "?signature=test", nil
}
func (s *testImageStore) Delete(_ context.Context, id string) error {
	delete(s.objects, id)
	return nil
}
