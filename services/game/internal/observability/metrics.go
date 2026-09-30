package observability

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricNamespace = "quizgo_game"

type Metrics struct {
	roomsActive        prometheus.Gauge
	actorsActive       prometheus.Gauge
	wsActive           prometheus.Gauge
	wsReconnects       *prometheus.CounterVec
	wsCommands         *prometheus.CounterVec
	wsCommandErrors    *prometheus.CounterVec
	wsCommandDuration  *prometheus.HistogramVec
	redisErrors        *prometheus.CounterVec
	roomRecoveries     *prometheus.CounterVec
	ownershipTakeovers prometheus.Counter
}

func NewRegistry() (*prometheus.Registry, *Metrics) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return registry, NewMetrics(registry)
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	metrics := &Metrics{
		roomsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricNamespace, Name: "rooms_active", Help: "Number of room actors currently loaded by this replica.",
		}),
		actorsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricNamespace, Name: "actors_active", Help: "Number of room actor loops currently running on this replica.",
		}),
		wsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: metricNamespace, Name: "websocket_connections_active", Help: "Number of accepted WebSocket connections currently open.",
		}),
		wsReconnects: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricNamespace, Name: "websocket_reconnects_total", Help: "Successful ticket-based WebSocket authentications.",
		}, []string{"role"}),
		wsCommands: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricNamespace, Name: "websocket_commands_total", Help: "WebSocket commands received after authentication.",
		}, []string{"command", "role"}),
		wsCommandErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricNamespace, Name: "websocket_command_errors_total", Help: "WebSocket command errors returned to clients.",
		}, []string{"command", "role", "code"}),
		wsCommandDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: metricNamespace, Name: "websocket_command_duration_seconds", Help: "Time spent validating and dispatching WebSocket commands.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"command", "role"}),
		redisErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricNamespace, Name: "redis_errors_total", Help: "Redis operation errors.",
		}, []string{"operation"}),
		roomRecoveries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricNamespace, Name: "room_recoveries_total", Help: "Room actors restored from persisted Redis state.",
		}, []string{"reason"}),
		ownershipTakeovers: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: metricNamespace, Name: "ownership_takeovers_total", Help: "Room ownership acquisitions after an earlier lease.",
		}),
	}
	registerer.MustRegister(
		metrics.roomsActive, metrics.actorsActive, metrics.wsActive, metrics.wsReconnects,
		metrics.wsCommands, metrics.wsCommandErrors, metrics.wsCommandDuration,
		metrics.redisErrors, metrics.roomRecoveries, metrics.ownershipTakeovers,
	)
	return metrics
}

func Handler(gatherer prometheus.Gatherer) http.Handler {
	return promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func (m *Metrics) RoomOpened() {
	if m != nil {
		m.roomsActive.Inc()
	}
}

func (m *Metrics) RoomClosed() {
	if m != nil {
		m.roomsActive.Dec()
	}
}

func (m *Metrics) ActorStarted() {
	if m != nil {
		m.actorsActive.Inc()
	}
}

func (m *Metrics) ActorStopped() {
	if m != nil {
		m.actorsActive.Dec()
	}
}

func (m *Metrics) WebSocketOpened() {
	if m != nil {
		m.wsActive.Inc()
	}
}

func (m *Metrics) WebSocketClosed() {
	if m != nil {
		m.wsActive.Dec()
	}
}

func (m *Metrics) WebSocketReconnect(role string) {
	if m != nil {
		m.wsReconnects.WithLabelValues(normalizeRole(role)).Inc()
	}
}

func (m *Metrics) WebSocketCommand(command, role string) {
	if m != nil {
		m.wsCommands.WithLabelValues(normalizeCommand(command), normalizeRole(role)).Inc()
	}
}

func (m *Metrics) WebSocketCommandError(command, role, code string) {
	if m != nil {
		m.wsCommandErrors.WithLabelValues(normalizeCommand(command), normalizeRole(role), normalizeErrorCode(code)).Inc()
	}
}

func (m *Metrics) ObserveWebSocketCommand(command, role string, elapsed time.Duration) {
	if m != nil {
		m.wsCommandDuration.WithLabelValues(normalizeCommand(command), normalizeRole(role)).Observe(elapsed.Seconds())
	}
}

func (m *Metrics) RedisError(operation string) {
	if m != nil {
		m.redisErrors.WithLabelValues(normalizeRedisOperation(operation)).Inc()
	}
}

func (m *Metrics) RoomRecovered(reason string) {
	if m != nil {
		m.roomRecoveries.WithLabelValues(normalizeRecoveryReason(reason)).Inc()
	}
}

func (m *Metrics) OwnershipTakeover() {
	if m != nil {
		m.ownershipTakeovers.Inc()
	}
}

func normalizeCommand(value string) string {
	switch value {
	case "start", "answer", "next", "finish", "remove_player", "leave":
		return value
	default:
		return "unknown"
	}
}

func normalizeRole(value string) string {
	switch value {
	case "host", "player":
		return value
	default:
		return "unknown"
	}
}

func normalizeErrorCode(value string) string {
	switch value {
	case "unauthorized", "game_not_found", "nickname_taken", "invalid_payload", "unknown_message_type",
		"rate_limited", "room_owner_changed", "room_not_local", "invalid_phase", "already_answered", "question_closed":
		return value
	default:
		return "internal_error"
	}
}

func normalizeRedisOperation(value string) string {
	switch value {
	case "ping", "save", "update", "get_by_id", "get_by_code", "acquire_room", "lookup_room_owner",
		"renew_room", "release_room", "update_owned":
		return value
	default:
		return "unknown"
	}
}

func normalizeRecoveryReason(value string) string {
	switch value {
	case "ownership_takeover", "persisted_state":
		return value
	default:
		return "unknown"
	}
}
