package quizhandler

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/kungrem23/quizgo/internal/http/middleware/respond"
)

type UserHandler struct {
	service userService
}

func NewUserHandler(service userService) *UserHandler {
	return &UserHandler{service: service}
}

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
