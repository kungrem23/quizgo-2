package quizhandler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	// "github.com/golang-jwt/jwt/v5"
	"github.com/kungrem23/quizgo/internal/domain/quiz"
	"github.com/kungrem23/quizgo/internal/http/middleware"
	"github.com/kungrem23/quizgo/internal/http/middleware/respond"
	// "github.com/kungrem23/quizgo/internal/utils"
)

type QuestionHandler struct {
	service questionService
}

func NewQuestionHandler(service questionService) *QuestionHandler {
	return &QuestionHandler{service: service}
}

func (h *QuestionHandler) GetQuestion(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "bad request",
		})
		return
	}
	question, err := h.service.GetQuestion(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.WriteJSON(w, http.StatusNotFound, respond.ErrorResponse{
				Error: "not found",
			})
			return
		}
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{
			Error: "server error",
		})
		return
	}
	respond.WriteJSON(w, http.StatusOK, newQuestionResponse(question))
}

type CreateQuestionRequest struct {
	QuizId      int    `json:"quiz_id"`
	TextContent string `json:"text_content"`
}

type ChangeQuestionPositionRequest struct {
	Position int `json:"position"`
}

func (h *QuestionHandler) CreateQuestion(w http.ResponseWriter, r *http.Request) {
	userId, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
			Error: "invalid token",
		})
		return
	}
	var req CreateQuestionRequest
	err := decodeJSON(r, &req)
	if err != nil {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "bad request",
		})
		return
	}
	req.TextContent = strings.TrimSpace(req.TextContent)
	if req.QuizId <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "invalid quiz id",
		})
		return
	}
	if req.TextContent == "" {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "empty text content",
		})
		return
	}
	err = h.service.CreateQuestionAsAuthor(r.Context(), req.TextContent, req.QuizId, userId)
	if err != nil {
		if errors.Is(err, middleware.ErrInsufficientRights) {
			respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
				Error: "you cant edit this quiz",
			})
			return
		} else if errors.Is(err, sql.ErrNoRows) {
			respond.WriteJSON(w, http.StatusNotFound, respond.ErrorResponse{
				Error: "not found",
			})
			return
		}
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{
			Error: "server error",
		})
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *QuestionHandler) DeleteQuestion(w http.ResponseWriter, r *http.Request) {
	userId, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
			Error: "invalid token",
		})
		return
	}
	questionIdStr := r.PathValue("id")
	questionId, err := strconv.Atoi(questionIdStr)
	if err != nil || questionId <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "invalid question id",
		})
		return
	}
	err = h.service.DeleteQuestionAsAuthor(r.Context(), questionId, userId)
	if err != nil {
		if errors.Is(err, middleware.ErrInsufficientRights) {
			respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
				Error: "you cant edit this question",
			})
			return
		} else if errors.Is(err, sql.ErrNoRows) {
			respond.WriteJSON(w, http.StatusNotFound, respond.ErrorResponse{
				Error: "not found",
			})
			return
		}
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{
			Error: "server error",
		})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *QuestionHandler) ChangeQuestionPosition(w http.ResponseWriter, r *http.Request) {
	userId, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
			Error: "invalid token",
		})
		return
	}

	questionId, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || questionId <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "invalid question id",
		})
		return
	}

	var req ChangeQuestionPositionRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "bad request",
		})
		return
	}
	if req.Position <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "invalid position",
		})
		return
	}

	err = h.service.ChangeQuestionPosition(r.Context(), questionId, req.Position, userId)
	if err != nil {
		if errors.Is(err, quiz.ErrInvalidQuestionPosition) {
			respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
				Error: "invalid position",
			})
			return
		}
		if errors.Is(err, middleware.ErrInsufficientRights) {
			respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
				Error: "you cant edit this question",
			})
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			respond.WriteJSON(w, http.StatusNotFound, respond.ErrorResponse{
				Error: "not found",
			})
			return
		}
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{
			Error: "server error",
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *QuestionHandler) ListQuestions(w http.ResponseWriter, r *http.Request) {

}
