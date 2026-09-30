package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/kungrem23/quizgo/services/game/internal/application"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
	"github.com/kungrem23/quizgo/services/game/internal/observability"
)

const (
	defaultMaxMessageBytes     = 16 << 10
	defaultHandshakeTimeout    = 10 * time.Second
	defaultCommandTimeout      = 5 * time.Second
	defaultWriteTimeout        = 5 * time.Second
	defaultPingInterval        = 30 * time.Second
	defaultPongTimeout         = 10 * time.Second
	defaultPlayerCommandRate   = 5
	defaultPlayerCommandBurst  = 10
	defaultHostCommandRate     = 10
	defaultHostCommandBurst    = 20
	defaultInvalidMessageLimit = 5
)

var (
	errInvalidPayload     = errors.New("invalid payload")
	errUnknownMessageType = errors.New("unknown message type")
	errRateLimited        = errors.New("command rate limit exceeded")
)

type Handler struct {
	hub                 *application.Hub
	logger              *slog.Logger
	metrics             *observability.Metrics
	maxMessageBytes     int64
	handshakeTimeout    time.Duration
	commandTimeout      time.Duration
	writeTimeout        time.Duration
	pingInterval        time.Duration
	pongTimeout         time.Duration
	playerCommandRate   int
	playerCommandBurst  int
	hostCommandRate     int
	hostCommandBurst    int
	invalidMessageLimit int
}

// Options configures WebSocket resource, rate and liveness limits. Values that
// are zero or negative use the package defaults.
type Options struct {
	MaxMessageBytes     int64
	HandshakeTimeout    time.Duration
	CommandTimeout      time.Duration
	WriteTimeout        time.Duration
	PingInterval        time.Duration
	PongTimeout         time.Duration
	PlayerCommandRate   int
	PlayerCommandBurst  int
	HostCommandRate     int
	HostCommandBurst    int
	InvalidMessageLimit int
	Metrics             *observability.Metrics
}

func New(hub *application.Hub, logger *slog.Logger) *Handler {
	return NewWithOptions(hub, logger, Options{})
}

// NewWithOptions creates a handler with custom WebSocket safety limits.
func NewWithOptions(hub *application.Hub, logger *slog.Logger, options Options) *Handler {
	if options.MaxMessageBytes <= 0 {
		options.MaxMessageBytes = defaultMaxMessageBytes
	}
	if options.HandshakeTimeout <= 0 {
		options.HandshakeTimeout = defaultHandshakeTimeout
	}
	if options.CommandTimeout <= 0 {
		options.CommandTimeout = defaultCommandTimeout
	}
	if options.WriteTimeout <= 0 {
		options.WriteTimeout = defaultWriteTimeout
	}
	if options.PingInterval <= 0 {
		options.PingInterval = defaultPingInterval
	}
	if options.PongTimeout <= 0 {
		options.PongTimeout = defaultPongTimeout
	}
	if options.PlayerCommandRate <= 0 {
		options.PlayerCommandRate = defaultPlayerCommandRate
	}
	if options.PlayerCommandBurst <= 0 {
		options.PlayerCommandBurst = defaultPlayerCommandBurst
	}
	if options.HostCommandRate <= 0 {
		options.HostCommandRate = defaultHostCommandRate
	}
	if options.HostCommandBurst <= 0 {
		options.HostCommandBurst = defaultHostCommandBurst
	}
	if options.InvalidMessageLimit <= 0 {
		options.InvalidMessageLimit = defaultInvalidMessageLimit
	}
	return &Handler{
		hub: hub, logger: logger, metrics: options.Metrics,
		maxMessageBytes:  options.MaxMessageBytes,
		handshakeTimeout: options.HandshakeTimeout,
		commandTimeout:   options.CommandTimeout,
		writeTimeout:     options.WriteTimeout,
		pingInterval:     options.PingInterval, pongTimeout: options.PongTimeout,
		playerCommandRate: options.PlayerCommandRate, playerCommandBurst: options.PlayerCommandBurst,
		hostCommandRate: options.HostCommandRate, hostCommandBurst: options.HostCommandBurst,
		invalidMessageLimit: options.InvalidMessageLimit,
	}
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if h == nil || h.hub == nil {
		http.Error(writer, "realtime service unavailable", http.StatusServiceUnavailable)
		return
	}
	connectionID := websocketConnectionID(request)
	connectionLogger := h.logger
	if connectionLogger != nil {
		connectionLogger = connectionLogger.With("component", "websocket", "connection_id", connectionID)
	}
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		if connectionLogger != nil {
			connectionLogger.WarnContext(request.Context(), "websocket_accept_failed", "error", err)
		}
		return
	}
	connectedAt := time.Now()
	if h.metrics != nil {
		h.metrics.WebSocketOpened()
		defer h.metrics.WebSocketClosed()
	}
	connection.SetReadLimit(h.maxMessageBytes)
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()

	session, err := h.handshake(ctx, connection)
	if err != nil {
		if connectionLogger != nil {
			level := slog.LevelWarn
			if wsErrorCode(err) == "internal_error" {
				level = slog.LevelError
			}
			connectionLogger.Log(ctx, level, "websocket_handshake_failed", "code", wsErrorCode(err), "error", err)
		}
		h.writeError(ctx, connection, "", err)
		status := websocket.StatusPolicyViolation
		reason := "handshake failed"
		if _, ok := application.IsNotRoomOwner(err); ok || errors.Is(err, application.ErrLeaseLost) {
			status = websocket.StatusTryAgainLater
			reason = "room owner changed"
		}
		_ = connection.Close(status, reason)
		return
	}
	if connectionLogger != nil {
		connectionLogger = connectionLogger.With(
			"game_id", session.GameID, "participant_id", session.ParticipantID, "role", session.Role,
		)
		connectionLogger.InfoContext(ctx, "websocket_connected")
		defer func() {
			connectionLogger.Info("websocket_disconnected", "duration_ms", time.Since(connectedAt).Milliseconds())
		}()
	}

	var workers sync.WaitGroup
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			cancel()
			_ = connection.CloseNow()
		})
	}
	defer func() {
		stop()
		workers.Wait()
		h.hub.Disconnect(session)
	}()

	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-session.Events:
				if !ok {
					stop()
					return
				}
				if err := h.writeMessage(ctx, connection, ServerMessage{Type: event.Type, Sequence: event.Sequence, Payload: event.Payload}); err != nil {
					stop()
					return
				}
			}
		}
	}()

	workers.Add(1)
	go func() {
		defer workers.Done()
		select {
		case <-ctx.Done():
		case <-session.Replaced():
			stop()
		}
	}()

	workers.Add(1)
	go func() {
		defer workers.Done()
		if err := heartbeat(ctx, connection, h.pingInterval, h.pongTimeout); err != nil {
			stop()
		}
	}()

	guard := newCommandGuard(session.Role, h)
	for {
		message, err := readClientMessage(ctx, connection)
		if err != nil {
			if !isClientViolation(err) {
				return
			}
			h.observeCommandError(ctx, connectionLogger, session, message, err)
			if h.rejectClientMessage(ctx, connection, guard, message.RequestID, err) {
				return
			}
			continue
		}
		if h.metrics != nil {
			h.metrics.WebSocketCommand(message.Type, string(session.Role))
		}
		if !guard.allow(time.Now()) {
			h.observeCommandError(ctx, connectionLogger, session, message, errRateLimited)
			if h.rejectClientMessage(ctx, connection, guard, message.RequestID, errRateLimited) {
				return
			}
			continue
		}
		startedAt := time.Now()
		commandCtx, commandCancel := context.WithTimeout(ctx, h.commandTimeout)
		result, err := h.dispatch(commandCtx, session, message)
		commandCancel()
		if h.metrics != nil {
			h.metrics.ObserveWebSocketCommand(message.Type, string(session.Role), time.Since(startedAt))
		}
		if err != nil {
			h.observeCommandError(ctx, connectionLogger, session, message, err)
			if h.rejectClientMessage(ctx, connection, guard, message.RequestID, err) {
				return
			}
			continue
		}
		if connectionLogger != nil {
			connectionLogger.DebugContext(ctx, "websocket_command_accepted",
				"command", message.Type, "request_id", message.RequestID, "duplicate", result.Duplicate,
				"duration_ms", time.Since(startedAt).Milliseconds(),
			)
		}
		if err := h.writeMessage(ctx, connection, ServerMessage{Type: "command_accepted", RequestID: message.RequestID, Payload: map[string]bool{"duplicate": result.Duplicate}}); err != nil {
			return
		}
		guard.success()
		if message.Type == "leave" {
			_ = connection.Close(websocket.StatusNormalClosure, "connection closed")
			return
		}
	}
}

func (h *Handler) handshake(ctx context.Context, connection *websocket.Conn) (*application.Session, error) {
	readCtx, cancel := context.WithTimeout(ctx, h.handshakeTimeout)
	defer cancel()
	message, err := readClientMessage(readCtx, connection)
	if err != nil {
		if isClientViolation(err) {
			return nil, err
		}
		return nil, game.ErrUnauthorized
	}
	switch message.Type {
	case "join":
		var payload struct {
			Code     string `json:"code"`
			Nickname string `json:"nickname"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		normalizedCode := strings.ToUpper(strings.TrimSpace(payload.Code))
		expected := roomExpectedByContext(ctx)
		if normalizedCode == "" || len(payload.Code) > 32 || len([]rune(strings.TrimSpace(payload.Nickname))) < 1 || len([]rune(strings.TrimSpace(payload.Nickname))) > 30 ||
			(expected.code != "" && normalizedCode != expected.code) || (expected.gameID != "" && expected.code == "") {
			return nil, errInvalidPayload
		}
		joined, err := h.hub.Join(ctx, payload.Code, payload.Nickname)
		if err != nil {
			return nil, err
		}
		if err := h.writeMessage(ctx, connection, ServerMessage{Type: "joined", RequestID: message.RequestID, Payload: map[string]any{
			"game_id": joined.Session.GameID, "player_id": joined.Player.ID,
			"nickname": joined.Player.Nickname, "ticket": joined.Ticket,
		}}); err != nil {
			h.hub.Disconnect(joined.Session)
			return nil, err
		}
		return joined.Session, nil
	case "host_auth":
		var payload struct {
			GameID        string `json:"game_id"`
			ParticipantID string `json:"participant_id"`
			Ticket        string `json:"ticket"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		if !validCredentialField(payload.GameID, 128) || !validCredentialField(payload.Ticket, 512) {
			return nil, errInvalidPayload
		}
		if expected := roomExpectedByContext(ctx); expected.gameID != "" && payload.GameID != expected.gameID {
			return nil, errInvalidPayload
		}
		session, err := h.hub.AuthenticateHost(ctx, payload.GameID, payload.Ticket)
		if err != nil {
			return nil, err
		}
		if err := h.writeAuthenticated(ctx, connection, message.RequestID, session); err != nil {
			h.hub.Disconnect(session)
			return nil, err
		}
		if h.metrics != nil {
			h.metrics.WebSocketReconnect(string(application.RoleHost))
		}
		return session, nil
	case "player_auth":
		var payload struct {
			GameID        string `json:"game_id"`
			ParticipantID string `json:"participant_id"`
			Ticket        string `json:"ticket"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		if !validCredentialField(payload.GameID, 128) || !validCredentialField(payload.ParticipantID, 128) || !validCredentialField(payload.Ticket, 512) {
			return nil, errInvalidPayload
		}
		if expected := roomExpectedByContext(ctx); expected.gameID != "" && payload.GameID != expected.gameID {
			return nil, errInvalidPayload
		}
		session, err := h.hub.AuthenticatePlayer(ctx, payload.GameID, payload.ParticipantID, payload.Ticket)
		if err != nil {
			return nil, err
		}
		if err := h.writeAuthenticated(ctx, connection, message.RequestID, session); err != nil {
			h.hub.Disconnect(session)
			return nil, err
		}
		if h.metrics != nil {
			h.metrics.WebSocketReconnect(string(application.RolePlayer))
		}
		return session, nil
	default:
		if isKnownMessageType(message.Type) {
			return nil, game.ErrUnauthorized
		}
		return nil, errUnknownMessageType
	}
}

func (h *Handler) writeAuthenticated(ctx context.Context, connection *websocket.Conn, requestID string, session *application.Session) error {
	return h.writeMessage(ctx, connection, ServerMessage{Type: "authenticated", RequestID: requestID, Payload: map[string]any{
		"game_id": session.GameID, "participant_id": session.ParticipantID, "role": session.Role,
	}})
}

func (h *Handler) dispatch(ctx context.Context, session *application.Session, message ClientMessage) (application.CommandResult, error) {
	if !isCommandType(message.Type) {
		return application.CommandResult{}, errUnknownMessageType
	}
	if !commandAllowed(session.Role, message.Type) {
		return application.CommandResult{}, game.ErrUnauthorized
	}
	switch message.Type {
	case "start", "next", "finish", "leave":
		if err := validateEmptyPayload(message.Payload); err != nil {
			return application.CommandResult{}, err
		}
		return h.hub.DispatchCommand(ctx, session, application.Command{Type: message.Type, RequestID: message.RequestID})
	case "remove_player":
		var payload struct {
			PlayerID string `json:"player_id"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil || !validCredentialField(payload.PlayerID, 128) {
			return application.CommandResult{}, game.ErrInvalidPlayer
		}
		return h.hub.DispatchCommand(ctx, session, application.Command{Type: "remove_player", RequestID: message.RequestID, PlayerID: payload.PlayerID})
	case "answer":
		var payload struct {
			AnswerID int64 `json:"answer_id"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil || payload.AnswerID < 1 {
			return application.CommandResult{}, game.ErrInvalidAnswer
		}
		return h.hub.DispatchCommand(ctx, session, application.Command{Type: "answer", RequestID: message.RequestID, AnswerID: payload.AnswerID})
	default:
		return application.CommandResult{}, errUnknownMessageType
	}
}

func (h *Handler) rejectClientMessage(ctx context.Context, connection *websocket.Conn, guard *commandGuard, requestID string, err error) bool {
	h.writeError(ctx, connection, requestID, err)
	if !isClientViolation(err) || !guard.violation() {
		return false
	}
	_ = connection.Close(websocket.StatusPolicyViolation, "too many invalid messages")
	return true
}

func (h *Handler) observeCommandError(ctx context.Context, logger *slog.Logger, session *application.Session, message ClientMessage, err error) {
	code := wsErrorCode(err)
	role := "unknown"
	if session != nil {
		role = string(session.Role)
	}
	if h.metrics != nil {
		h.metrics.WebSocketCommandError(message.Type, role, code)
	}
	if logger != nil {
		level := slog.LevelWarn
		if code == "internal_error" {
			level = slog.LevelError
		}
		logger.Log(ctx, level, "websocket_command_rejected",
			"command", message.Type, "request_id", message.RequestID, "code", code, "error", err,
		)
	}
}

func wsErrorCode(err error) string {
	code := "internal_error"
	switch {
	case errors.Is(err, game.ErrUnauthorized):
		code = "unauthorized"
	case errors.Is(err, game.ErrGameNotFound):
		code = "game_not_found"
	case errors.Is(err, game.ErrNicknameTaken):
		code = "nickname_taken"
	case errors.Is(err, game.ErrInvalidPlayer), errors.Is(err, game.ErrInvalidAnswer), errors.Is(err, game.ErrInvalidRequestID):
		code = "invalid_payload"
	case errors.Is(err, errInvalidPayload):
		code = "invalid_payload"
	case errors.Is(err, errUnknownMessageType):
		code = "unknown_message_type"
	case errors.Is(err, errRateLimited):
		code = "rate_limited"
	case errors.Is(err, application.ErrLeaseLost):
		code = "room_owner_changed"
	case errors.Is(err, game.ErrInvalidPhase):
		code = "invalid_phase"
	case errors.Is(err, game.ErrAlreadyAnswered):
		code = "already_answered"
	case errors.Is(err, game.ErrQuestionClosed):
		code = "question_closed"
	default:
		if _, ok := application.IsNotRoomOwner(err); ok {
			code = "room_not_local"
		}
	}
	return code
}

func (h *Handler) writeError(ctx context.Context, connection *websocket.Conn, requestID string, err error) {
	code := wsErrorCode(err)
	_ = h.writeMessage(ctx, connection, ServerMessage{Type: "error", RequestID: requestID, Payload: map[string]string{"code": code}})
}

func websocketConnectionID(request *http.Request) string {
	if request != nil {
		if value := request.Header.Get(internalCorrelationHeader); value != "" {
			if _, err := uuid.Parse(value); err == nil {
				return value
			}
		}
	}
	return uuid.NewString()
}

func decodePayload(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return fmt.Errorf("%w: payload is required", errInvalidPayload)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %v", errInvalidPayload, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%w: payload must contain exactly one JSON value", errInvalidPayload)
	}
	return nil
}

func (h *Handler) writeMessage(ctx context.Context, connection *websocket.Conn, message ServerMessage) error {
	writeCtx, cancel := context.WithTimeout(ctx, h.writeTimeout)
	defer cancel()
	return wsjson.Write(writeCtx, connection, message)
}

func heartbeat(ctx context.Context, connection *websocket.Conn, pingInterval, pongTimeout time.Duration) error {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, pongTimeout)
			err := connection.Ping(pingCtx)
			cancel()
			if err != nil {
				return err
			}
		}
	}
}
