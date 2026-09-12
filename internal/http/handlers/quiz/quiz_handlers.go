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

// GetQuiz returns a quiz with its questions and public answers.
//
// @Summary Получить квиз
// @Description Возвращает квиз с вопросами и ответами без признака is_correct.
// @Tags quizzes
// @Produce json
// @Param id path int true "ID квиза" minimum(1)
// @Success 200 {object} QuizResponse
// @Failure 400 {object} respond.ErrorResponse "Некорректный ID квиза"
// @Failure 404 {object} respond.ErrorResponse "Квиз не найден"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /quizzes/{id} [get]
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
	// Название не должно быть пустым после удаления пробелов по краям.
	Title string `json:"title" validate:"required" minLength:"1" example:"География"`
}

// CreateQuiz creates a quiz owned by the authenticated user.
//
// @Summary Создать квиз
// @Description Автор определяется по JWT. Тело запроса должно содержать один JSON-объект без неизвестных полей.
// @Tags quizzes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateQuizRequest true "Новый квиз"
// @Success 201 "Квиз создан; тело ответа пустое"
// @Failure 400 {object} respond.ErrorResponse "Некорректный JSON или пустое название"
// @Failure 401 {object} respond.ErrorResponse "Отсутствует или недействителен Bearer-токен"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /quizzes [post]
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

// ListQuizzes returns all quizzes with their questions and public answers.
//
// @Summary Получить список квизов
// @Description Возвращает все квизы с вопросами и ответами без признака is_correct. Пагинации нет.
// @Tags quizzes
// @Produce json
// @Success 200 {array} QuizResponse
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /quizzes [get]
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

// ListQuizzesByAuthor returns an author's quizzes with public answers.
//
// @Summary Получить квизы автора
// @Description Возвращает квизы автора с вопросами и ответами без признака is_correct. Если квизов нет, возвращает пустой массив.
// @Tags quizzes
// @Produce json
// @Param authorId path int true "ID автора" minimum(1)
// @Success 200 {array} QuizResponse
// @Failure 400 {object} respond.ErrorResponse "Некорректный ID автора"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /users/{authorId}/quizzes [get]
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
