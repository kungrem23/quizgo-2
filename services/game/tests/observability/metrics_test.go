package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kungrem23/quizgo/services/game/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsExposeBoundedLabels(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := observability.NewMetrics(registry)
	metrics.RoomOpened()
	metrics.ActorStarted()
	metrics.WebSocketOpened()
	metrics.WebSocketReconnect("player")
	metrics.WebSocketCommand("answer", "player")
	metrics.WebSocketCommandError("answer", "player", "invalid_phase")
	metrics.ObserveWebSocketCommand("answer", "player", 20*time.Millisecond)
	metrics.RedisError("renew_room")
	metrics.RoomRecovered("ownership_takeover")
	metrics.OwnershipTakeover()

	// Arbitrary values must be collapsed instead of becoming cardinality.
	metrics.WebSocketCommand("game-id-secret", "player-id-secret")
	metrics.WebSocketCommandError("game-id-secret", "player-id-secret", "room-id-secret")
	metrics.RedisError("redis-key-secret")
	metrics.RoomRecovered("game-id-secret")

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	observability.Handler(registry).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", response.Code)
	}
	body := response.Body.String()
	for _, expected := range []string{
		"quizgo_game_rooms_active 1",
		"quizgo_game_actors_active 1",
		"quizgo_game_websocket_connections_active 1",
		`quizgo_game_websocket_reconnects_total{role="player"} 1`,
		`quizgo_game_websocket_commands_total{command="answer",role="player"} 1`,
		`quizgo_game_websocket_command_errors_total{code="invalid_phase",command="answer",role="player"} 1`,
		`quizgo_game_websocket_command_duration_seconds_count{command="answer",role="player"} 1`,
		`quizgo_game_redis_errors_total{operation="renew_room"} 1`,
		`quizgo_game_room_recoveries_total{reason="ownership_takeover"} 1`,
		"quizgo_game_ownership_takeovers_total 1",
		`quizgo_game_websocket_commands_total{command="unknown",role="unknown"} 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics do not contain %q:\n%s", expected, body)
		}
	}
	for _, forbidden := range []string{"game-id-secret", "player-id-secret", "room-id-secret", "redis-key-secret"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("high-cardinality value %q leaked into metric labels", forbidden)
		}
	}
}
