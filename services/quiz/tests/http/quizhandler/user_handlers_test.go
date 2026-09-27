package quizhandler_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/http/handlers/quiz"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kungrem23/quizgo/services/quiz/internal/domain"
)

type userServiceStub struct {
	getUserFunc func(ctx context.Context, id int) (quiz.User, error)
}

func (s userServiceStub) GetUser(ctx context.Context, id int) (quiz.User, error) {
	return s.getUserFunc(ctx, id)
}

func TestGetUser_Success(t *testing.T) {
	service := userServiceStub{getUserFunc: func(_ context.Context, id int) (quiz.User, error) {
		if id != 42 {
			t.Errorf("id = %d, want 42", id)
		}
		return quiz.User{Id: 42, Username: "alice", PasswordHash: "must-not-leak"}, nil
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	req.SetPathValue("id", "42")

	NewUserHandler(service).GetUser(recorder, req)

	assertDTOResponse(t, recorder, http.StatusOK, UserResponse{ID: 42, Username: "alice"})
}

func TestGetUser_InvalidID(t *testing.T) {
	for _, id := range []string{"nope", "0", "-1"} {
		t.Run(id, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/users/"+id, nil)
			req.SetPathValue("id", id)

			NewUserHandler(userServiceStub{}).GetUser(recorder, req)

			assertErrorResponse(t, recorder, http.StatusBadRequest, "invalid user id")
		})
	}
}

func TestGetUser_ServiceErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{name: "not found", err: fmt.Errorf("wrapped: %w", sql.ErrNoRows), status: http.StatusNotFound, message: "not found"},
		{name: "internal error", err: errors.New("database unavailable"), status: http.StatusInternalServerError, message: "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := userServiceStub{getUserFunc: func(context.Context, int) (quiz.User, error) {
				return quiz.User{}, tt.err
			}}
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
			req.SetPathValue("id", "42")

			NewUserHandler(service).GetUser(recorder, req)

			assertErrorResponse(t, recorder, tt.status, tt.message)
		})
	}
}
