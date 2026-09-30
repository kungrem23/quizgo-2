package redis_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kungrem23/quizgo/services/game/internal/observability"
	redisstore "github.com/kungrem23/quizgo/services/game/internal/store/redis"
	"github.com/prometheus/client_golang/prometheus"
)

func TestRedisErrorsAreReported(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := observability.NewMetrics(registry)
	store := redisstore.New(redisstore.Config{Address: "127.0.0.1:1", Metrics: metrics})
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := store.Ping(ctx); err == nil {
		t.Fatal("expected Redis ping to fail")
	}

	response := httptest.NewRecorder()
	observability.Handler(registry).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if body := response.Body.String(); !strings.Contains(body, `quizgo_game_redis_errors_total{operation="ping"} 1`) {
		t.Fatalf("Redis error metric is missing:\n%s", body)
	}
}
