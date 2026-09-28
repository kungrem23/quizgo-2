package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/kungrem23/quizgo/services/game/internal/application"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GameService interface {
	CreateGame(context.Context, application.CreateGameRequest) (application.CreatedGame, error)
}

type Dependency struct {
	Name  string
	Check func(context.Context) error
}

func New(service GameService, realtime http.Handler, dependencies ...Dependency) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(writer http.ResponseWriter, request *http.Request) {
		for _, dependency := range dependencies {
			if dependency.Check == nil || dependency.Check(request.Context()) != nil {
				writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "dependency": dependency.Name})
				return
			}
		}
		writeJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /api/games", func(writer http.ResponseWriter, request *http.Request) {
		handleCreateGame(writer, request, service)
	})
	if realtime == nil {
		mux.HandleFunc("GET /ws", func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "websocket transport is unavailable"})
		})
	} else {
		mux.Handle("GET /ws", realtime)
	}
	return mux
}

func handleCreateGame(writer http.ResponseWriter, request *http.Request, service GameService) {
	if service == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "game service is unavailable"})
		return
	}
	token, ok := bearerToken(request.Header.Get("Authorization"))
	if !ok {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "bearer token is required"})
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	var body struct {
		QuizID int64 `json:"quiz_id"`
	}
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || decoder.Decode(&struct{}{}) != io.EOF || body.QuizID < 1 {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	created, err := service.CreateGame(request.Context(), application.CreateGameRequest{QuizID: body.QuizID, AccessToken: token})
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeJSON(writer, http.StatusCreated, map[string]any{
		"id": created.Game.ID, "code": created.Game.Code, "phase": created.Game.Phase,
		"quiz_revision": created.Game.Quiz.Revision, "host_ticket": created.HostTicket,
	})
}

func bearerToken(value string) (string, bool) {
	parts := strings.Fields(value)
	returnValue := ""
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		returnValue = parts[1]
	}
	return returnValue, returnValue != ""
}

func writeApplicationError(writer http.ResponseWriter, err error) {
	code := status.Code(err)
	httpStatus := http.StatusInternalServerError
	switch code {
	case codes.InvalidArgument:
		httpStatus = http.StatusBadRequest
	case codes.Unauthenticated:
		httpStatus = http.StatusUnauthorized
	case codes.PermissionDenied:
		httpStatus = http.StatusForbidden
	case codes.NotFound:
		httpStatus = http.StatusNotFound
	case codes.FailedPrecondition, codes.AlreadyExists:
		httpStatus = http.StatusConflict
	case codes.DeadlineExceeded, codes.Unavailable:
		httpStatus = http.StatusServiceUnavailable
	default:
		if errors.Is(err, context.DeadlineExceeded) {
			httpStatus = http.StatusServiceUnavailable
		}
	}
	writeJSON(writer, httpStatus, map[string]string{"error": http.StatusText(httpStatus)})
}

func writeJSON(writer http.ResponseWriter, statusCode int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(value)
}
