package auth

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/kungrem23/quizgo/services/quiz/internal/authn"
	"github.com/kungrem23/quizgo/services/quiz/internal/domain"
	"github.com/kungrem23/quizgo/services/quiz/internal/http/middleware/respond"
)

type AuthHandler struct {
	service      *quiz.Service
	cookieSecure bool
}

type CookieConfig struct {
	Secure bool
}

const refreshCookieName = "quizgo_refresh"

func NewAuthHandler(service *quiz.Service, configs ...CookieConfig) *AuthHandler {
	config := CookieConfig{Secure: true}
	if len(configs) > 0 {
		config = configs[0]
	}
	return &AuthHandler{service: service, cookieSecure: config.Secure}
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
	session, err := h.service.LoginSession(r.Context(), req.Username, req.Password)
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
	if session.RefreshToken != "" {
		h.setRefreshCookie(w, session.RefreshToken, session.RefreshExpiresAt)
	}
	w.Header().Set("Cache-Control", "no-store")
	respond.WriteJSON(w, http.StatusOK, LoginResponse{Token: "Bearer " + session.AccessToken})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !sameOrigin(r) {
		respond.WriteJSON(w, http.StatusForbidden, respond.ErrorResponse{Error: "forbidden"})
		return
	}
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || cookie.Value == "" || h.service == nil {
		h.clearRefreshCookie(w)
		respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{Error: "invalid refresh token"})
		return
	}
	session, err := h.service.RefreshSession(r.Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, authn.ErrInvalidRefreshToken) {
			h.clearRefreshCookie(w)
			respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{Error: "invalid refresh token"})
			return
		}
		log.Printf("Rotating refresh token error: %v", err)
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{Error: "server error"})
		return
	}
	h.setRefreshCookie(w, session.RefreshToken, session.RefreshExpiresAt)
	respond.WriteJSON(w, http.StatusOK, LoginResponse{Token: "Bearer " + session.AccessToken})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !sameOrigin(r) {
		respond.WriteJSON(w, http.StatusForbidden, respond.ErrorResponse{Error: "forbidden"})
		return
	}
	cookie, cookieErr := r.Cookie(refreshCookieName)
	h.clearRefreshCookie(w)
	if cookieErr == nil && cookie.Value != "" && h.service != nil {
		if err := h.service.LogoutSession(r.Context(), cookie.Value); err != nil &&
			!errors.Is(err, authn.ErrInvalidRefreshToken) {
			log.Printf("Revoking refresh token error: %v", err)
			respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{Error: "server error"})
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: token, Path: "/auth", Expires: expiresAt,
		MaxAge: maxAge, HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: "", Path: "/auth", Expires: time.Unix(0, 0),
		MaxAge: -1, HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
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
