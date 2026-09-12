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

// GetQuestion returns a question without loading its answers.
//
// @Summary Получить вопрос
// @Description Возвращает вопрос. В текущей реализации поле answers равно null; ответы загружаются при получении квиза.
// @Tags questions
// @Produce json
// @Param id path int true "ID вопроса" minimum(1)
// @Success 200 {object} QuestionResponse
// @Failure 400 {object} respond.ErrorResponse "Некорректный ID вопроса"
// @Failure 404 {object} respond.ErrorResponse "Вопрос не найден"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /questions/{id} [get]
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
	QuizId int `json:"quiz_id" validate:"required" minimum:"1" example:"1"`
	// Текст не должен быть пустым после удаления пробелов по краям.
	TextContent string `json:"text_content" validate:"required" minLength:"1" example:"Столица Франции?"`
}

type ChangeQuestionPositionRequest struct {
	// Позиция начинается с 1 и не может превышать число вопросов в квизе.
	Position int `json:"position" validate:"required" minimum:"1" example:"2"`
}

// CreateQuestion adds a question to a quiz owned by the authenticated user.
//
// @Summary Создать вопрос
// @Description Доступно только автору квиза. Тело запроса должно содержать один JSON-объект без неизвестных полей.
// @Tags questions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateQuestionRequest true "Новый вопрос"
// @Success 201 "Вопрос создан; тело ответа пустое"
// @Failure 400 {object} respond.ErrorResponse "Некорректный JSON, ID квиза или пустой текст"
// @Failure 401 {object} respond.ErrorResponse "Недействительный токен или пользователь не является автором квиза"
// @Failure 404 {object} respond.ErrorResponse "Квиз не найден"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /questions [post]
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

// DeleteQuestion deletes a question owned by the authenticated user.
//
// @Summary Удалить вопрос
// @Description Доступно только автору квиза.
// @Tags questions
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID вопроса" minimum(1)
// @Success 204 "Вопрос удалён; тело ответа пустое"
// @Failure 400 {object} respond.ErrorResponse "Некорректный ID вопроса"
// @Failure 401 {object} respond.ErrorResponse "Недействительный токен или пользователь не является автором квиза"
// @Failure 404 {object} respond.ErrorResponse "Вопрос не найден"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /questions/{id} [delete]
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

// ChangeQuestionPosition reorders a question within its quiz.
//
// @Summary Изменить позицию вопроса
// @Description Доступно только автору квиза. Позиция начинается с 1 и не может превышать число вопросов в квизе. Остальные вопросы сдвигаются. Тело запроса должно содержать один JSON-объект без неизвестных полей.
// @Tags questions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID вопроса" minimum(1)
// @Param request body ChangeQuestionPositionRequest true "Новая позиция"
// @Success 204 "Позиция изменена; тело ответа пустое"
// @Failure 400 {object} respond.ErrorResponse "Некорректный JSON, ID вопроса или позиция"
// @Failure 401 {object} respond.ErrorResponse "Недействительный токен или пользователь не является автором квиза"
// @Failure 404 {object} respond.ErrorResponse "Вопрос не найден"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /questions/{id}/position [patch]
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
