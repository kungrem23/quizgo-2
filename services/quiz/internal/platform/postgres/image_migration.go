package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

type ImageUploader interface {
	Put(context.Context, string, string, []byte) error
}

// Retain all source bytes if any upload fails. Retrying overwrites the same S3
// keys. Do not run the previous API version concurrently during this migration.
func migrateLegacyImages(db *sql.DB, store ImageUploader) error {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `LOCK TABLE images IN ACCESS EXCLUSIVE MODE`); err != nil {
		return err
	}
	// Load only IDs up front; each image is read separately to bound memory.
	rows, err := tx.QueryContext(ctx, `SELECT id FROM images WHERE content IS NOT NULL ORDER BY id`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		var data []byte
		var contentType string
		if err = tx.QueryRowContext(ctx, `SELECT content, COALESCE(content_type, 'application/octet-stream') FROM images WHERE id=$1`, id).Scan(&data, &contentType); err != nil {
			return err
		}
		if err = store.Put(ctx, id, contentType, data); err != nil {
			return fmt.Errorf("migrate image %s to S3 (source bytes retained): %w", id, err)
		}
		if _, err = tx.ExecContext(ctx, `UPDATE images SET content=NULL, content_type=NULL, image_url=$2 WHERE id=$1`, id, "/api/images/"+id); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if len(ids) > 0 {
		log.Printf("migrated %d images from PostgreSQL to S3", len(ids))
	}
	return nil
}
