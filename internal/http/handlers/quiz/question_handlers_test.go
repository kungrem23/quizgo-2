package quizhandler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kungrem23/quizgo/internal/domain/quiz"
	"github.com/kungrem23/quizgo/internal/http/middleware"
)

type questionServiceStub struct {
	questionService

	createQuestionFunc func(ctx context.Context, textContent string, quizID, authorID int) error
	deleteQuestionFunc func(ctx context.Context, id, userID int) error
	changePositionFunc func(ctx context.Context, id, newPosition, authorID int) error
	getQuestionFunc    func(ctx context.Context, id int) (quiz.Question, error)
}

func (s questionServiceStub) CreateQuestionAsAuthor(ctx context.Context, textContent string, quizID, authorID int) error {
	return s.createQuestionFunc(ctx, textContent, quizID, authorID)
}

func (s questionServiceStub) DeleteQuestionAsAuthor(ctx context.Context, id, userID int) error {
	return s.deleteQuestionFunc(ctx, id, userID)
}

func (s questionServiceStub) GetQuestion(ctx context.Context, id int) (quiz.Question, error) {
	return s.getQuestionFunc(ctx, id)
}

func (s questionServiceStub) ChangeQuestionPosition(ctx context.Context, id, newPosition, authorID int) error {
	return s.changePositionFunc(ctx, id, newPosition, authorID)
}

// ================= GetQuestion =================

func TestGetQuestion_Success(t *testing.T) {
	want := quiz.Question{
		Id:          8,
		QuizId:      7,
		Position:    2,
		TextContent: "Capital of France?",
		ImageId:     "image-1",
		Answers: []quiz.Answer{
			{Id: 9, TextContent: "Paris", IsCorrect: true, QuestionId: 8},
		},
	}
	service := questionServiceStub{getQuestionFunc: func(_ context.Context, id int) (quiz.Question, error) {
		if id != 8 {
			t.Errorf("id = %d, want 8", id)
		}
		return want, nil
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/questions/8", nil)
	req.SetPathValue("id", "8")

	NewQuestionHandler(service).GetQuestion(recorder, req)

	assertDTOResponse(t, recorder, http.StatusOK, newQuestionResponse(want))
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
}

func TestGetQuestion_InvalidID(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/questions/nope", nil)
	req.SetPathValue("id", "nope")

	NewQuestionHandler(questionServiceStub{}).GetQuestion(recorder, req)

	assertErrorResponse(t, recorder, http.StatusBadRequest, "bad request")
}

func TestGetQuestion_ServiceErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{name: "question not found", err: fmt.Errorf("wrapped: %w", sql.ErrNoRows), status: http.StatusNotFound, message: "not found"},
		{name: "internal error", err: errors.New("database unavailable"), status: http.StatusInternalServerError, message: "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := questionServiceStub{getQuestionFunc: func(context.Context, int) (quiz.Question, error) {
				return quiz.Question{}, tt.err
			}}
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/questions/8", nil)
			req.SetPathValue("id", "8")
			NewQuestionHandler(service).GetQuestion(recorder, req)
			assertErrorResponse(t, recorder, tt.status, tt.message)
		})
	}
}

// ================ CreateQuestion ================

func TestCreateQuestion_Success(t *testing.T) {
	called := false
	service := questionServiceStub{createQuestionFunc: func(_ context.Context, text string, quizID, authorID int) error {
		called = true
		if text != "Capital of France?" || quizID != 7 || authorID != 42 {
			t.Errorf("unexpected service args: text=%q quizID=%d authorID=%d", text, quizID, authorID)
		}
		return nil
	}}
	recorder := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodPost, "/questions", `{"quiz_id":7,"text_content":"  Capital of France?  "}`)

	serveAuthenticated(NewQuestionHandler(service).CreateQuestion, recorder, req)

	assertEmptyResponse(t, recorder, http.StatusCreated)
	if !called {
		t.Error("service was not called")
	}
}

func TestCreateQuestion_Unauthorized(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/questions", strings.NewReader(`{"text_content":"Question"}`))

	NewQuestionHandler(questionServiceStub{}).CreateQuestion(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, "invalid token")
}

func TestCreateQuestion_InvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "malformed JSON", body: `{`, want: "bad request"},
		{name: "invalid quiz id", body: `{"quiz_id":0,"text_content":"Question"}`, want: "invalid quiz id"},
		{name: "empty text", body: `{"quiz_id":7,"text_content":"  \t "}`, want: "empty text content"},
		{name: "unknown field", body: `{"quiz_id":7,"text_content":"Question","extra":true}`, want: "bad request"},
		{name: "multiple JSON values", body: `{"quiz_id":7,"text_content":"Question"} {}`, want: "bad request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := authenticatedRequest(t, http.MethodPost, "/questions", tt.body)
			serveAuthenticated(NewQuestionHandler(questionServiceStub{}).CreateQuestion, recorder, req)
			assertErrorResponse(t, recorder, http.StatusBadRequest, tt.want)
		})
	}
}

func TestCreateQuestion_ServiceErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{name: "insufficient rights", err: fmt.Errorf("wrapped: %w", middleware.ErrInsufficientRights), status: http.StatusUnauthorized, message: "you cant edit this quiz"},
		{name: "quiz not found", err: fmt.Errorf("wrapped: %w", sql.ErrNoRows), status: http.StatusNotFound, message: "not found"},
		{name: "internal error", err: errors.New("database unavailable"), status: http.StatusInternalServerError, message: "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := questionServiceStub{createQuestionFunc: func(context.Context, string, int, int) error { return tt.err }}
			recorder := httptest.NewRecorder()
			req := authenticatedRequest(t, http.MethodPost, "/questions", `{"quiz_id":7,"text_content":"Question"}`)
			serveAuthenticated(NewQuestionHandler(service).CreateQuestion, recorder, req)
			assertErrorResponse(t, recorder, tt.status, tt.message)
		})
	}
}

// ================ DeleteQuestion ================

func TestDeleteQuestion_Success(t *testing.T) {
	called := false
	service := questionServiceStub{deleteQuestionFunc: func(_ context.Context, id, userID int) error {
		called = true
		if id != 8 || userID != 42 {
			t.Errorf("unexpected service args: id=%d userID=%d", id, userID)
		}
		return nil
	}}
	recorder := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodDelete, "/questions/8", "")
	req.SetPathValue("id", "8")

	serveAuthenticated(NewQuestionHandler(service).DeleteQuestion, recorder, req)

	assertEmptyResponse(t, recorder, http.StatusNoContent)
	if !called {
		t.Error("service was not called")
	}
}

func TestDeleteQuestion_Unauthorized(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/questions/8", nil)
	req.SetPathValue("id", "8")

	NewQuestionHandler(questionServiceStub{}).DeleteQuestion(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, "invalid token")
}

func TestDeleteQuestion_InvalidID(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodDelete, "/questions/nope", "")
	req.SetPathValue("id", "nope")

	serveAuthenticated(NewQuestionHandler(questionServiceStub{}).DeleteQuestion, recorder, req)

	assertErrorResponse(t, recorder, http.StatusBadRequest, "invalid question id")
}

func TestDeleteQuestion_ServiceErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{name: "insufficient rights", err: fmt.Errorf("wrapped: %w", middleware.ErrInsufficientRights), status: http.StatusUnauthorized, message: "you cant edit this question"},
		{name: "question not found", err: fmt.Errorf("wrapped: %w", sql.ErrNoRows), status: http.StatusNotFound, message: "not found"},
		{name: "internal error", err: errors.New("database unavailable"), status: http.StatusInternalServerError, message: "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := questionServiceStub{deleteQuestionFunc: func(context.Context, int, int) error { return tt.err }}
			recorder := httptest.NewRecorder()
			req := authenticatedRequest(t, http.MethodDelete, "/questions/8", "")
			req.SetPathValue("id", "8")
			serveAuthenticated(NewQuestionHandler(service).DeleteQuestion, recorder, req)
			assertErrorResponse(t, recorder, tt.status, tt.message)
		})
	}
}

// ================ ChangeQuestionPosition ================

func TestChangeQuestionPosition_Success(t *testing.T) {
	called := false
	service := questionServiceStub{changePositionFunc: func(_ context.Context, id, position, authorID int) error {
		called = true
		if id != 8 || position != 3 || authorID != 42 {
			t.Errorf("unexpected service args: id=%d position=%d authorID=%d", id, position, authorID)
		}
		return nil
	}}
	recorder := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodPatch, "/questions/8/position", `{"position":3}`)
	req.SetPathValue("id", "8")

	serveAuthenticated(NewQuestionHandler(service).ChangeQuestionPosition, recorder, req)

	assertEmptyResponse(t, recorder, http.StatusNoContent)
	if !called {
		t.Error("service was not called")
	}
}

func TestChangeQuestionPosition_Unauthorized(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/questions/8/position", strings.NewReader(`{"position":3}`))
	req.SetPathValue("id", "8")

	NewQuestionHandler(questionServiceStub{}).ChangeQuestionPosition(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, "invalid token")
}

func TestChangeQuestionPosition_InvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		id   string
		body string
		want string
	}{
		{name: "invalid id", id: "nope", body: `{"position":3}`, want: "invalid question id"},
		{name: "non-positive id", id: "0", body: `{"position":3}`, want: "invalid question id"},
		{name: "malformed JSON", id: "8", body: `{`, want: "bad request"},
		{name: "invalid position", id: "8", body: `{"position":0}`, want: "invalid position"},
		{name: "multiple JSON values", id: "8", body: `{"position":3} {}`, want: "bad request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := authenticatedRequest(t, http.MethodPatch, "/questions/"+tt.id+"/position", tt.body)
			req.SetPathValue("id", tt.id)
			serveAuthenticated(NewQuestionHandler(questionServiceStub{}).ChangeQuestionPosition, recorder, req)
			assertErrorResponse(t, recorder, http.StatusBadRequest, tt.want)
		})
	}
}

func TestChangeQuestionPosition_ServiceErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{name: "insufficient rights", err: fmt.Errorf("wrapped: %w", middleware.ErrInsufficientRights), status: http.StatusUnauthorized, message: "you cant edit this question"},
		{name: "question not found", err: fmt.Errorf("wrapped: %w", sql.ErrNoRows), status: http.StatusNotFound, message: "not found"},
		{name: "invalid position", err: fmt.Errorf("wrapped: %w", quiz.ErrInvalidQuestionPosition), status: http.StatusBadRequest, message: "invalid position"},
		{name: "internal error", err: errors.New("database unavailable"), status: http.StatusInternalServerError, message: "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := questionServiceStub{changePositionFunc: func(context.Context, int, int, int) error { return tt.err }}
			recorder := httptest.NewRecorder()
			req := authenticatedRequest(t, http.MethodPatch, "/questions/8/position", `{"position":3}`)
			req.SetPathValue("id", "8")
			serveAuthenticated(NewQuestionHandler(service).ChangeQuestionPosition, recorder, req)
			assertErrorResponse(t, recorder, tt.status, tt.message)
		})
	}
}
