package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/kungrem23/quizgo/internal/http/middleware/respond"
)

var ErrInvalidToken error = errors.New("Invalid token")
var ErrInsufficientRights error = errors.New("Insufficient rights")

type contextKey string

const userIDKey contextKey = "user_id"

func UserIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}

type AccessTokenVerifier interface {
	VerifyAccessToken(string) (int, error)
}

func Auth(verifier AccessTokenVerifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || verifier == nil {
			respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
				Error: "invalid token",
			})
			return
		}
		userID, err := verifier.VerifyAccessToken(parts[1])
		if err != nil {
			respond.WriteJSON(w, http.StatusUnauthorized, respond.ErrorResponse{
				Error: "invalid token",
			})
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
