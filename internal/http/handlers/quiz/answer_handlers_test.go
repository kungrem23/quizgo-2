package quizhandler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/kungrem23/quizgo/internal/domain/quiz"
	"github.com/kungrem23/quizgo/internal/http/middleware"
	"github.com/kungrem23/quizgo/internal/http/middleware/respond"
	"github.com/kungrem23/quizgo/internal/utils"
)

type answerServiceStub struct {
	answerService

	createAnswerFunc      func(ctx context.Context, textContent string, isCorrect bool, questionID, userID int) error
	deleteAnswerFunc      func(ctx context.Context, id, userID int) error
	getAnswerFunc         func(ctx context.Context, id int) (quiz.Answer, error)
	getAnswerAsAuthorFunc func(ctx context.Context, id, userID int) (quiz.Answer, error)
}

func (s answerServiceStub) CreateAnswerAsAuthor(ctx context.Context, textContent string, isCorrect bool, questionID, userID int) error {
	return s.createAnswerFunc(ctx, textContent, isCorrect, questionID, userID)
}

func (s answerServiceStub) DeleteAnswerAsAuthor(ctx context.Context, id, userID int) error {
	return s.deleteAnswerFunc(ctx, id, userID)
}

func (s answerServiceStub) GetAnswer(ctx context.Context, id int) (quiz.Answer, error) {
	return s.getAnswerFunc(ctx, id)
}

func (s answerServiceStub) GetAnswerAsAuthor(ctx context.Context, id, userID int) (quiz.Answer, error) {
	return s.getAnswerAsAuthorFunc(ctx, id, userID)
}

func authenticatedRequest(t *testing.T, method, target, body string) *http.Request {
	t.Helper()
	token, err := utils.GenerateJWT(42)
	if err != nil {
		t.Fatalf("generate JWT: %v", err)
	}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func serveAuthenticated(handler http.HandlerFunc, recorder *httptest.ResponseRecorder, request *http.Request) {
	middleware.Auth(handler).ServeHTTP(recorder, request)
}

func assertDTOResponse[T any](t *testing.T, recorder *httptest.ResponseRecorder, status int, want T) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d; body = %q", recorder.Code, status, recorder.Body.String())
	}
	var got T
	decoder := json.NewDecoder(recorder.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&got); err != nil {
		t.Fatalf("decode response DTO: %v; body = %q", err, recorder.Body.String())
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("response DTO = %#v, want %#v", got, want)
	}
}

func assertErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	assertDTOResponse(t, recorder, status, respond.ErrorResponse{Error: message})
}

func assertEmptyResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("status = %d, want %d; body = %q", recorder.Code, status, recorder.Body.String())
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("body = %q, want empty body", recorder.Body.String())
	}
}

// ================ CreateAnswer ================

func TestCreateAnswer_Success(t *testing.T) {
	called := false
	service := answerServiceStub{createAnswerFunc: func(_ context.Context, text string, correct bool, questionID, userID int) error {
		called = true
		if text != "Paris" || !correct || questionID != 7 || userID != 42 {
			t.Errorf("unexpected service args: text=%q correct=%v questionID=%d userID=%d", text, correct, questionID, userID)
		}
		return nil
	}}
	recorder := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodPost, "/answers", `{"text_content":"  Paris  ","is_correct":true,"question_id":7}`)

	serveAuthenticated(NewAnswerHandler(service).CreateAnswer, recorder, req)

	assertEmptyResponse(t, recorder, http.StatusCreated)
	if !called {
		t.Error("service was not called")
	}
}

func TestCreateAnswer_Unauthorized(t *testing.T) {
	handler := NewAnswerHandler(answerServiceStub{})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/answers", strings.NewReader(`{"text_content":"Paris"}`))

	handler.CreateAnswer(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, "invalid token")
}

func TestCreateAnswer_InvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "malformed JSON", body: `{`, want: "bad request"},
		{name: "invalid question id", body: `{"question_id":0,"text_content":"Answer"}`, want: "invalid question id"},
		{name: "empty text", body: `{"text_content":"  \t "}`, want: "empty text content"},
		{name: "unknown field", body: `{"question_id":7,"text_content":"Answer","extra":true}`, want: "bad request"},
		{name: "multiple JSON values", body: `{"question_id":7,"text_content":"Answer"} {}`, want: "bad request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := authenticatedRequest(t, http.MethodPost, "/answers", tt.body)
			serveAuthenticated(NewAnswerHandler(answerServiceStub{}).CreateAnswer, recorder, req)
			assertErrorResponse(t, recorder, http.StatusBadRequest, tt.want)
		})
	}
}

func TestCreateAnswer_ServiceErrors(t *testing.T) {
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
			service := answerServiceStub{createAnswerFunc: func(context.Context, string, bool, int, int) error { return tt.err }}
			recorder := httptest.NewRecorder()
			req := authenticatedRequest(t, http.MethodPost, "/answers", `{"text_content":"Paris","question_id":7}`)
			serveAuthenticated(NewAnswerHandler(service).CreateAnswer, recorder, req)
			assertErrorResponse(t, recorder, tt.status, tt.message)
		})
	}
}

// ================ DeleteAnswer ================

func TestDeleteAnswer_Success(t *testing.T) {
	called := false
	service := answerServiceStub{deleteAnswerFunc: func(_ context.Context, id, userID int) error {
		called = true
		if id != 8 || userID != 42 {
			t.Errorf("unexpected service args: id=%d userID=%d", id, userID)
		}
		return nil
	}}
	recorder := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodDelete, "/answers/8", "")
	req.SetPathValue("id", "8")

	serveAuthenticated(NewAnswerHandler(service).DeleteAnswer, recorder, req)

	assertEmptyResponse(t, recorder, http.StatusNoContent)
	if !called {
		t.Error("service was not called")
	}
}

func TestDeleteAnswer_Unauthorized(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/answers/8", nil)
	req.SetPathValue("id", "8")

	NewAnswerHandler(answerServiceStub{}).DeleteAnswer(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, "invalid token")
}

func TestDeleteAnswer_InvalidID(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodDelete, "/answers/nope", "")
	req.SetPathValue("id", "nope")

	serveAuthenticated(NewAnswerHandler(answerServiceStub{}).DeleteAnswer, recorder, req)

	assertErrorResponse(t, recorder, http.StatusBadRequest, "invalid answer id")
}

func TestDeleteAnswer_ServiceErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{name: "insufficient rights", err: fmt.Errorf("wrapped: %w", middleware.ErrInsufficientRights), status: http.StatusUnauthorized, message: "you cant edit this answer"},
		{name: "answer not found", err: fmt.Errorf("wrapped: %w", sql.ErrNoRows), status: http.StatusNotFound, message: "not found"},
		{name: "internal error", err: errors.New("database unavailable"), status: http.StatusInternalServerError, message: "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := answerServiceStub{deleteAnswerFunc: func(context.Context, int, int) error { return tt.err }}
			recorder := httptest.NewRecorder()
			req := authenticatedRequest(t, http.MethodDelete, "/answers/8", "")
			req.SetPathValue("id", "8")
			serveAuthenticated(NewAnswerHandler(service).DeleteAnswer, recorder, req)
			assertErrorResponse(t, recorder, tt.status, tt.message)
		})
	}
}

// ================= GetAnswer ==================

func TestGetAnswer_Success(t *testing.T) {
	want := quiz.Answer{Id: 8, TextContent: "Paris", IsCorrect: true, QuestionId: 7}
	service := answerServiceStub{getAnswerFunc: func(_ context.Context, id int) (quiz.Answer, error) {
		if id != 8 {
			t.Errorf("id = %d, want 8", id)
		}
		return want, nil
	}}
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/answers/8", nil)
	req.SetPathValue("id", "8")

	NewAnswerHandler(service).GetAnswer(recorder, req)

	assertDTOResponse(t, recorder, http.StatusOK, newPublicAnswerResponse(want))
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
}

// ================= GetAnswerAsAuthor ==================

func TestGetAnswerAsAuthor_Success(t *testing.T) {
	want := quiz.Answer{Id: 8, TextContent: "Paris", IsCorrect: true, QuestionId: 7}
	service := answerServiceStub{getAnswerAsAuthorFunc: func(_ context.Context, id, userID int) (quiz.Answer, error) {
		if id != 8 || userID != 42 {
			t.Errorf("unexpected service args: id=%d userID=%d", id, userID)
		}
		return want, nil
	}}
	recorder := httptest.NewRecorder()
	req := authenticatedRequest(t, http.MethodGet, "/answers/8/author", "")
	req.SetPathValue("id", "8")

	serveAuthenticated(NewAnswerHandler(service).GetAnswerAsAuthor, recorder, req)

	assertDTOResponse(t, recorder, http.StatusOK, newAuthorAnswerResponse(want))
}

func TestGetAnswerAsAuthor_Unauthorized(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/answers/8/author", nil)
	req.SetPathValue("id", "8")

	NewAnswerHandler(answerServiceStub{}).GetAnswerAsAuthor(recorder, req)

	assertErrorResponse(t, recorder, http.StatusUnauthorized, "invalid token")
}

func TestGetAnswerAsAuthor_ServiceErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{name: "insufficient rights", err: middleware.ErrInsufficientRights, status: http.StatusUnauthorized, message: "you cant view this answer"},
		{name: "not found", err: sql.ErrNoRows, status: http.StatusNotFound, message: "not found"},
		{name: "internal error", err: errors.New("database unavailable"), status: http.StatusInternalServerError, message: "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := answerServiceStub{getAnswerAsAuthorFunc: func(context.Context, int, int) (quiz.Answer, error) {
				return quiz.Answer{}, tt.err
			}}
			recorder := httptest.NewRecorder()
			req := authenticatedRequest(t, http.MethodGet, "/answers/8/author", "")
			req.SetPathValue("id", "8")
			serveAuthenticated(NewAnswerHandler(service).GetAnswerAsAuthor, recorder, req)
			assertErrorResponse(t, recorder, tt.status, tt.message)
		})
	}
}

func TestGetAnswer_InvalidID(t *testing.T) {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/answers/nope", nil)
	req.SetPathValue("id", "nope")

	NewAnswerHandler(answerServiceStub{}).GetAnswer(recorder, req)

	assertErrorResponse(t, recorder, http.StatusBadRequest, "invalid answer id")
}

func TestGetAnswer_ServiceErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{name: "answer not found", err: fmt.Errorf("wrapped: %w", sql.ErrNoRows), status: http.StatusNotFound, message: "not found"},
		{name: "internal error", err: errors.New("database unavailable"), status: http.StatusInternalServerError, message: "server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := answerServiceStub{getAnswerFunc: func(context.Context, int) (quiz.Answer, error) {
				return quiz.Answer{}, tt.err
			}}
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/answers/8", nil)
			req.SetPathValue("id", "8")
			NewAnswerHandler(service).GetAnswer(recorder, req)
			assertErrorResponse(t, recorder, tt.status, tt.message)
		})
	}
}
