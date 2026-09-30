// Command ws-load runs a deliberately small WebSocket baseline load test.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/kungrem23/quizgo/services/game/tests/systemtest"
)

type config struct {
	httpURL        string
	wsURL          string
	authorization  string
	quizID         int64
	answerID       int64
	rooms          int
	playersPerRoom int
	duration       time.Duration
	commandRate    float64
}

type wireMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type loadConnection struct {
	connection *websocket.Conn
	requestID  string
	pendingMu  sync.Mutex
	pending    []time.Time
}

type measurements struct {
	connections    atomic.Int64
	commandsSent   atomic.Int64
	commandAcks    atomic.Int64
	sendErrors     atomic.Int64
	readErrors     atomic.Int64
	latencyMu      sync.Mutex
	latencies      []time.Duration
	commandErrorMu sync.Mutex
	commandErrors  map[string]int64
}

func main() {
	configuration := parseFlags()
	if err := run(configuration); err != nil {
		fmt.Fprintln(os.Stderr, "ws-load:", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	var value config
	flag.StringVar(&value.httpURL, "http-url", "http://127.0.0.1:3000", "HTTP base URL used to provision quizzes and games")
	flag.StringVar(&value.wsURL, "ws-url", "ws://127.0.0.1:8081/ws", "game WebSocket URL")
	flag.StringVar(&value.authorization, "token", "", "author Authorization value; omit with quiz-id to auto-provision")
	flag.Int64Var(&value.quizID, "quiz-id", 0, "existing playable quiz ID")
	flag.Int64Var(&value.answerID, "answer-id", 0, "valid answer ID for the first question")
	flag.IntVar(&value.rooms, "rooms", 5, "number of rooms")
	flag.IntVar(&value.playersPerRoom, "players-per-room", 10, "players in each room")
	flag.DurationVar(&value.duration, "duration", 30*time.Second, "measured command phase duration")
	flag.Float64Var(&value.commandRate, "command-rate", 100, "total WebSocket commands per second across all players")
	flag.Parse()
	return value
}

func run(configuration config) error {
	if configuration.rooms < 1 || configuration.playersPerRoom < 1 || configuration.duration <= 0 || configuration.commandRate <= 0 {
		return errors.New("rooms, players-per-room, duration and command-rate must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), configuration.duration+2*time.Minute)
	defer cancel()
	api := systemtest.Client{BaseURL: configuration.httpURL, HTTP: &http.Client{Timeout: 10 * time.Second}}
	if configuration.authorization == "" && configuration.quizID == 0 && configuration.answerID == 0 {
		fixture, err := api.ProvisionQuiz(ctx, 1, 120)
		if err != nil {
			return err
		}
		configuration.authorization = fixture.Authorization
		configuration.quizID = fixture.QuizID
		configuration.answerID = fixture.Questions[0].CorrectAnswerID
	} else if configuration.authorization == "" || configuration.quizID < 1 || configuration.answerID < 1 {
		return errors.New("token, quiz-id and answer-id must be provided together")
	}
	if !strings.HasPrefix(strings.ToLower(configuration.authorization), "bearer ") {
		configuration.authorization = "Bearer " + configuration.authorization
	}

	setupStarted := time.Now()
	measured := &measurements{commandErrors: make(map[string]int64)}
	clients, allConnections, err := setupRooms(ctx, api, configuration, measured)
	if err != nil {
		closeConnections(allConnections)
		return err
	}
	defer closeConnections(allConnections)
	setupDuration := time.Since(setupStarted)

	readCtx, stopReaders := context.WithCancel(context.Background())
	defer stopReaders()
	for _, client := range clients {
		go client.read(readCtx, measured)
	}
	runStarted := time.Now()
	interval := time.Duration(float64(time.Second) / configuration.commandRate)
	if interval < time.Microsecond {
		interval = time.Microsecond
	}
	ticker := time.NewTicker(interval)
	timer := time.NewTimer(configuration.duration)
	defer ticker.Stop()
	defer timer.Stop()
	index := 0
runLoop:
	for {
		select {
		case <-timer.C:
			break runLoop
		case <-ticker.C:
			client := clients[index%len(clients)]
			index++
			writeCtx, cancelWrite := context.WithTimeout(context.Background(), 2*time.Second)
			client.send(writeCtx, configuration.answerID, measured)
			cancelWrite()
		}
	}
	elapsed := time.Since(runStarted)
	// Leave a short drain window so response latency is not truncated at test end.
	time.Sleep(time.Second)
	stopReaders()
	printResults(configuration, setupDuration, elapsed, measured)
	return nil
}

func setupRooms(ctx context.Context, api systemtest.Client, configuration config, measured *measurements) ([]*loadConnection, []*websocket.Conn, error) {
	games := make([]systemtest.Game, configuration.rooms)
	for index := range games {
		created, err := api.CreateGame(ctx, configuration.authorization, configuration.quizID)
		if err != nil {
			return nil, nil, err
		}
		games[index] = created
	}

	type result struct {
		clients     []*loadConnection
		connections []*websocket.Conn
		err         error
	}
	results := make(chan result, len(games))
	for roomIndex, game := range games {
		go func() {
			clients, connections, err := setupRoom(ctx, configuration.wsURL, game, roomIndex, configuration.playersPerRoom, configuration.answerID, measured)
			results <- result{clients: clients, connections: connections, err: err}
		}()
	}
	var clients []*loadConnection
	var connections []*websocket.Conn
	var firstError error
	for range games {
		room := <-results
		connections = append(connections, room.connections...)
		if room.err != nil && firstError == nil {
			firstError = room.err
		}
		clients = append(clients, room.clients...)
	}
	if firstError != nil {
		return nil, connections, firstError
	}
	return clients, connections, nil
}

func setupRoom(ctx context.Context, wsURL string, game systemtest.Game, roomIndex, playerCount int, answerID int64, measured *measurements) ([]*loadConnection, []*websocket.Conn, error) {
	host, err := connect(ctx, withQuery(wsURL, "game_id", game.ID))
	if err != nil {
		return nil, nil, err
	}
	connections := []*websocket.Conn{host}
	measured.connections.Add(1)
	if err := write(ctx, host, "host_auth", "load-host-auth", map[string]any{"game_id": game.ID, "ticket": game.HostTicket}); err != nil {
		return nil, connections, err
	}
	if _, err := waitFor(ctx, host, "authenticated"); err != nil {
		return nil, connections, err
	}

	type playerResult struct {
		connection *websocket.Conn
		err        error
	}
	joined := make(chan playerResult, playerCount)
	for playerIndex := 0; playerIndex < playerCount; playerIndex++ {
		go func(playerIndex int) {
			connection, err := connect(ctx, withQuery(wsURL, "code", game.Code))
			if err == nil {
				err = write(ctx, connection, "join", "load-join", map[string]any{
					"code": game.Code, "nickname": fmt.Sprintf("R%d-P%d", roomIndex+1, playerIndex+1),
				})
			}
			if err == nil {
				_, err = waitFor(ctx, connection, "joined")
			}
			if err == nil {
				_, err = waitFor(ctx, connection, "state")
			}
			joined <- playerResult{connection: connection, err: err}
		}(playerIndex)
	}
	players := make([]*websocket.Conn, 0, playerCount)
	for index := 0; index < playerCount; index++ {
		player := <-joined
		if player.connection != nil {
			connections = append(connections, player.connection)
		}
		if player.err != nil {
			return nil, connections, player.err
		}
		players = append(players, player.connection)
		measured.connections.Add(1)
	}
	if err := write(ctx, host, "start", "load-start", nil); err != nil {
		return nil, connections, err
	}
	if _, err := waitFor(ctx, host, "command_accepted"); err != nil {
		return nil, connections, err
	}
	if _, err := waitFor(ctx, host, "question_opened"); err != nil {
		return nil, connections, err
	}

	clients := make([]*loadConnection, len(players))
	answers := make(chan error, len(players))
	for index, connection := range players {
		go func(index int, connection *websocket.Conn) {
			requestID := "load-answer"
			err := write(ctx, connection, "answer", requestID, map[string]int64{"answer_id": answerID})
			if err == nil {
				_, err = waitFor(ctx, connection, "command_accepted")
			}
			if err == nil {
				clients[index] = &loadConnection{connection: connection, requestID: requestID}
			}
			answers <- err
		}(index, connection)
	}
	for range players {
		if err := <-answers; err != nil {
			return nil, connections, err
		}
	}
	return clients, connections, nil
}

func (c *loadConnection) send(ctx context.Context, answerID int64, measured *measurements) {
	c.pendingMu.Lock()
	err := write(ctx, c.connection, "answer", c.requestID, map[string]int64{"answer_id": answerID})
	if err == nil {
		c.pending = append(c.pending, time.Now())
	}
	c.pendingMu.Unlock()
	if err != nil {
		measured.sendErrors.Add(1)
		return
	}
	measured.commandsSent.Add(1)
}

func (c *loadConnection) read(ctx context.Context, measured *measurements) {
	for {
		var message wireMessage
		if err := wsjson.Read(ctx, c.connection, &message); err != nil {
			if ctx.Err() == nil {
				measured.readErrors.Add(1)
			}
			return
		}
		if message.Type != "command_accepted" && message.Type != "error" {
			continue
		}
		var started time.Time
		c.pendingMu.Lock()
		if len(c.pending) > 0 {
			started = c.pending[0]
			c.pending = c.pending[1:]
		}
		c.pendingMu.Unlock()
		if !started.IsZero() {
			measured.recordLatency(time.Since(started))
		}
		if message.Type == "command_accepted" {
			measured.commandAcks.Add(1)
			continue
		}
		var payload struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(message.Payload, &payload)
		if payload.Code == "" {
			payload.Code = "unknown"
		}
		measured.commandErrorMu.Lock()
		measured.commandErrors[payload.Code]++
		measured.commandErrorMu.Unlock()
	}
}

func (m *measurements) recordLatency(value time.Duration) {
	m.latencyMu.Lock()
	m.latencies = append(m.latencies, value)
	m.latencyMu.Unlock()
}

func printResults(configuration config, setupDuration, elapsed time.Duration, measured *measurements) {
	measured.latencyMu.Lock()
	latencies := append([]time.Duration(nil), measured.latencies...)
	measured.latencyMu.Unlock()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	fmt.Printf("WS load test complete\n")
	fmt.Printf("rooms=%d players_per_room=%d connections=%d setup=%s duration=%s target_command_rate=%.1f/s\n",
		configuration.rooms, configuration.playersPerRoom, measured.connections.Load(), setupDuration.Round(time.Millisecond), elapsed.Round(time.Millisecond), configuration.commandRate)
	measured.commandErrorMu.Lock()
	commandErrorTotal := int64(0)
	for _, count := range measured.commandErrors {
		commandErrorTotal += count
	}
	responsesPending := measured.commandsSent.Load() - measured.commandAcks.Load() - commandErrorTotal
	measured.commandErrorMu.Unlock()
	fmt.Printf("commands_sent=%d command_acks=%d responses_pending=%d send_errors=%d read_errors=%d achieved_send_rate=%.1f/s achieved_ack_rate=%.1f/s\n",
		measured.commandsSent.Load(), measured.commandAcks.Load(), responsesPending, measured.sendErrors.Load(), measured.readErrors.Load(),
		float64(measured.commandsSent.Load())/elapsed.Seconds(), float64(measured.commandAcks.Load())/elapsed.Seconds())
	fmt.Printf("latency_samples=%d p50=%s p95=%s p99=%s max=%s\n",
		len(latencies), percentile(latencies, 0.50), percentile(latencies, 0.95), percentile(latencies, 0.99), percentile(latencies, 1))
	measured.commandErrorMu.Lock()
	defer measured.commandErrorMu.Unlock()
	if len(measured.commandErrors) == 0 {
		fmt.Println("command_errors=none")
		return
	}
	keys := make([]string, 0, len(measured.commandErrors))
	for key := range measured.commandErrors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("command_errors{code=%q}=%d\n", key, measured.commandErrors[key])
	}
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(math.Ceil(float64(len(values))*quantile)) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index].Round(time.Microsecond)
}

func connect(ctx context.Context, endpoint string) (*websocket.Conn, error) {
	connection, _, err := websocket.Dial(ctx, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", endpoint, err)
	}
	return connection, nil
}

func write(ctx context.Context, connection *websocket.Conn, messageType, requestID string, payload any) error {
	message := map[string]any{"type": messageType, "request_id": requestID}
	if payload != nil {
		message["payload"] = payload
	}
	return wsjson.Write(ctx, connection, message)
}

func waitFor(ctx context.Context, connection *websocket.Conn, messageType string) (wireMessage, error) {
	for {
		var message wireMessage
		if err := wsjson.Read(ctx, connection, &message); err != nil {
			return wireMessage{}, fmt.Errorf("wait for %s: %w", messageType, err)
		}
		if message.Type == "error" {
			return wireMessage{}, fmt.Errorf("server error while waiting for %s: %s", messageType, message.Payload)
		}
		if message.Type == messageType {
			return message, nil
		}
	}
}

func withQuery(endpoint, key, value string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func closeConnections(connections []*websocket.Conn) {
	for _, connection := range connections {
		if connection != nil {
			_ = connection.Close(websocket.StatusNormalClosure, "load test complete")
		}
	}
}
