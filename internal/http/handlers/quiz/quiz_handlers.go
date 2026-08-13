package quizhandler

import (
	"database/sql"
	"strings"

	// "encoding/json"
	"errors"
	"net/http"
	"strconv"

	// "github.com/gorilla/mux"
	// "github.com/golang-jwt/jwt/v5"
	// "github.com/kungrem23/quizgo/internal/domain/quiz"
	"github.com/kungrem23/quizgo/internal/http/middleware"
	"github.com/kungrem23/quizgo/internal/http/middleware/respond"
	// "github.com/kungrem23/quizgo/internal/utils"
	// "github.com/kungrem23/quizgo/internal/store/models"
)

type QuizHandler struct {
	service quizService
}

func NewQuizHandler(service quizService) *QuizHandler {
	return &QuizHandler{service: service}
}

func (h *QuizHandler) GetQuiz(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "bad request",
		})
		return
	}
	quiz, err := h.service.GetQuiz(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respond.WriteJSON(w, http.StatusNotFound, respond.ErrorResponse{
				Error: "not found",
			})
			return
		} else {
			respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{
				Error: "server error",
			})
			return
		}

	}
	respond.WriteJSON(w, http.StatusOK, newQuizResponse(quiz))
}

type CreateQuizRequest struct {
	Title string `json:"title"`
}

func (h *QuizHandler) CreateQuiz(w http.ResponseWriter, r *http.Request) {
	authorId, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
			Error: "invalid token",
		})
		return
	}
	var req CreateQuizRequest
	err := decodeJSON(r, &req)
	if err != nil {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "bad request",
		})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "empty title",
		})
		return
	}
	err = h.service.CreateQuiz(r.Context(), req.Title, authorId)
	if err != nil {
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{
			Error: "server error",
		})
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *QuizHandler) ListQuizzes(w http.ResponseWriter, r *http.Request) {
	quizzes, err := h.service.ListQuizzes(r.Context())
	if err != nil {
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{
			Error: "server error",
		})
		return
	}
	respond.WriteJSON(w, http.StatusOK, newQuizResponses(quizzes))
}

func (h *QuizHandler) ListQuizzesByAuthor(w http.ResponseWriter, r *http.Request) {
	authorIdStr := r.PathValue("authorId")
	authorId, err := strconv.Atoi(authorIdStr)
	if err != nil || authorId <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "invalid id",
		})
		return
	}
	quizzes, err := h.service.ListQuizzesByAuthor(r.Context(), authorId)
	if err != nil {
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{
			Error: "server error",
		})
		return
	}
	respond.WriteJSON(w, http.StatusOK, newQuizResponses(quizzes))
}
