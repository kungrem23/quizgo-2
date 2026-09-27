package quiz_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/domain"

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func newPostgresRepoMock(t *testing.T) (*PostgresRepository, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	cleanup := func() {
		mock.ExpectClose()
		if err := db.Close(); err != nil {
			t.Errorf("close DB: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unmet sql expectations: %v", err)
		}
	}
	return NewPostgresRepository(db), mock, cleanup
}
