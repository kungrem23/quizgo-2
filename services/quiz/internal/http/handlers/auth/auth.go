package auth

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/kungrem23/quizgo/services/quiz/internal/domain"
	"github.com/kungrem23/quizgo/services/quiz/internal/http/middleware/respond"
)

type AuthHandler struct {
	service *quiz.Service
}

func NewAuthHandler(service *quiz.Service) *AuthHandler {
	return &AuthHandler{service: service}
}

var loginRegex = regexp.MustCompile(`^[a-zA-Z0-9._\-#$!]{3,40}$`)

var passwordRegex = regexp.MustCompile(`^[a-zA-Z0-9!@#$%^&*()\-_=+\[\]{};:'",.<>\/?\\|~]{3,72}$`)
var newPasswordRegex = regexp.MustCompile(`^[a-zA-Z0-9!@#$%^&*()\-_=+\[\]{};:'",.<>\/?\\|~]{8,72}$`)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r *LoginRequest) ValidateLogin() bool {
	if !loginRegex.MatchString(r.Username) {
		return false
	}
	return true
}

func (r *LoginRequest) ValidatePassword() bool {
	if !passwordRegex.MatchString(r.Password) {
		return false
	}
	return true
}

func (r *LoginRequest) ValidateNewPassword() bool {
	return newPasswordRegex.MatchString(r.Password)
}

type LoginResponse struct {
	Token string `json:"token"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var req LoginRequest
	err := decodeAuthJSON(r, &req)
	if err != nil {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "bad request",
		})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !req.ValidateLogin() {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "validation failed",
			Fields: map[string]string{
				"username": "invalid",
			},
		})
		return
	}
	if !req.ValidatePassword() {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "validation failed",
			Fields: map[string]string{
				"password": "invalid",
			},
		})
		return
	}
	token, err := h.service.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, quiz.ErrInvalidUsername), errors.Is(err, quiz.ErrInvalidPassword):
			respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
				Error: "unauthorized",
			})
			return
		default:
			log.Printf("Generating JWT error: %v", err)
			respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{
				Error: "server error",
			})
			return
		}
	}
	respond.WriteJSON(w, http.StatusOK, LoginResponse{Token: "Bearer " + token})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var req LoginRequest
	err := decodeAuthJSON(r, &req)
	if err != nil {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{Error: "bad request"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !req.ValidateLogin() {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "validation failed",
			Fields: map[string]string{
				"username": "invalid",
			},
		})
		return
	}
	if !req.ValidateNewPassword() {
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
			Error: "validation failed",
			Fields: map[string]string{
				"password": "invalid",
			},
		})
		return
	}
	err = h.service.Register(r.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, quiz.ErrTakenUsername) {
			respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{
				Error: "registration failed",
				Fields: map[string]string{
					"username": "already exists",
				},
			})
			return
		} else {
			respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{
				Error: "registration failed",
			})
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
}

func decodeAuthJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
