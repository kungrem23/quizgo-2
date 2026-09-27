package quiz_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/domain"

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"regexp"
	"testing"
)

func TestCreateImageRecordStoresOnlyIDAndOwner(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO images (id, author_id) VALUES ($1,$2)`)).WithArgs("image-1", 42).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repo.CreateImageRecord(context.Background(), "image-1", 42); err != nil {
		t.Fatal(err)
	}
}

func TestImageExists(t *testing.T) {
	for _, found := range []bool{true, false} {
		repo, mock, cleanup := newPostgresRepoMock(t)
		rows := sqlmock.NewRows([]string{"id"})
		if found {
			rows.AddRow("image-1")
		}
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM images WHERE id=$1`)).WithArgs("image-1").WillReturnRows(rows)
		err := repo.ImageExists(context.Background(), "image-1")
		if found && err != nil || !found && !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("found=%v: %v", found, err)
		}
		cleanup()
	}
}

func TestDeleteImageChecksOwnershipUsageAndKeepsRecordOnStorageFailure(t *testing.T) {
	storageFailure := errors.New("S3 unavailable")
	for _, scenario := range []string{"missing-or-other-owner", "in-use", "storage-failure", "success"} {
		t.Run(scenario, func(t *testing.T) {
			repo, mock, cleanup := newPostgresRepoMock(t)
			defer cleanup()
			mock.ExpectBegin()
			ownerRows := sqlmock.NewRows([]string{"id"})
			if scenario != "missing-or-other-owner" {
				ownerRows.AddRow("image-1")
			}
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM images WHERE id=$1 AND author_id=$2 FOR UPDATE`)).WithArgs("image-1", 42).WillReturnRows(ownerRows)
			if scenario != "missing-or-other-owner" {
				mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS(SELECT 1 FROM questions WHERE image_id=$1)`)).WithArgs("image-1").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(scenario == "in-use"))
			}
			if scenario == "success" {
				mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM images WHERE id=$1 AND author_id=$2`)).WithArgs("image-1", 42).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			called := false
			err := repo.DeleteUploadedImage(context.Background(), "image-1", 42, func(context.Context, string) error {
				called = true
				if scenario == "storage-failure" {
					return storageFailure
				}
				return nil
			})
			switch scenario {
			case "missing-or-other-owner":
				if called || !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("%v, called=%v", err, called)
				}
			case "in-use":
				if called || !errors.Is(err, ErrImageInUse) {
					t.Fatalf("%v, called=%v", err, called)
				}
			case "storage-failure":
				if !called || !errors.Is(err, storageFailure) {
					t.Fatal(err)
				}
			case "success":
				if !called || err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
