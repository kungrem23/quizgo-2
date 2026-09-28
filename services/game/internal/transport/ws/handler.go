package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/kungrem23/quizgo/services/game/internal/application"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
)

type Handler struct {
	hub    *application.Hub
	logger *log.Logger
}

func New(hub *application.Hub, logger *log.Logger) *Handler {
	return &Handler{hub: hub, logger: logger}
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if h == nil || h.hub == nil {
		http.Error(writer, "realtime service unavailable", http.StatusServiceUnavailable)
		return
	}
	connection, err := websocket.Accept(writer, request, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	connection.SetReadLimit(64 << 10)
	ctx, cancel := context.WithCancel(request.Context())
	defer cancel()
	defer connection.Close(websocket.StatusNormalClosure, "connection closed")

	session, err := h.handshake(ctx, connection)
	if err != nil {
		h.writeError(ctx, connection, "", err)
		_ = connection.Close(websocket.StatusPolicyViolation, "handshake failed")
		return
	}
	defer h.hub.Disconnect(session)

	go func() {
		defer cancel()
		for event := range session.Events {
			if err := writeMessage(ctx, connection, ServerMessage{Type: event.Type, Sequence: event.Sequence, Payload: event.Payload}); err != nil {
				return
			}
		}
	}()

	for {
		var message ClientMessage
		if err := wsjson.Read(ctx, connection, &message); err != nil {
			return
		}
		if err := h.dispatch(ctx, session, message); err != nil {
			h.writeError(ctx, connection, message.RequestID, err)
			continue
		}
		if err := writeMessage(ctx, connection, ServerMessage{Type: "command_accepted", RequestID: message.RequestID}); err != nil {
			return
		}
	}
}

func (h *Handler) handshake(ctx context.Context, connection *websocket.Conn) (*application.Session, error) {
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var message ClientMessage
	if err := wsjson.Read(readCtx, connection, &message); err != nil {
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
		joined, err := h.hub.Join(ctx, payload.Code, payload.Nickname)
		if err != nil {
			return nil, err
		}
		if err := writeMessage(ctx, connection, ServerMessage{Type: "joined", RequestID: message.RequestID, Payload: map[string]any{
			"game_id": joined.Session.GameID, "player_id": joined.Player.ID,
			"nickname": joined.Player.Nickname, "ticket": joined.Ticket,
		}}); err != nil {
			h.hub.Disconnect(joined.Session)
			return nil, err
		}
		return joined.Session, nil
	case "host_auth", "player_auth":
		var payload struct {
			GameID        string `json:"game_id"`
			ParticipantID string `json:"participant_id"`
			Ticket        string `json:"ticket"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		var session *application.Session
		var err error
		if message.Type == "host_auth" {
			session, err = h.hub.AuthenticateHost(ctx, payload.GameID, payload.Ticket)
		} else {
			session, err = h.hub.AuthenticatePlayer(ctx, payload.GameID, payload.ParticipantID, payload.Ticket)
		}
		if err != nil {
			return nil, err
		}
		if err := writeMessage(ctx, connection, ServerMessage{Type: "authenticated", RequestID: message.RequestID, Payload: map[string]any{
			"game_id": session.GameID, "participant_id": session.ParticipantID, "role": session.Role,
		}}); err != nil {
			h.hub.Disconnect(session)
			return nil, err
		}
		return session, nil
	default:
		return nil, game.ErrUnauthorized
	}
}

func (h *Handler) dispatch(ctx context.Context, session *application.Session, message ClientMessage) error {
	switch message.Type {
	case "start", "next":
		return h.hub.Dispatch(ctx, session, message.Type, 0)
	case "answer":
		var payload struct {
			AnswerID int64 `json:"answer_id"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil || payload.AnswerID < 1 {
			return game.ErrInvalidAnswer
		}
		return h.hub.Dispatch(ctx, session, "answer", payload.AnswerID)
	default:
		return errors.New("unknown command")
	}
}

func (h *Handler) writeError(ctx context.Context, connection *websocket.Conn, requestID string, err error) {
	code := "internal_error"
	switch {
	case errors.Is(err, game.ErrUnauthorized):
		code = "unauthorized"
	case errors.Is(err, game.ErrGameNotFound):
		code = "game_not_found"
	case errors.Is(err, game.ErrNicknameTaken):
		code = "nickname_taken"
	case errors.Is(err, game.ErrInvalidPlayer), errors.Is(err, game.ErrInvalidAnswer):
		code = "invalid_payload"
	case errors.Is(err, game.ErrInvalidPhase):
		code = "invalid_phase"
	case errors.Is(err, game.ErrAlreadyAnswered):
		code = "already_answered"
	case errors.Is(err, game.ErrQuestionClosed):
		code = "question_closed"
	}
	if code == "internal_error" && h.logger != nil {
		h.logger.Printf("websocket command failed: %v", err)
	}
	_ = writeMessage(ctx, connection, ServerMessage{Type: "error", RequestID: requestID, Payload: map[string]string{"code": code}})
}

func decodePayload(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return errors.New("payload is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("payload must contain exactly one JSON value")
	}
	return nil
}

func writeMessage(ctx context.Context, connection *websocket.Conn, message ServerMessage) error {
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return wsjson.Write(writeCtx, connection, message)
}
