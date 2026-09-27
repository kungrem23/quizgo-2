package quizhandler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/kungrem23/quizgo/services/quiz/internal/http/middleware/respond"
)

type UserHandler struct {
	service userService
}

func NewUserHandler(service userService) *UserHandler {
	return &UserHandler{service: service}
}

// GetUser returns a user's public profile.
//
// @Summary Получить пользователя
// @Description Возвращает ID и имя пользователя. Хеш пароля не передаётся.
// @Tags users
// @Produce json
// @Param id path int true "ID пользователя" minimum(1)
// @Success 200 {object} UserResponse
// @Failure 400 {object} respond.ErrorResponse "Некорректный ID пользователя"
// @Failure 404 {object} respond.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} respond.ErrorResponse "Ошибка сервера"
// @Router /users/{id} [get]
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "invalid user id",
		})
		return
	}

	user, err := h.service.GetUser(r.Context(), id)
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

	respond.WriteJSON(w, http.StatusOK, UserResponse{
		ID:       user.Id,
		Username: user.Username,
	})
}
