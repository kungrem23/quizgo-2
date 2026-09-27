package quiz

import (
	"context"
	"database/sql"
)

func (r *PostgresRepository) CreateImageRecord(ctx context.Context, id string, userID int) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO images (id, author_id) VALUES ($1,$2)`, id, userID)
	return err
}

func (r *PostgresRepository) ImageExists(ctx context.Context, id string) error {
	var found string
	return r.db.QueryRowContext(ctx, `SELECT id FROM images WHERE id=$1`, id).Scan(&found)
}

// Keep the row locked while removing the object, preventing a concurrent save
// from attaching it. A failed storage deletion leaves metadata available to retry.
func (r *PostgresRepository) DeleteUploadedImage(ctx context.Context, id string, userID int, remove func(context.Context, string) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var found string
	if err = tx.QueryRowContext(ctx, `SELECT id FROM images WHERE id=$1 AND author_id=$2 FOR UPDATE`, id, userID).Scan(&found); err != nil {
		return err
	}
	var inUse bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM questions WHERE image_id=$1)`, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrImageInUse
	}
	if err = remove(ctx, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM images WHERE id=$1 AND author_id=$2`, id, userID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}
