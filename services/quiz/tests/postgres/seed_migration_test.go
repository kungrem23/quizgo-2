package postgres_test

import (
	"database/sql"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	. "github.com/kungrem23/quizgo/services/quiz/internal/platform/postgres"
	"github.com/kungrem23/quizgo/services/quiz/internal/security"
)

func TestSeedQuizMigration(t *testing.T) {
	dsn := os.Getenv("QUIZ_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("QUIZ_TEST_POSTGRES_DSN is unset")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "quizgo_seed_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

	if err = ApplyMigrations(db); err != nil {
		t.Fatal(err)
	}
	if err = ApplyMigrations(db); err != nil {
		t.Fatalf("repeat migration failed: %v", err)
	}

	var passwordHash string
	if err = db.QueryRow(`SELECT password_hash FROM users WHERE username='username'`).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	if !security.CheckPasswordHash("password", passwordHash) {
		t.Fatal("seeded password hash does not match password")
	}

	var quizzes, questions, answers, correctAnswers, invalidQuestions int
	if err = db.QueryRow(`SELECT count(*) FROM quizzes q JOIN users u ON u.id=q.author_id WHERE u.username='username' AND q.title='Удивительные факты'`).Scan(&quizzes); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM questions q JOIN quizzes quiz ON quiz.id=q.quiz_id JOIN users u ON u.id=quiz.author_id WHERE u.username='username' AND quiz.title='Удивительные факты'`).Scan(&questions); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT count(*), count(*) FILTER (WHERE a.is_correct) FROM answers a JOIN questions q ON q.id=a.question_id JOIN quizzes quiz ON quiz.id=q.quiz_id JOIN users u ON u.id=quiz.author_id WHERE u.username='username' AND quiz.title='Удивительные факты'`).Scan(&answers, &correctAnswers); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM (SELECT q.id FROM questions q JOIN quizzes quiz ON quiz.id=q.quiz_id JOIN users u ON u.id=quiz.author_id LEFT JOIN answers a ON a.question_id=q.id WHERE u.username='username' AND quiz.title='Удивительные факты' GROUP BY q.id HAVING count(a.id) <> 4 OR count(a.id) FILTER (WHERE a.is_correct) <> 1) invalid`).Scan(&invalidQuestions); err != nil {
		t.Fatal(err)
	}
	if quizzes != 1 || questions != 10 || answers != 40 || correctAnswers != 10 || invalidQuestions != 0 {
		t.Fatalf("unexpected seed counts: quizzes=%d questions=%d answers=%d correct=%d invalid_questions=%d", quizzes, questions, answers, correctAnswers, invalidQuestions)
	}

	var correct string
	if err = db.QueryRow(`SELECT string_agg(a.text_content, '|' ORDER BY q.position) FROM answers a JOIN questions q ON q.id=a.question_id JOIN quizzes quiz ON quiz.id=q.quiz_id JOIN users u ON u.id=quiz.author_id WHERE u.username='username' AND quiz.title='Удивительные факты' AND a.is_correct`).Scan(&correct); err != nil {
		t.Fatal(err)
	}
	if correct != "Осьминог|Мёд|Пурпурный|Пакман|Коала|Банан|В ухе|Консервная банка|Галлий|Слон" {
		t.Fatalf("unexpected correct answers: %q", correct)
	}

	expectedImages := map[int]string{
		1:  "d0b66245-e945-4339-b99e-585b2f83a87d",
		2:  "",
		3:  "61627a9d-e2fb-41c1-9784-d15739671565",
		4:  "5058cb76-2217-45ed-b6ae-627fa7822efd",
		5:  "b22da69c-b86d-4f8d-a779-db6c50a6496e",
		6:  "a3d24e86-d685-4ff1-93e0-80b6c20999b5",
		7:  "18c0ba81-094a-42cf-83de-a9b4e53d13ae",
		8:  "",
		9:  "",
		10: "de5e1abe-2e94-4817-848c-db44e8a2126d",
	}
	rows, err := db.Query(`SELECT q.position, COALESCE(q.image_id, '') FROM questions q JOIN quizzes quiz ON quiz.id=q.quiz_id JOIN users u ON u.id=quiz.author_id WHERE u.username='username' AND quiz.title='Удивительные факты' ORDER BY q.position`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var position int
		var imageID string
		if err = rows.Scan(&position, &imageID); err != nil {
			t.Fatal(err)
		}
		if imageID != expectedImages[position] {
			t.Errorf("question %d image ID = %q, want %q", position, imageID, expectedImages[position])
		}
		delete(expectedImages, position)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(expectedImages) != 0 {
		t.Fatalf("questions missing from image mapping: %v", expectedImages)
	}

	if _, err = db.Exec(`UPDATE questions SET image_id=NULL WHERE quiz_id=(SELECT q.id FROM quizzes q JOIN users u ON u.id=q.author_id WHERE u.username='username' AND q.title='Удивительные факты') AND position=1`); err != nil {
		t.Fatal(err)
	}
	if err = ApplyMigrations(db); err != nil {
		t.Fatalf("migration after an editor change failed: %v", err)
	}
	var retainedImageID string
	if err = db.QueryRow(`SELECT COALESCE(question.image_id, '') FROM questions question JOIN quizzes quiz ON quiz.id=question.quiz_id JOIN users u ON u.id=quiz.author_id WHERE u.username='username' AND quiz.title='Удивительные факты' AND question.position=1`).Scan(&retainedImageID); err != nil {
		t.Fatal(err)
	}
	if retainedImageID != "" {
		t.Fatalf("repeat migration overwrote an editor change with image %q", retainedImageID)
	}
}
