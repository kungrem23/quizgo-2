package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/kungrem23/quizgo/services/game/tests/systemtest"
)

type serverMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Sequence  uint64          `json:"sequence"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type peer struct {
	connection *websocket.Conn
}

type playerCredentials struct {
	GameID   string `json:"game_id"`
	PlayerID string `json:"player_id"`
	Ticket   string `json:"ticket"`
}

type statePayload struct {
	Phase            string  `json:"phase"`
	CorrectAnswerIDs []int64 `json:"correct_answer_ids"`
	Players          []struct {
		ID       string `json:"id"`
		Nickname string `json:"nickname"`
		Score    int64  `json:"score"`
	} `json:"players"`
}

func TestSystemGameLifecycle(t *testing.T) {
	httpURL := os.Getenv("QUIZGO_E2E_HTTP_URL")
	gameURL := os.Getenv("QUIZGO_E2E_GAME_URL")
	if httpURL == "" || gameURL == "" {
		t.Skip("set QUIZGO_E2E_HTTP_URL and QUIZGO_E2E_GAME_URL, or run scripts/e2e.sh")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	waitForReady(t, ctx, gameURL)

	api := systemtest.Client{BaseURL: httpURL, HTTP: &http.Client{Timeout: 10 * time.Second}}
	fixture, err := api.ProvisionQuiz(ctx, 2, 5)
	if err != nil {
		t.Fatal(err)
	}
	created, err := api.CreateGame(ctx, fixture.Authorization, fixture.QuizID)
	if err != nil {
		t.Fatal(err)
	}
	wsURL := websocketURL(gameURL)

	host := authenticateHost(t, ctx, wsURL, created)
	players := make([]*peer, 3)
	credentials := make([]playerCredentials, len(players))
	defer func() {
		host.close()
		for _, player := range players {
			player.close()
		}
	}()
	for index := range players {
		players[index], credentials[index] = joinPlayer(t, ctx, wsURL, created, fmt.Sprintf("Player-%d", index+1))
	}

	// Role authorization is checked on the real WebSocket boundary.
	players[0].write(t, ctx, "start", "player-cannot-start", nil)
	assertErrorCode(t, players[0].wait(t, ctx, "error"), "player-cannot-start", "unauthorized")

	// Ticket-based reconnect replaces the old connection and restores state.
	players[0].close()
	players[0] = authenticatePlayer(t, ctx, wsURL, credentials[0])
	players[0].wait(t, ctx, "state")
	host.close()
	host = authenticateHost(t, ctx, wsURL, created)
	host.wait(t, ctx, "state")

	host.write(t, ctx, "start", "start-1", nil)
	assertAck(t, host.wait(t, ctx, "command_accepted"), "start-1", false)
	host.write(t, ctx, "start", "start-1", nil)
	assertAck(t, host.wait(t, ctx, "command_accepted"), "start-1", true)
	players[0].wait(t, ctx, "countdown_started")
	players[0].wait(t, ctx, "question_opened")

	answerQuestion(t, ctx, players, []int64{
		fixture.Questions[0].CorrectAnswerID,
		fixture.Questions[0].CorrectAnswerID,
		fixture.Questions[0].IncorrectAnswerID,
	}, "q1")

	if os.Getenv("QUIZGO_E2E_RESTART_GAME") == "1" {
		restartGameService(t, ctx)
		waitForReady(t, ctx, gameURL)
		// WebSockets do not migrate between processes; tickets reconnect them to
		// the actor restored from the Redis snapshot and its persisted deadline.
		host = authenticateHost(t, ctx, wsURL, created)
		state := decodeState(t, host.wait(t, ctx, "state"))
		for index := range players {
			players[index] = authenticatePlayer(t, ctx, wsURL, credentials[index])
			players[index].wait(t, ctx, "state")
		}
		if state.Phase == "question_open" {
			state = decodeState(t, host.wait(t, ctx, "question_closed"))
		}
		assertScoreboard(t, state, credentials)
	} else {
		assertScoreboard(t, decodeState(t, players[0].wait(t, ctx, "question_closed")), credentials)
	}

	host.write(t, ctx, "next", "next-1", nil)
	assertAck(t, host.wait(t, ctx, "command_accepted"), "next-1", false)
	players[0].wait(t, ctx, "countdown_started")
	players[0].wait(t, ctx, "question_opened")
	answerQuestion(t, ctx, players, []int64{
		fixture.Questions[1].CorrectAnswerID,
		fixture.Questions[1].IncorrectAnswerID,
		fixture.Questions[1].CorrectAnswerID,
	}, "q2")
	closed := decodeState(t, players[0].wait(t, ctx, "question_closed"))
	if closed.Phase != "finished" {
		t.Fatalf("last question closed in phase %q, want finished", closed.Phase)
	}
	final := decodeState(t, players[0].wait(t, ctx, "game_finished"))
	if final.Phase != "finished" || len(final.Players) != len(players) {
		t.Fatalf("unexpected final results: %+v", final)
	}
	scores := scoresByID(final)
	if scores[credentials[0].PlayerID] <= scores[credentials[1].PlayerID] || scores[credentials[0].PlayerID] <= scores[credentials[2].PlayerID] {
		t.Fatalf("player answering both questions correctly should lead: %+v", scores)
	}
}

func joinPlayer(t *testing.T, ctx context.Context, base string, game systemtest.Game, nickname string) (*peer, playerCredentials) {
	t.Helper()
	connection := dial(t, ctx, withQuery(base, "code", game.Code))
	connection.write(t, ctx, "join", "join-"+nickname, map[string]any{"code": game.Code, "nickname": nickname})
	joined := connection.wait(t, ctx, "joined")
	var credentials playerCredentials
	if err := json.Unmarshal(joined.Payload, &credentials); err != nil {
		t.Fatal(err)
	}
	if credentials.GameID != game.ID || credentials.PlayerID == "" || credentials.Ticket == "" {
		t.Fatalf("unexpected joined payload: %s", joined.Payload)
	}
	connection.wait(t, ctx, "state")
	return connection, credentials
}

func authenticateHost(t *testing.T, ctx context.Context, base string, game systemtest.Game) *peer {
	t.Helper()
	connection := dial(t, ctx, withQuery(base, "game_id", game.ID))
	connection.write(t, ctx, "host_auth", "host-auth", map[string]any{"game_id": game.ID, "ticket": game.HostTicket})
	connection.wait(t, ctx, "authenticated")
	return connection
}

func authenticatePlayer(t *testing.T, ctx context.Context, base string, credentials playerCredentials) *peer {
	t.Helper()
	connection := dial(t, ctx, withQuery(base, "game_id", credentials.GameID))
	connection.write(t, ctx, "player_auth", "player-auth", map[string]any{
		"game_id": credentials.GameID, "participant_id": credentials.PlayerID, "ticket": credentials.Ticket,
	})
	connection.wait(t, ctx, "authenticated")
	return connection
}

func answerQuestion(t *testing.T, ctx context.Context, players []*peer, answerIDs []int64, prefix string) {
	t.Helper()
	for index, player := range players {
		requestID := fmt.Sprintf("%s-answer-%d", prefix, index)
		player.write(t, ctx, "answer", requestID, map[string]int64{"answer_id": answerIDs[index]})
		assertAck(t, player.wait(t, ctx, "command_accepted"), requestID, false)
	}
}

func dial(t *testing.T, ctx context.Context, endpoint string) *peer {
	t.Helper()
	connection, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", endpoint, err)
	}
	return &peer{connection: connection}
}

func (p *peer) write(t *testing.T, ctx context.Context, messageType, requestID string, payload any) {
	t.Helper()
	message := map[string]any{"type": messageType, "request_id": requestID}
	if payload != nil {
		message["payload"] = payload
	}
	if err := wsjson.Write(ctx, p.connection, message); err != nil {
		t.Fatalf("write %s: %v", messageType, err)
	}
}

func (p *peer) wait(t *testing.T, parent context.Context, messageType string) serverMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	for {
		var message serverMessage
		if err := wsjson.Read(ctx, p.connection, &message); err != nil {
			t.Fatalf("wait for %s: %v", messageType, err)
		}
		if message.Type == messageType {
			return message
		}
	}
}

func (p *peer) close() {
	if p != nil && p.connection != nil {
		_ = p.connection.Close(websocket.StatusNormalClosure, "test complete")
	}
}

func assertAck(t *testing.T, message serverMessage, requestID string, duplicate bool) {
	t.Helper()
	var payload struct {
		Duplicate bool `json:"duplicate"`
	}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if message.RequestID != requestID || payload.Duplicate != duplicate {
		t.Fatalf("unexpected command ack: %+v payload=%s", message, message.Payload)
	}
}

func assertErrorCode(t *testing.T, message serverMessage, requestID, code string) {
	t.Helper()
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if message.RequestID != requestID || payload.Code != code {
		t.Fatalf("unexpected error: %+v payload=%s", message, message.Payload)
	}
}

func decodeState(t *testing.T, message serverMessage) statePayload {
	t.Helper()
	var state statePayload
	if err := json.Unmarshal(message.Payload, &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func assertScoreboard(t *testing.T, state statePayload, players []playerCredentials) {
	t.Helper()
	if state.Phase != "scoreboard" || len(state.CorrectAnswerIDs) != 1 || len(state.Players) != len(players) {
		t.Fatalf("unexpected scoreboard: %+v", state)
	}
	scores := scoresByID(state)
	if scores[players[0].PlayerID] <= 0 || scores[players[1].PlayerID] <= 0 || scores[players[2].PlayerID] != 0 {
		t.Fatalf("unexpected first-question scores: %+v", scores)
	}
}

func scoresByID(state statePayload) map[string]int64 {
	result := make(map[string]int64, len(state.Players))
	for _, player := range state.Players {
		result[player.ID] = player.Score
	}
	return result
}

func websocketURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(strings.TrimRight(httpURL, "/"), "http") + "/ws"
}

func withQuery(endpoint, key, value string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		panic(err)
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func waitForReady(t *testing.T, ctx context.Context, gameURL string) {
	t.Helper()
	endpoint := strings.TrimRight(gameURL, "/") + "/readyz"
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		response, err := http.DefaultClient.Do(request)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s: %v", endpoint, ctx.Err())
		case <-ticker.C:
		}
	}
}

func restartGameService(t *testing.T, ctx context.Context) {
	t.Helper()
	project := os.Getenv("QUIZGO_E2E_COMPOSE_PROJECT")
	composeFile := os.Getenv("QUIZGO_E2E_COMPOSE_FILE")
	if project == "" || composeFile == "" {
		t.Fatal("restart requested without QUIZGO_E2E_COMPOSE_PROJECT and QUIZGO_E2E_COMPOSE_FILE")
	}
	command := exec.CommandContext(ctx, "docker", "compose", "--project-name", project, "--file", composeFile, "restart", "game")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("restart game service: %v\n%s", err, output)
	}
}
