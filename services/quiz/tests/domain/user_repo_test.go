package quiz_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/domain"

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// ================ CreateNewUser ==================

func TestCreateNewUser_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		username     = "qwerty"
		passwordHash = "asdfgh"
	)
	query := regexp.QuoteMeta(`INSERT INTO users
	(username, password_hash)
	VALUES ($1, $2)
	RETURNING id, username, password_hash;`)
	mock.ExpectExec(query).
		WithArgs(username, passwordHash).
		WillReturnResult(sqlmock.NewResult(0, 1))
	err := repo.CreateNewUser(
		context.Background(),
		username,
		passwordHash)
	if err != nil {
		t.Fatalf("CreateNewUser error got: %v | expect: %v", err, nil)
	}
}

// ================= GetAllUsers ===================

func TestGetAllUsers_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		userId1       = 1
		username1     = "qwerty1"
		passwordHash1 = "asdfgh1"

		userId2       = 2
		username2     = "qwerty2"
		passwordHash2 = "asdfgh2"
	)
	query := regexp.QuoteMeta(`SELECT id, username, password_hash FROM users`)
	mock.ExpectQuery(query).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash"}).
			AddRow(userId1, username1, passwordHash1).
			AddRow(userId2, username2, passwordHash2))
	want := []User{
		{
			Id:           userId1,
			Username:     username1,
			PasswordHash: passwordHash1,
		},
		{
			Id:           userId2,
			Username:     username2,
			PasswordHash: passwordHash2,
		},
	}
	got, err := repo.GetAllUsers(context.Background())
	if err != nil {
		t.Fatalf("GetAllUsers error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAllUsers got: %#v | expect: %#v", got, want)
	}
}

func TestGetAllUsers_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		userId1       = 1
		username1     = "qwerty1"
		passwordHash1 = "asdfgh1"

		userId2       = 2
		username2     = "qwerty2"
		passwordHash2 = "asdfgh2"
	)
	query := regexp.QuoteMeta(`SELECT id, username, password_hash FROM users`)
	mock.ExpectQuery(query).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash"}))
	var want []User
	got, err := repo.GetAllUsers(context.Background())
	if err != nil {
		t.Fatalf("GetAllUsers error got: %v | expect: %v", err, nil)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetAllUsers got: %#v | expect: %#v", got, want)
	}
}

// =================== GetUser =====================

func TestGetUser_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		userId       = 1
		username     = "qwerty1"
		passwordHash = "asdfgh1"
	)
	query := regexp.QuoteMeta(`SELECT id, username, password_hash FROM users
	WHERE id = $1`)
	mock.ExpectQuery(query).
		WithArgs(userId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash"}).
			AddRow(userId, username, passwordHash))
	want := User{
		Id:           userId,
		Username:     username,
		PasswordHash: passwordHash,
	}
	got, err := repo.GetUser(context.Background(), userId)
	if err != nil {
		t.Fatalf("GetUser error got: %v | expect: %v", err, nil)
	}
	if got != want {
		t.Fatalf("GetUser got: %#v | expect: %#v", got, want)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		userId       = 1
		username     = "qwerty1"
		passwordHash = "asdfgh1"
	)
	query := regexp.QuoteMeta(`SELECT id, username, password_hash FROM users
	WHERE id = $1`)
	mock.ExpectQuery(query).
		WithArgs(userId).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash"}))
	_, err := repo.GetUser(context.Background(), userId)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetUser error got: %v | expect: %v", err, sql.ErrNoRows)
	}
}

// ========== GetUserByUsernameteNewUser ===========

func TestGetUserByUsername_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		userId       = 1
		username     = "qwerty1"
		passwordHash = "asdfgh1"
	)
	query := regexp.QuoteMeta(`SELECT id, username, password_hash FROM users
	WHERE username=$1`)
	mock.ExpectQuery(query).
		WithArgs(username).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash"}).
			AddRow(userId, username, passwordHash))
	want := User{
		Id:           userId,
		Username:     username,
		PasswordHash: passwordHash,
	}
	got, err := repo.GetUserByUsername(context.Background(), username)
	if err != nil {
		t.Fatalf("GetUser error got: %v | expect: %v", err, nil)
	}
	if got != want {
		t.Fatalf("GetUser got: %#v | expect: %#v", got, want)
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		userId       = 1
		username     = "qwerty1"
		passwordHash = "asdfgh1"
	)
	query := regexp.QuoteMeta(`SELECT id, username, password_hash FROM users
	WHERE username=$1`)
	mock.ExpectQuery(query).
		WithArgs(username).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "password_hash"}))
	_, err := repo.GetUserByUsername(context.Background(), username)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetUser error got: %v | expect: %v", err, sql.ErrNoRows)
	}
}

// =============== DeleteUsereNewUser ==============

func TestDeleteUser_Success(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		userId = 1
	)
	query := regexp.QuoteMeta("DELETE FROM users WHERE id = $1;")
	mock.ExpectExec(query).
		WithArgs(userId).
		WillReturnResult(sqlmock.NewResult(0, 1))
	err := repo.DeleteUser(context.Background(), userId)
	if err != nil {
		t.Fatalf("DeleteUser error got: %v | expect: %v", err, nil)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	repo, mock, cleanup := newPostgresRepoMock(t)
	defer cleanup()
	const (
		userId = 1
	)
	query := regexp.QuoteMeta("DELETE FROM users WHERE id = $1;")
	mock.ExpectExec(query).
		WithArgs(userId).
		WillReturnResult(sqlmock.NewResult(0, 0))
	err := repo.DeleteUser(context.Background(), userId)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteUser error got: %v | expect: %v", err, ErrNotFound)
	}
}

// func TestCreateNewUser_(t *testing.T) {
// 	repo, mock, cleanup := newPostgresRepoMock(t)
// 	defer cleanup()
// 	const (

// 	)
// 	query := regexp.QuoteMeta()
// 	mock.ExpectQuery().
// 		WithArgs().
// 		WillReturnRows(sqlmock.NewRows([]string{}).
// 			AddRow())
// 	err := repo.
// 	if err != nil {
// 		t.Fatalf(" error got: %v | expect: %v", err, nil)
// 	}
// }
