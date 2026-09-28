package ws_test

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/kungrem23/quizgo/services/game/internal/application"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
	websockettransport "github.com/kungrem23/quizgo/services/game/internal/transport/ws"
)

type catalogStub struct{ snapshot game.QuizSnapshot }

func (c catalogStub) GetPlayableQuiz(context.Context, int64, string) (game.QuizSnapshot, error) {
	return c.snapshot, nil
}

type repositoryStub struct {
	mu    sync.Mutex
	value game.Game
}

func (r *repositoryStub) Save(_ context.Context, value game.Game, _ time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.value = value
	return nil
}

func (r *repositoryStub) Update(_ context.Context, value game.Game, _ time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.value = value
	return nil
}

func (r *repositoryStub) GetByID(_ context.Context, id string) (game.Game, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.value.ID != id {
		return game.Game{}, game.ErrGameNotFound
	}
	return r.value, nil
}

func (r *repositoryStub) GetByCode(_ context.Context, code string) (game.Game, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.value.Code != code {
		return game.Game{}, game.ErrGameNotFound
	}
	return r.value, nil
}

type serverMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Sequence  uint64          `json:"sequence"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

func TestWebSocketJoinStartAndAnswer(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 1,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	repository := &repositoryStub{}
	service := application.New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), application.CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	hub := application.NewHub(repository, time.Hour)
	defer hub.Close()
	server := httptest.NewServer(websockettransport.New(hub, log.New(io.Discard, "", 0)))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	playerConnection, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer playerConnection.Close(websocket.StatusNormalClosure, "test complete")
	writeClientMessage(t, ctx, playerConnection, "join", "join-1", map[string]any{"code": created.Game.Code, "nickname": "Alice"})
	joined := waitForMessage(t, ctx, playerConnection, "joined")
	var joinedPayload struct {
		GameID   string `json:"game_id"`
		PlayerID string `json:"player_id"`
		Ticket   string `json:"ticket"`
	}
	if err := json.Unmarshal(joined.Payload, &joinedPayload); err != nil {
		t.Fatal(err)
	}
	if joinedPayload.GameID != created.Game.ID || joinedPayload.PlayerID == "" || joinedPayload.Ticket == "" {
		t.Fatalf("unexpected joined payload: %s", joined.Payload)
	}

	hostConnection, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hostConnection.Close(websocket.StatusNormalClosure, "test complete")
	writeClientMessage(t, ctx, hostConnection, "host_auth", "auth-1", map[string]any{
		"game_id": created.Game.ID, "ticket": created.HostTicket,
	})
	waitForMessage(t, ctx, hostConnection, "authenticated")
	writeClientMessage(t, ctx, hostConnection, "start", "start-1", nil)
	countdown := waitForMessage(t, ctx, playerConnection, "countdown_started")
	if !strings.Contains(string(countdown.Payload), `"phase":"countdown"`) || !strings.Contains(string(countdown.Payload), `"countdown_ends_at"`) || strings.Contains(string(countdown.Payload), `"current_question"`) {
		t.Fatalf("unexpected countdown event: %#v", countdown)
	}
	opened := waitForMessage(t, ctx, playerConnection, "question_opened")
	if opened.Sequence == 0 || strings.Contains(string(opened.Payload), "is_correct") {
		t.Fatalf("unsafe question event: %#v", opened)
	}

	writeClientMessage(t, ctx, playerConnection, "answer", "answer-1", map[string]any{"answer_id": 1})
	accepted := waitForMessage(t, ctx, playerConnection, "answer_accepted")
	if accepted.Sequence <= opened.Sequence {
		t.Fatalf("sequence did not advance: opened=%d accepted=%d", opened.Sequence, accepted.Sequence)
	}
	closed := waitForMessage(t, ctx, playerConnection, "question_closed")
	var closedPayload struct {
		Phase            game.Phase `json:"phase"`
		CorrectAnswerIDs []int64    `json:"correct_answer_ids"`
	}
	if err := json.Unmarshal(closed.Payload, &closedPayload); err != nil {
		t.Fatal(err)
	}
	if closedPayload.Phase != game.PhaseScoreboard || len(closedPayload.CorrectAnswerIDs) != 1 || closedPayload.CorrectAnswerIDs[0] != 1 {
		t.Fatalf("unexpected scoreboard payload: %s", closed.Payload)
	}
	writeClientMessage(t, ctx, hostConnection, "next", "next-1", nil)
	finished := waitForMessage(t, ctx, playerConnection, "game_finished")
	if !strings.Contains(string(finished.Payload), `"phase":"finished"`) {
		t.Fatalf("unexpected finished event: %#v", finished)
	}
}

func writeClientMessage(t *testing.T, ctx context.Context, connection *websocket.Conn, messageType, requestID string, payload any) {
	t.Helper()
	raw := json.RawMessage(nil)
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		raw = encoded
	}
	if err := wsjson.Write(ctx, connection, websockettransport.ClientMessage{Type: messageType, RequestID: requestID, Payload: raw}); err != nil {
		t.Fatal(err)
	}
}

func waitForMessage(t *testing.T, ctx context.Context, connection *websocket.Conn, messageType string) serverMessage {
	t.Helper()
	for {
		var message serverMessage
		if err := wsjson.Read(ctx, connection, &message); err != nil {
			t.Fatalf("waiting for %q: %v", messageType, err)
		}
		if message.Type == "error" {
			t.Fatalf("waiting for %q received error: %s", messageType, message.Payload)
		}
		if message.Type == messageType {
			return message
		}
	}
}
