package httptransport_test

import . "github.com/kungrem23/quizgo/services/game/internal/transport/http"

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kungrem23/quizgo/services/game/internal/application"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
)

type serviceStub struct{ request application.CreateGameRequest }

func (s *serviceStub) CreateGame(_ context.Context, request application.CreateGameRequest) (application.CreatedGame, error) {
	s.request = request
	return application.CreatedGame{Game: game.Game{ID: "game-1", Code: "ABC123", Phase: game.PhaseLobby, Quiz: game.QuizSnapshot{Revision: 4}}, HostTicket: "host-ticket"}, nil
}

func TestCreateGame(t *testing.T) {
	service := &serviceStub{}
	request := httptest.NewRequest(http.MethodPost, "/api/games", strings.NewReader(`{"quiz_id":7}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	New(service, nil).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.request.QuizID != 7 || service.request.AccessToken != "access-token" || !strings.Contains(response.Body.String(), `"host_ticket":"host-ticket"`) {
		t.Fatalf("status=%d request=%#v body=%s", response.Code, service.request, response.Body.String())
	}
}

func TestWebSocketRouteIsUnavailableWithoutHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/ws", nil)
	response := httptest.NewRecorder()
	New(nil, nil).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestReadinessReportsFailedDependency(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	New(nil, nil, Dependency{Name: "redis", Check: func(context.Context) error { return context.DeadlineExceeded }}).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "redis") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestMetricsEndpoint(t *testing.T) {
	metrics := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("test_metric 1\n"))
	})
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	NewWithMetrics(nil, nil, metrics).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "test_metric 1") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
