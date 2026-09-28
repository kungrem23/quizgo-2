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

type joinedPayload struct {
	GameID   string `json:"game_id"`
	PlayerID string `json:"player_id"`
	Ticket   string `json:"ticket"`
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
	var joinedData joinedPayload
	if err := json.Unmarshal(joined.Payload, &joinedData); err != nil {
		t.Fatal(err)
	}
	if joinedData.GameID != created.Game.ID || joinedData.PlayerID == "" || joinedData.Ticket == "" {
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
	assertCommandAck(t, waitForMessage(t, ctx, hostConnection, "command_accepted"), "start-1", false)
	writeClientMessage(t, ctx, hostConnection, "start", "start-1", nil)
	assertCommandAck(t, waitForMessage(t, ctx, hostConnection, "command_accepted"), "start-1", true)
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
	if closedPayload.Phase != game.PhaseFinished || len(closedPayload.CorrectAnswerIDs) != 1 || closedPayload.CorrectAnswerIDs[0] != 1 {
		t.Fatalf("unexpected scoreboard payload: %s", closed.Payload)
	}
	finished := waitForMessage(t, ctx, playerConnection, "game_finished")
	if !strings.Contains(string(finished.Payload), `"phase":"finished"`) {
		t.Fatalf("unexpected finished event: %#v", finished)
	}
}

func TestWebSocketLobbyLifecycleAndAuthorization(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
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

	aliceConnection, alice := joinPlayer(t, ctx, wsURL, created.Game.Code, "Alice")
	defer aliceConnection.Close(websocket.StatusNormalClosure, "test complete")
	bobConnection, bob := joinPlayer(t, ctx, wsURL, created.Game.Code, "Bob")
	defer bobConnection.Close(websocket.StatusNormalClosure, "test complete")
	hostConnection, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hostConnection.Close(websocket.StatusNormalClosure, "test complete")
	writeClientMessage(t, ctx, hostConnection, "host_auth", "host-auth", map[string]any{"game_id": created.Game.ID, "ticket": created.HostTicket})
	waitForMessage(t, ctx, hostConnection, "authenticated")

	writeClientMessage(t, ctx, aliceConnection, "remove_player", "forbidden-remove", map[string]any{"player_id": bob.PlayerID})
	waitForError(t, ctx, aliceConnection, "forbidden-remove", "unauthorized")
	writeClientMessage(t, ctx, aliceConnection, "finish", "forbidden-finish", nil)
	waitForError(t, ctx, aliceConnection, "forbidden-finish", "unauthorized")

	writeClientMessage(t, ctx, bobConnection, "leave", "leave-bob", nil)
	waitForMessage(t, ctx, bobConnection, "command_accepted")
	left := waitForMessage(t, ctx, hostConnection, "player_left")
	if !strings.Contains(string(left.Payload), bob.PlayerID) {
		t.Fatalf("unexpected player_left event: %s", left.Payload)
	}
	assertPlayerAuthRejected(t, ctx, wsURL, created.Game.ID, bob)

	writeClientMessage(t, ctx, hostConnection, "remove_player", "remove-alice", map[string]any{"player_id": alice.PlayerID})
	removed := waitForMessage(t, ctx, aliceConnection, "player_removed")
	if !strings.Contains(string(removed.Payload), alice.PlayerID) {
		t.Fatalf("unexpected player_removed event: %s", removed.Payload)
	}
	assertPlayerAuthRejected(t, ctx, wsURL, created.Game.ID, alice)

	writeClientMessage(t, ctx, hostConnection, "finish", "finish-lobby", nil)
	waitForMessage(t, ctx, hostConnection, "game_finished")
	writeClientMessage(t, ctx, hostConnection, "remove_player", "remove-after-finish", map[string]any{"player_id": alice.PlayerID})
	waitForError(t, ctx, hostConnection, "remove-after-finish", "invalid_phase")
}

func TestWebSocketReconnectClosesReplacedConnection(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	oldConnection, joined := joinPlayer(t, ctx, wsURL, created.Game.Code, "Alice")
	defer oldConnection.CloseNow()
	replacement, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close(websocket.StatusNormalClosure, "test complete")
	writeClientMessage(t, ctx, replacement, "player_auth", "reconnect", map[string]any{
		"game_id": created.Game.ID, "participant_id": joined.PlayerID, "ticket": joined.Ticket,
	})
	waitForMessage(t, ctx, replacement, "authenticated")
	state := waitForMessage(t, ctx, replacement, "state")
	if !strings.Contains(string(state.Payload), `"phase":"lobby"`) || !strings.Contains(string(state.Payload), joined.PlayerID) {
		t.Fatalf("unexpected reconnect state: %s", state.Payload)
	}

	readUntilClosed(t, ctx, oldConnection)
	writeClientMessage(t, ctx, replacement, "answer", "still-current", map[string]any{"answer_id": 1})
	waitForError(t, ctx, replacement, "still-current", "invalid_phase")
}

func TestWebSocketReconnectAfterRestartReceivesCaughtUpState(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{
			{ID: 1, Text: "Q1", TimeLimitSeconds: 10, Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}}},
			{ID: 2, Text: "Q2", TimeLimitSeconds: 10, Answers: []game.Answer{{ID: 3, Text: "A", IsCorrect: true}, {ID: 4, Text: "B"}}},
		},
	}
	repository := &repositoryStub{}
	service := application.New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), application.CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}

	initialHub := application.NewHub(repository, time.Hour)
	joined, err := initialHub.Join(context.Background(), created.Game.Code, "Alice")
	if err != nil {
		initialHub.Close()
		t.Fatal(err)
	}
	initialHub.Close()

	recovered, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	startedAt := time.Now().UTC().Add(-time.Minute)
	if err := recovered.Start(startedAt); err != nil {
		t.Fatal(err)
	}
	countdownEndsAt := *recovered.CountdownEndsAt
	expectedClosedAt := countdownEndsAt.Add(10 * time.Second)
	if err := repository.Update(context.Background(), recovered, time.Hour); err != nil {
		t.Fatal(err)
	}

	recoveredHub := application.NewHub(repository, time.Hour)
	defer recoveredHub.Close()
	server := httptest.NewServer(websockettransport.New(recoveredHub, log.New(io.Discard, "", 0)))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connection, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(websocket.StatusNormalClosure, "test complete")
	writeClientMessage(t, ctx, connection, "player_auth", "reconnect-after-restart", map[string]any{
		"game_id": created.Game.ID, "participant_id": joined.Player.ID, "ticket": joined.Ticket,
	})
	waitForMessage(t, ctx, connection, "authenticated")
	stateMessage := waitForMessage(t, ctx, connection, "state")
	var state struct {
		Phase            game.Phase `json:"phase"`
		CountdownEndsAt  *time.Time `json:"countdown_ends_at"`
		QuestionClosesAt *time.Time `json:"question_closes_at"`
		CorrectAnswerIDs []int64    `json:"correct_answer_ids"`
	}
	if err := json.Unmarshal(stateMessage.Payload, &state); err != nil {
		t.Fatal(err)
	}
	if state.Phase != game.PhaseScoreboard || state.CountdownEndsAt != nil || state.QuestionClosesAt != nil || len(state.CorrectAnswerIDs) != 1 || state.CorrectAnswerIDs[0] != 1 {
		t.Fatalf("unexpected recovered websocket state: %s", stateMessage.Payload)
	}
	persisted, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.QuestionOpenedAt == nil || !persisted.QuestionOpenedAt.Equal(countdownEndsAt) || !persisted.LastActivity.Equal(expectedClosedAt) {
		t.Fatalf("recovered websocket state used non-deterministic timestamps: %#v", persisted)
	}
}

func TestWebSocketHeartbeatClosesUnresponsiveConnectionAndAllowsReconnect(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
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
	handler := websockettransport.NewWithOptions(hub, log.New(io.Discard, "", 0), websockettransport.Options{
		PingInterval: 20 * time.Millisecond,
		PongTimeout:  30 * time.Millisecond,
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	unresponsive, joined := joinPlayerWithOptions(t, ctx, wsURL, created.Game.Code, "Alice", &websocket.DialOptions{
		OnPingReceived: func(context.Context, []byte) bool { return false },
	})
	defer unresponsive.CloseNow()
	readUntilClosed(t, ctx, unresponsive)

	reconnected, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reconnected.Close(websocket.StatusNormalClosure, "test complete")
	writeClientMessage(t, ctx, reconnected, "player_auth", "after-heartbeat", map[string]any{
		"game_id": created.Game.ID, "participant_id": joined.PlayerID, "ticket": joined.Ticket,
	})
	waitForMessage(t, ctx, reconnected, "authenticated")
	state := waitForMessage(t, ctx, reconnected, "state")
	if !strings.Contains(string(state.Payload), joined.PlayerID) {
		t.Fatalf("player state was lost after heartbeat disconnect: %s", state.Payload)
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

func joinPlayer(t *testing.T, ctx context.Context, wsURL, code, nickname string) (*websocket.Conn, joinedPayload) {
	t.Helper()
	return joinPlayerWithOptions(t, ctx, wsURL, code, nickname, nil)
}

func joinPlayerWithOptions(t *testing.T, ctx context.Context, wsURL, code, nickname string, options *websocket.DialOptions) (*websocket.Conn, joinedPayload) {
	t.Helper()
	connection, _, err := websocket.Dial(ctx, wsURL, options)
	if err != nil {
		t.Fatal(err)
	}
	writeClientMessage(t, ctx, connection, "join", "join-"+nickname, map[string]any{"code": code, "nickname": nickname})
	message := waitForMessage(t, ctx, connection, "joined")
	var joined joinedPayload
	if err := json.Unmarshal(message.Payload, &joined); err != nil {
		connection.Close(websocket.StatusInternalError, "invalid joined payload")
		t.Fatal(err)
	}
	return connection, joined
}

func assertPlayerAuthRejected(t *testing.T, ctx context.Context, wsURL, gameID string, player joinedPayload) {
	t.Helper()
	connection, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close(websocket.StatusNormalClosure, "test complete")
	writeClientMessage(t, ctx, connection, "player_auth", "reconnect", map[string]any{
		"game_id": gameID, "participant_id": player.PlayerID, "ticket": player.Ticket,
	})
	var message serverMessage
	if err := wsjson.Read(ctx, connection, &message); err != nil {
		if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
			t.Fatalf("rejected reconnect closed with %v", err)
		}
		return
	}
	if message.Type != "error" || !strings.Contains(string(message.Payload), `"code":"unauthorized"`) {
		t.Fatalf("unexpected rejected reconnect response: %#v", message)
	}
}

func waitForError(t *testing.T, ctx context.Context, connection *websocket.Conn, requestID, code string) serverMessage {
	t.Helper()
	for {
		var message serverMessage
		if err := wsjson.Read(ctx, connection, &message); err != nil {
			t.Fatalf("waiting for error %q: %v", code, err)
		}
		if message.Type != "error" || message.RequestID != requestID {
			continue
		}
		var payload struct {
			Code string `json:"code"`
		}
		if err := json.Unmarshal(message.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Code != code {
			t.Fatalf("error code = %q, want %q", payload.Code, code)
		}
		return message
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

func readUntilClosed(t *testing.T, ctx context.Context, connection *websocket.Conn) {
	t.Helper()
	for {
		var message serverMessage
		if err := wsjson.Read(ctx, connection, &message); err != nil {
			return
		}
	}
}

func assertCommandAck(t *testing.T, message serverMessage, requestID string, duplicate bool) {
	t.Helper()
	if message.RequestID != requestID {
		t.Fatalf("ack request_id = %q, want %q", message.RequestID, requestID)
	}
	var payload struct {
		Duplicate bool `json:"duplicate"`
	}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Duplicate != duplicate {
		t.Fatalf("ack duplicate = %t, want %t", payload.Duplicate, duplicate)
	}
}
