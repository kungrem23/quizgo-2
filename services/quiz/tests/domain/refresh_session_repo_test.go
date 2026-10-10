package quiz_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/kungrem23/quizgo/services/quiz/internal/authn"
)

func TestRefreshSessionRepositoryRotatesAtomically(t *testing.T) {
	repo, mock, closeDB := newPostgresRepoMock(t)
	defer closeDB()
	now := time.Date(2026, time.October, 11, 10, 0, 0, 0, time.UTC)
	expiresAt := now.Add(24 * time.Hour)
	presentedHash := []byte("presented")
	nextHash := []byte("next")

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT user_id, secret_hash, expires_at, revoked_at
		FROM refresh_sessions WHERE id = $1 FOR UPDATE`)).
		WithArgs("session-id").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "secret_hash", "expires_at", "revoked_at"}).
			AddRow(42, presentedHash, expiresAt, nil))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE refresh_sessions
		SET secret_hash = $2, last_used_at = $3 WHERE id = $1`)).
		WithArgs("session-id", nextHash, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	userID, gotExpiry, err := repo.RotateRefreshSession(
		context.Background(), "session-id", presentedHash, nextHash, now,
	)
	if err != nil || userID != 42 || !gotExpiry.Equal(expiresAt) {
		t.Fatalf("RotateRefreshSession = %d, %v, %v", userID, gotExpiry, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshSessionRepositoryRevokesReplayedToken(t *testing.T) {
	repo, mock, closeDB := newPostgresRepoMock(t)
	defer closeDB()
	now := time.Date(2026, time.October, 11, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT user_id, secret_hash, expires_at, revoked_at
		FROM refresh_sessions WHERE id = $1 FOR UPDATE`)).
		WithArgs("session-id").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "secret_hash", "expires_at", "revoked_at"}).
			AddRow(42, []byte("current"), now.Add(time.Hour), nil))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE refresh_sessions SET revoked_at = $2 WHERE id = $1`)).
		WithArgs("session-id", now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	_, _, err := repo.RotateRefreshSession(
		context.Background(), "session-id", []byte("old"), []byte("next"), now,
	)
	if !errors.Is(err, authn.ErrInvalidRefreshToken) {
		t.Fatalf("RotateRefreshSession error = %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
