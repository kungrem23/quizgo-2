package quizhandler_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/http/handlers/quiz"

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kungrem23/quizgo/services/quiz/internal/domain"
)

type quizServiceStub struct {
	getQuizFunc             func(ctx context.Context, id int) (quiz.Quiz, error)
	createQuizFunc          func(ctx context.Context, title string, authorID int) error
	listQuizzesFunc         func(ctx context.Context) ([]quiz.Quiz, error)
	listQuizzesByAuthorFunc func(ctx context.Context, authorID int) ([]quiz.Quiz, error)
}

func (s quizServiceStub) GetQuiz(ctx context.Context, id int) (quiz.Quiz, error) {
	return s.getQuizFunc(ctx, id)
}

func (s quizServiceStub) CreateQuiz(ctx context.Context, title string, authorID int) (quiz.Quiz, error) {
	err := s.createQuizFunc(ctx, title, authorID)
	return quiz.Quiz{Id: 7, Title: title, AuthorId: authorID, Questions: []*quiz.Question{}}, err
}

func (s quizServiceStub) ListQuizzes(ctx context.Context) ([]quiz.Quiz, error) {
	return s.listQuizzesFunc(ctx)
}

func (s quizServiceStub) ListQuizzesByAuthor(ctx context.Context, authorID int) ([]quiz.Quiz, error) {
	return s.listQuizzesByAuthorFunc(ctx, authorID)
}

// ================= GetQuiz =================

func TestGetQuiz_Success(t *testing.T) {
	want := quiz.Quiz{
		Id:       7,
		Title:    "Geography",
		AuthorId: 42,
		Questions: []*quiz.Question{
			{Id: 8, QuizId: 7, Position: 1, TextContent: "Capital of France?", ImageId: "image-1", Answers: []quiz.Answer{}},
		},
	}
	service := quizServiceStub{getQuizFunc: func(_ context.Context, id int) (quiz.Quiz, error) {
		if id != 7 {
			t.Errorf("id = %d, want 7", id)
		}
		return want, nil
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/quizzes/7", nil)
	req.SetPathValue("id", "7")

	NewQuizHandler(service).GetQuiz(recorder, req)

	assertDTOResponse(t, recorder, http.StatusOK, newQuizResponse(want))
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
}

func TestGetQuiz_InvalidID(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/quizzes/nope", nil)
	req.SetPathValue("id", "nope")

	NewQuizHandler(quizServiceStub{}).GetQuiz(recorder, req)

	assertErrorResponse(t, recorder, http.StatusBadRequest, "bad request")
}

func TestGetQuiz_ServiceErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{name: "quiz not found", err: fmt.Errorf("wrapped: %w", sql.ErrNoRows), status: http.StatusNotFound, message: "not found"},
		{name: "internal error", err: errors.New("database unavailable"), status: http.StatusInternalServerError, message: "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := quizServiceStub{getQuizFunc: func(context.Context, int) (quiz.Quiz, error) {
				return quiz.Quiz{}, tt.err
			}}
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/quizzes/7", nil)
			req.SetPathValue("id", "7")
			NewQuizHandler(service).GetQuiz(recorder, req)
			assertErrorResponse(t, recorder, tt.status, tt.message)
		})
	}
}

// ================= CreateQuiz =================

func TestCreateQuiz_Success(t *testing.T) {
	called := false
	service := quizServiceStub{createQuizFunc: func(_ context.Context, title string, authorID int) error {
		called = true
		if title != "Geography" || authorID != 42 {
			t.Errorf("unexpected service args: title=%q authorID=%d", title, authorID)
		}
		return nil
	}}
	recorder := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodPost, "/quizzes", `{"title":"  Geography  "}`)

	serveAuthenticated(NewQuizHandler(service).CreateQuiz, recorder, req)

	assertDTOResponse(t, recorder, http.StatusCreated, newQuizResponse(quiz.Quiz{Id: 7, Title: "Geography", AuthorId: 42, Questions: []*quiz.Question{}}))
	if recorder.Header().Get("Location") != "/api/quizzes/7" {
		t.Error("missing resource location")
	}
	if !called {
		t.Error("service was not called")
	}
}

func TestCreateQuiz_Unauthorized(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/quizzes", strings.NewReader(`{"title":"Geography"}`))

	NewQuizHandler(quizServiceStub{}).CreateQuiz(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, "invalid token")
}

func TestCreateQuiz_InvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "malformed JSON", body: `{`, want: "bad request"},
		{name: "empty title", body: `{"title":"  \t "}`, want: "empty title"},
		{name: "unknown field", body: `{"title":"Quiz","extra":true}`, want: "bad request"},
		{name: "multiple JSON values", body: `{"title":"Quiz"} {}`, want: "bad request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := authenticatedRequest(t, http.MethodPost, "/quizzes", tt.body)
			serveAuthenticated(NewQuizHandler(quizServiceStub{}).CreateQuiz, recorder, req)
			assertErrorResponse(t, recorder, http.StatusBadRequest, tt.want)
		})
	}
}

func TestCreateQuiz_ServiceError(t *testing.T) {
	service := quizServiceStub{createQuizFunc: func(context.Context, string, int) error {
		return errors.New("database unavailable")
	}}
	recorder := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodPost, "/quizzes", `{"title":"Geography"}`)

	serveAuthenticated(NewQuizHandler(service).CreateQuiz, recorder, req)

	assertErrorResponse(t, recorder, http.StatusInternalServerError, "server error")
}

// ================= ListQuizzes =================

func TestListQuizzes_Success(t *testing.T) {
	want := []quiz.Quiz{
		{Id: 1, Title: "Geography", AuthorId: 42, Questions: []*quiz.Question{}},
		{Id: 2, Title: "History", AuthorId: 43, Questions: nil},
	}
	service := quizServiceStub{listQuizzesFunc: func(context.Context) ([]quiz.Quiz, error) {
		return want, nil
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/quizzes", nil)

	NewQuizHandler(service).ListQuizzes(recorder, req)

	assertDTOResponse(t, recorder, http.StatusOK, newQuizResponses(want))
}

func TestListQuizzes_ServiceError(t *testing.T) {
	service := quizServiceStub{listQuizzesFunc: func(context.Context) ([]quiz.Quiz, error) {
		return nil, errors.New("database unavailable")
	}}
	recorder := httptest.NewRecorder()

	NewQuizHandler(service).ListQuizzes(recorder, httptest.NewRequest(http.MethodGet, "/quizzes", nil))

	assertErrorResponse(t, recorder, http.StatusInternalServerError, "server error")
}

// ================= ListQuizzesByAuthor =================

func TestListQuizzesByAuthor_Success(t *testing.T) {
	want := []quiz.Quiz{{Id: 1, Title: "Geography", AuthorId: 42, Questions: []*quiz.Question{}}}
	service := quizServiceStub{listQuizzesByAuthorFunc: func(_ context.Context, authorID int) ([]quiz.Quiz, error) {
		if authorID != 42 {
			t.Errorf("authorID = %d, want 42", authorID)
		}
		return want, nil
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/authors/42/quizzes", nil)
	req.SetPathValue("authorId", "42")

	NewQuizHandler(service).ListQuizzesByAuthor(recorder, req)

	assertDTOResponse(t, recorder, http.StatusOK, newQuizResponses(want))
}

func TestListQuizzesByAuthor_InvalidID(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/authors/nope/quizzes", nil)
	req.SetPathValue("authorId", "nope")

	NewQuizHandler(quizServiceStub{}).ListQuizzesByAuthor(recorder, req)

	assertErrorResponse(t, recorder, http.StatusBadRequest, "invalid id")
}

func TestListQuizzesByAuthor_ServiceError(t *testing.T) {
	service := quizServiceStub{listQuizzesByAuthorFunc: func(context.Context, int) ([]quiz.Quiz, error) {
		return nil, errors.New("database unavailable")
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/authors/42/quizzes", nil)
	req.SetPathValue("authorId", "42")

	NewQuizHandler(service).ListQuizzesByAuthor(recorder, req)

	assertErrorResponse(t, recorder, http.StatusInternalServerError, "server error")
}
