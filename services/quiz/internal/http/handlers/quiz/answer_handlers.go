package quizhandler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	// "github.com/kungrem23/quizgo/services/quiz/internal/domain"
	"github.com/kungrem23/quizgo/services/quiz/internal/http/middleware"
	"github.com/kungrem23/quizgo/services/quiz/internal/http/middleware/respond"
)

type AnswerHandler struct {
	service answerService
}

func NewAnswerHandler(service answerService) *AnswerHandler {
	return &AnswerHandler{service: service}
}

type CreateAnswerRequest struct {
	// Текст не должен быть пустым после удаления пробелов по краям.
	TextContent string `json:"text_content" validate:"required" minLength:"1" example:"Париж"`
	IsCorrect   bool   `json:"is_correct" default:"false" example:"true"`
	QuestionId  int    `json:"question_id" validate:"required" minimum:"1" example:"1"`
}

// CreateAnswer adds an answer to a question owned by the authenticated user.
//
// @Summary Создать ответ
// @Description Доступно только автору квиза. Если is_correct не передан, используется false. Тело запроса должно содержать один JSON-объект без неизвестных полей.
// @Tags answers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateAnswerRequest true "Новый ответ"
// @Success 201 "Ответ создан; тело ответа пустое"
// @Failure 400 {object} respond.ErrorResponse "Некорректный JSON, ID вопроса или пустой текст"
// @Failure 401 {object} respond.ErrorResponse "Недействительный токен или пользователь не является автором квиза"
// @Failure 404 {object} respond.ErrorResponse "Вопрос не найден"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /answers [post]
func (h *AnswerHandler) CreateAnswer(w http.ResponseWriter, r *http.Request) {
	userId, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
			Error: "invalid token",
		})
		return
	}
	var req CreateAnswerRequest
	err := decodeJSON(r, &req)
	if err != nil {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "bad request",
		})
		return
	}
	req.TextContent = strings.TrimSpace(req.TextContent)
	if req.TextContent == "" {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "empty text content",
		})
		return
	}
	if req.QuestionId <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "invalid question id",
		})
		return
	}
	err = h.service.CreateAnswerAsAuthor(r.Context(), req.TextContent, req.IsCorrect, req.QuestionId, userId)
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
	w.WriteHeader(http.StatusCreated)
}

// DeleteAnswer deletes an answer owned by the authenticated user.
//
// @Summary Удалить ответ
// @Description Доступно только автору квиза.
// @Tags answers
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID ответа" minimum(1)
// @Success 204 "Ответ удалён; тело ответа пустое"
// @Failure 400 {object} respond.ErrorResponse "Некорректный ID ответа"
// @Failure 401 {object} respond.ErrorResponse "Недействительный токен или пользователь не является автором квиза"
// @Failure 404 {object} respond.ErrorResponse "Ответ не найден"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /answers/{id} [delete]
func (h *AnswerHandler) DeleteAnswer(w http.ResponseWriter, r *http.Request) {
	userId, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
			Error: "invalid token",
		})
		return
	}
	answerIdStr := r.PathValue("id")
	answerId, err := strconv.Atoi(answerIdStr)
	if err != nil || answerId <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "invalid answer id",
		})
		return
	}
	err = h.service.DeleteAnswerAsAuthor(r.Context(), answerId, userId)
	if err != nil {
		if errors.Is(err, middleware.ErrInsufficientRights) {
			respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
				Error: "you cant edit this answer",
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

// GetAnswer returns an answer without exposing whether it is correct.
//
// @Summary Получить ответ
// @Description Публичное представление ответа без признака is_correct.
// @Tags answers
// @Produce json
// @Param id path int true "ID ответа" minimum(1)
// @Success 200 {object} PublicAnswerResponse
// @Failure 400 {object} respond.ErrorResponse "Некорректный ID ответа"
// @Failure 404 {object} respond.ErrorResponse "Ответ не найден"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /answers/{id} [get]
func (h *AnswerHandler) GetAnswer(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "invalid answer id",
		})
		return
	}
	answer, err := h.service.GetAnswer(r.Context(), id)
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
	respond.WriteJSON(w, http.StatusOK, newPublicAnswerResponse(answer))
}

// GetAnswerAsAuthor returns an answer including its correctness to the quiz author.
//
// @Summary Получить ответ для автора
// @Description Возвращает ответ с признаком is_correct. Доступно только автору квиза.
// @Tags answers
// @Produce json
// @Security BearerAuth
// @Param id path int true "ID ответа" minimum(1)
// @Success 200 {object} AuthorAnswerResponse
// @Failure 400 {object} respond.ErrorResponse "Некорректный ID ответа"
// @Failure 401 {object} respond.ErrorResponse "Недействительный токен или пользователь не является автором квиза"
// @Failure 404 {object} respond.ErrorResponse "Ответ не найден"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /answers/{id}/author [get]
func (h *AnswerHandler) GetAnswerAsAuthor(w http.ResponseWriter, r *http.Request) {
	userId, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{Error: "invalid token"})
		return
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{Error: "invalid answer id"})
		return
	}
	answer, err := h.service.GetAnswerAsAuthor(r.Context(), id, userId)
	if err != nil {
		if errors.Is(err, middleware.ErrInsufficientRights) {
			respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{Error: "you cant view this answer"})
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			respond.WriteJSON(w, http.StatusNotFound, respond.ErrorResponse{Error: "not found"})
			return
		}
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{Error: "server error"})
		return
	}
	respond.WriteJSON(w, http.StatusOK, newAuthorAnswerResponse(answer))
}

func (h *AnswerHandler) ListAnswersByQuestionId(w http.ResponseWriter, r *http.Request) {

}
