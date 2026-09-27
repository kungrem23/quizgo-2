package quiz_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/domain"

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/kungrem23/quizgo/services/quiz/internal/authn"
	"github.com/kungrem23/quizgo/services/quiz/internal/security"
)

type serviceRepoStub struct {
	Repository

	getUserByUsernameFunc func(context.Context, string) (User, error)
	createNewUserFunc     func(context.Context, string, string) error
}

func serviceTestTokens(t *testing.T) *authn.Manager {
	t.Helper()
	manager, err := authn.NewManager(authn.Config{
		Secret: "0123456789abcdef0123456789abcdef", Issuer: "quizgo", Audience: "quizgo-test", TTL: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func (r serviceRepoStub) GetUserByUsername(
	ctx context.Context, username string) (User, error) {
	return r.getUserByUsernameFunc(ctx, username)
}

func (r serviceRepoStub) CreateNewUser(
	ctx context.Context, username, passwordHash string) error {
	return r.createNewUserFunc(ctx, username, passwordHash)
}

// ================== Login ==================

func TestServiceLogin_Success(t *testing.T) {
	const (
		username = "username"
		password = "password"
	)
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("ServiceLogin HashPassword error got: %v | expect: %v",
			err, nil)
	}
	repo := serviceRepoStub{
		getUserByUsernameFunc: func(_ context.Context,
			gotUsername string) (User, error) {
			if gotUsername != username {
				t.Errorf("ServiceLogin error got username: %v | expect %v", gotUsername, username)
			}
			return User{
				Id:           7,
				Username:     username,
				PasswordHash: passwordHash,
			}, nil
		},
	}
	service := NewServiceWithTokens(repo, nil, serviceTestTokens(t))
	token, err := service.Login(context.Background(), username, password)
	if err != nil {
		t.Fatalf("ServiceLogin error got: %v | expect: %v", err, nil)
	}
	if token == "" {
		t.Fatalf("ServiceLogin got empty token")
	}
}

func TestServiceLogin_NotFound(t *testing.T) {
	const (
		username = "username"
		password = "password"
	)
	_, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("ServiceLogin HashPassword error got: %v | expect: %v",
			err, nil)
	}
	repo := serviceRepoStub{
		getUserByUsernameFunc: func(_ context.Context,
			gotUsername string) (User, error) {
			if gotUsername != username {
				t.Errorf("ServiceLogin error got username: %v | expect %v", gotUsername, username)
			}
			return User{}, sql.ErrNoRows
		},
	}
	service := NewService(repo)
	_, err = service.Login(context.Background(), username, password)
	if !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("ServiceLogin error got: %v | expect: %v", err, ErrInvalidUsername)
	}
}

func TestServiceLogin_Forbidden(t *testing.T) {
	const (
		username = "username"
		password = "password"
	)
	_, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("ServiceLogin HashPassword error got: %v | expect: %v",
			err, nil)
	}
	repo := serviceRepoStub{
		getUserByUsernameFunc: func(_ context.Context,
			gotUsername string) (User, error) {
			if gotUsername != username {
				t.Errorf("ServiceLogin error got username: %v | expect %v", gotUsername, username)
			}
			return User{
				Id:           7,
				Username:     username,
				PasswordHash: "bebra",
			}, nil
		},
	}
	service := NewService(repo)
	_, err = service.Login(context.Background(), username, password)
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("ServiceLogin error got: %v | expect: %v", err, ErrInvalidPassword)
	}
}

// ================== Register ==================

func TestServiceRegister_Success(t *testing.T) {
	const (
		username = "qwerty123"
		password = "12345678"
	)
	_, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("ServiceRegister HashPassword error got: %v | expect: %v", err, nil)
	}
	repo := serviceRepoStub{
		getUserByUsernameFunc: func(ctx context.Context,
			gotUsername string) (User, error) {
			if gotUsername != username {
				t.Errorf("ServiceRegister error got username: %v | expect %v", gotUsername, username)
			}
			return User{}, sql.ErrNoRows
		},
		createNewUserFunc: func(ctx context.Context,
			gotUsername, gotPasswordHash string) error {
			if gotUsername != username {
				t.Errorf("ServiceRegister got username: \"%v\" | expect username: \"%v\"",
					username, gotUsername)
			}
			if !security.CheckPasswordHash(password, gotPasswordHash) {
				t.Errorf("ServiceRegister passwordHash is not valid")
			}
			return nil
		},
	}
	service := NewService(repo)
	err = service.Register(context.Background(), username, password)
	if err != nil {
		t.Fatalf("ServiceRegister error got: %v | expect: %v", err, nil)
	}
}

func TestServiceRegister_UsernameTaken(t *testing.T) {
	const (
		username = "qwerty123"
		password = "12345678"
	)
	passwordHash, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("ServiceRegister HashPassword error got: %v | expect: %v", err, nil)
	}
	repo := serviceRepoStub{
		getUserByUsernameFunc: func(ctx context.Context,
			gotUsername string) (User, error) {
			if gotUsername != username {
				t.Errorf("ServiceRegister error got username: %v | expect %v", gotUsername, username)
			}
			return User{
				Id:           7,
				Username:     username,
				PasswordHash: passwordHash,
			}, nil
		},
	}
	service := NewService(repo)
	err = service.Register(context.Background(), username, password)
	if !errors.Is(err, ErrTakenUsername) {
		t.Fatalf("ServiceRegister error got: %v | expect: %v", err, ErrTakenUsername)
	}
}

func TestServiceRegister_GetUserError(t *testing.T) {
	const (
		username = "qwerty123"
		password = "12345678"
	)
	_, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("ServiceRegister HashPassword error got: %v | expect: %v", err, nil)
	}
	repo := serviceRepoStub{
		getUserByUsernameFunc: func(ctx context.Context,
			gotUsername string) (User, error) {
			if gotUsername != username {
				t.Errorf("ServiceRegister error got username: %v | expect %v", gotUsername, username)
			}
			return User{}, ErrCustom
		},
	}
	service := NewService(repo)
	err = service.Register(context.Background(), username, password)
	if !errors.Is(err, ErrCustom) {
		t.Fatalf("ServiceRegister error got: %v | expect: %v", err, ErrCustom)
	}
}

func TestServiceRegister_CreateUserError(t *testing.T) {
	const (
		username = "qwerty123"
		password = "12345678"
	)
	_, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("ServiceRegister HashPassword error got: %v | expect: %v", err, nil)
	}
	repo := serviceRepoStub{
		getUserByUsernameFunc: func(ctx context.Context,
			gotUsername string) (User, error) {
			if gotUsername != username {
				t.Errorf("ServiceRegister error got username: %v | expect %v", gotUsername, username)
			}
			return User{}, sql.ErrNoRows
		},
		createNewUserFunc: func(ctx context.Context,
			gotUsername, gotPasswordHash string) error {
			if gotUsername != username {
				t.Errorf("ServiceRegister got username: \"%v\" | expect username: \"%v\"",
					username, gotUsername)
			}
			if !security.CheckPasswordHash(password, gotPasswordHash) {
				t.Errorf("ServiceRegister passwordHash is not valid")
			}
			return ErrCustom
		},
	}
	service := NewService(repo)
	err = service.Register(context.Background(), username, password)
	if !errors.Is(err, ErrCustom) {
		t.Fatalf("ServiceRegister error got: %v | expect: %v", err, ErrCustom)
	}
}
