package quiz

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"time"

	"github.com/kungrem23/quizgo/services/quiz/internal/authn"
)

type RefreshSession struct {
	ID         string
	UserID     int
	SecretHash []byte
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

type RefreshSessionStore interface {
	CreateRefreshSession(context.Context, RefreshSession) error
	RotateRefreshSession(context.Context, string, []byte, []byte, time.Time) (int, time.Time, error)
	RevokeRefreshSession(context.Context, string, []byte, time.Time) error
}

func (r *PostgresRepository) CreateRefreshSession(ctx context.Context, session RefreshSession) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO refresh_sessions
		(id, user_id, secret_hash, created_at, last_used_at, expires_at)
		VALUES ($1, $2, $3, $4, $4, $5)`,
		session.ID, session.UserID, session.SecretHash, session.CreatedAt, session.ExpiresAt)
	return err
}

func (r *PostgresRepository) RotateRefreshSession(
	ctx context.Context, id string, presentedHash, nextHash []byte, now time.Time,
) (int, time.Time, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer tx.Rollback()

	var userID int
	var storedHash []byte
	var expiresAt time.Time
	var revokedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT user_id, secret_hash, expires_at, revoked_at
		FROM refresh_sessions WHERE id = $1 FOR UPDATE`, id).
		Scan(&userID, &storedHash, &expiresAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, time.Time{}, authn.ErrInvalidRefreshToken
	}
	if err != nil {
		return 0, time.Time{}, err
	}

	validHash := len(storedHash) == len(presentedHash) &&
		subtle.ConstantTimeCompare(storedHash, presentedHash) == 1
	if revokedAt.Valid || !expiresAt.After(now) || !validHash {
		if !revokedAt.Valid {
			if _, updateErr := tx.ExecContext(ctx,
				`UPDATE refresh_sessions SET revoked_at = $2 WHERE id = $1`, id, now); updateErr != nil {
				return 0, time.Time{}, updateErr
			}
		}
		if err = tx.Commit(); err != nil {
			return 0, time.Time{}, err
		}
		return 0, time.Time{}, authn.ErrInvalidRefreshToken
	}

	if _, err = tx.ExecContext(ctx, `UPDATE refresh_sessions
		SET secret_hash = $2, last_used_at = $3 WHERE id = $1`, id, nextHash, now); err != nil {
		return 0, time.Time{}, err
	}
	if err = tx.Commit(); err != nil {
		return 0, time.Time{}, err
	}
	return userID, expiresAt, nil
}

func (r *PostgresRepository) RevokeRefreshSession(
	ctx context.Context, id string, presentedHash []byte, now time.Time,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var storedHash []byte
	err = tx.QueryRowContext(ctx,
		`SELECT secret_hash FROM refresh_sessions WHERE id = $1 FOR UPDATE`, id).Scan(&storedHash)
	if errors.Is(err, sql.ErrNoRows) {
		return authn.ErrInvalidRefreshToken
	}
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx,
		`UPDATE refresh_sessions SET revoked_at = COALESCE(revoked_at, $2) WHERE id = $1`, id, now); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if len(storedHash) != len(presentedHash) || subtle.ConstantTimeCompare(storedHash, presentedHash) != 1 {
		return authn.ErrInvalidRefreshToken
	}
	return nil
}
