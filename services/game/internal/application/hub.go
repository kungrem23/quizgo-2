package application

import (
	"context"
	"crypto/subtle"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
)

type Role string

const (
	RoleHost   Role = "host"
	RolePlayer Role = "player"

	maxCommandReceipts = 1024
	maxRequestIDLength = 128
)

type Command struct {
	Type      string
	RequestID string
	AnswerID  int64
	PlayerID  string
}

type CommandResult struct {
	Duplicate bool
}

type OutboundEvent struct {
	Type     string
	Sequence uint64
	Payload  any
}

type Session struct {
	ID            string
	GameID        string
	ParticipantID string
	Role          Role
	Events        <-chan OutboundEvent
	Done          <-chan struct{}
	room          *room
	events        chan OutboundEvent
	done          chan struct{}
	replaced      chan struct{}
}

// Replaced returns a channel that closes when a newer authenticated session
// takes ownership of the participant.
func (s *Session) Replaced() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.replaced
}

type JoinedPlayer struct {
	Session *Session
	Player  game.Player
	Ticket  string
}

type PlayerView struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Score    int64  `json:"score"`
}

type AnswerView struct {
	ID   int64  `json:"id"`
	Text string `json:"text"`
}

type QuestionView struct {
	ID               int64        `json:"id"`
	Text             string       `json:"text"`
	ImageID          string       `json:"image_id,omitempty"`
	TimeLimitSeconds uint32       `json:"time_limit_seconds"`
	Answers          []AnswerView `json:"answers"`
}

type StateView struct {
	GameID               string        `json:"game_id"`
	Code                 string        `json:"code"`
	QuizTitle            string        `json:"quiz_title"`
	Phase                game.Phase    `json:"phase"`
	Players              []PlayerView  `json:"players"`
	CurrentQuestionIndex int           `json:"current_question_index"`
	CurrentQuestion      *QuestionView `json:"current_question,omitempty"`
	CountdownEndsAt      *time.Time    `json:"countdown_ends_at,omitempty"`
	QuestionClosesAt     *time.Time    `json:"question_closes_at,omitempty"`
	CorrectAnswerIDs     []int64       `json:"correct_answer_ids,omitempty"`
}

type Hub struct {
	repository GameRepository
	ttl        time.Duration
	now        func() time.Time
	newID      func() string
	newTicket  func() (string, error)

	mu     sync.Mutex
	rooms  map[string]*room
	closed bool
}

func NewHub(repository GameRepository, ttl time.Duration) *Hub {
	return &Hub{
		repository: repository, ttl: ttl, now: time.Now,
		newID: uuid.NewString, newTicket: randomTicket, rooms: make(map[string]*room),
	}
}

func (h *Hub) Join(ctx context.Context, code, nickname string) (JoinedPlayer, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return JoinedPlayer{}, game.ErrGameNotFound
	}
	value, err := h.repository.GetByCode(ctx, code)
	if err != nil {
		return JoinedPlayer{}, err
	}
	room, err := h.roomFor(value)
	if err != nil {
		return JoinedPlayer{}, err
	}
	ticket, err := h.newTicket()
	if err != nil {
		return JoinedPlayer{}, err
	}
	response := make(chan roomResponse, 1)
	request := roomRequest{
		kind: "join", participantID: h.newID(), nickname: nickname,
		ticketHash: ticketHash(ticket), now: h.now(), response: response,
	}
	if err := sendRoomRequest(ctx, room, request); err != nil {
		return JoinedPlayer{}, err
	}
	result, err := awaitRoomResponse(ctx, room, response)
	if err != nil {
		return JoinedPlayer{}, err
	}
	if result.err != nil {
		return JoinedPlayer{}, result.err
	}
	return JoinedPlayer{Session: result.session, Player: result.player, Ticket: ticket}, nil
}

func (h *Hub) AuthenticateHost(ctx context.Context, gameID, ticket string) (*Session, error) {
	return h.authenticate(ctx, gameID, "host", RoleHost, ticket)
}

func (h *Hub) AuthenticatePlayer(ctx context.Context, gameID, playerID, ticket string) (*Session, error) {
	return h.authenticate(ctx, gameID, playerID, RolePlayer, ticket)
}

func (h *Hub) authenticate(ctx context.Context, gameID, participantID string, role Role, ticket string) (*Session, error) {
	if gameID == "" || participantID == "" || ticket == "" {
		return nil, game.ErrUnauthorized
	}
	value, err := h.repository.GetByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	room, err := h.roomFor(value)
	if err != nil {
		return nil, err
	}
	response := make(chan roomResponse, 1)
	request := roomRequest{kind: "authenticate", participantID: participantID, role: role, ticketHash: ticketHash(ticket), response: response}
	if err := sendRoomRequest(ctx, room, request); err != nil {
		return nil, err
	}
	result, err := awaitRoomResponse(ctx, room, response)
	if err != nil {
		return nil, err
	}
	return result.session, result.err
}

func (h *Hub) Dispatch(ctx context.Context, session *Session, command string, answerID int64) error {
	_, err := h.DispatchCommand(ctx, session, Command{Type: command, AnswerID: answerID})
	return err
}

func (h *Hub) DispatchPlayer(ctx context.Context, session *Session, command, playerID string) error {
	_, err := h.DispatchCommand(ctx, session, Command{Type: command, PlayerID: playerID})
	return err
}

func (h *Hub) DispatchCommand(ctx context.Context, session *Session, command Command) (CommandResult, error) {
	if session == nil || session.room == nil {
		return CommandResult{}, game.ErrUnauthorized
	}
	if len(command.RequestID) > maxRequestIDLength {
		return CommandResult{}, game.ErrInvalidRequestID
	}
	response := make(chan roomResponse, 1)
	request := roomRequest{
		kind: command.Type, requestID: command.RequestID, session: session,
		answerID: command.AnswerID, targetPlayerID: command.PlayerID,
		now: h.now(), response: response,
	}
	if err := sendRoomRequest(ctx, session.room, request); err != nil {
		return CommandResult{}, err
	}
	result, err := awaitRoomResponse(ctx, session.room, response)
	if err != nil {
		return CommandResult{}, err
	}
	return CommandResult{Duplicate: result.duplicate}, result.err
}

func (h *Hub) Disconnect(session *Session) {
	if session == nil || session.room == nil {
		return
	}
	select {
	case session.room.requests <- roomRequest{kind: "disconnect", session: session}:
	case <-session.room.done:
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	rooms := make([]*room, 0, len(h.rooms))
	for _, room := range h.rooms {
		rooms = append(rooms, room)
	}
	h.mu.Unlock()
	for _, room := range rooms {
		select {
		case room.requests <- roomRequest{kind: "stop"}:
		case <-room.done:
		}
	}
	for _, room := range rooms {
		<-room.done
	}
}

func (h *Hub) roomFor(value game.Game) (*room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, errors.New("game hub is closed")
	}
	if existing := h.rooms[value.ID]; existing != nil {
		return existing, nil
	}
	created := &room{
		game: value, repository: h.repository, ttl: h.ttl,
		requests: make(chan roomRequest, 64), done: make(chan struct{}), subscribers: make(map[string]*Session),
	}
	h.rooms[value.ID] = created
	go created.run()
	return created, nil
}

type room struct {
	game              game.Game
	repository        GameRepository
	ttl               time.Duration
	requests          chan roomRequest
	done              chan struct{}
	subscribers       map[string]*Session
	transitionRetryAt time.Time
}

type roomRequest struct {
	kind           string
	session        *Session
	participantID  string
	nickname       string
	role           Role
	ticketHash     string
	answerID       int64
	targetPlayerID string
	requestID      string
	now            time.Time
	response       chan roomResponse
}

type roomResponse struct {
	session   *Session
	player    game.Player
	duplicate bool
	err       error
}

func sendRoomRequest(ctx context.Context, room *room, request roomRequest) error {
	select {
	case room.requests <- request:
		return nil
	case <-room.done:
		return game.ErrGameNotFound
	case <-ctx.Done():
		return ctx.Err()
	}
}

func awaitRoomResponse(ctx context.Context, room *room, response <-chan roomResponse) (roomResponse, error) {
	select {
	case result := <-response:
		return result, nil
	case <-room.done:
		return roomResponse{}, game.ErrGameNotFound
	case <-ctx.Done():
		return roomResponse{}, ctx.Err()
	}
}

func (r *room) run() {
	defer close(r.done)
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		var timerC <-chan time.Time
		if deadline := r.transitionDeadline(); deadline != nil {
			target := *deadline
			if r.transitionRetryAt.After(target) {
				target = r.transitionRetryAt
			}
			now := time.Now()
			if !target.After(now) {
				if err := r.advanceTimedPhase(now); err != nil {
					r.transitionRetryAt = now.Add(500 * time.Millisecond)
				} else {
					r.transitionRetryAt = time.Time{}
				}
				continue
			}
			delay := time.Until(target)
			if timer == nil {
				timer = time.NewTimer(delay)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(delay)
			}
			timerC = timer.C
		} else if timer != nil {
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}

		select {
		case request := <-r.requests:
			if request.kind == "stop" {
				for id := range r.subscribers {
					r.removeSubscriber(id)
				}
				return
			}
			if request.kind == "authenticate" {
				now := time.Now()
				if deadline := r.transitionDeadline(); deadline != nil && !deadline.After(now) && !r.transitionRetryAt.After(now) {
					if err := r.advanceTimedPhase(now); err != nil {
						r.transitionRetryAt = now.Add(500 * time.Millisecond)
					} else {
						r.transitionRetryAt = time.Time{}
					}
				}
			}
			r.handle(request)
		case transitionedAt := <-timerC:
			if err := r.advanceTimedPhase(transitionedAt); err != nil {
				r.transitionRetryAt = time.Now().Add(500 * time.Millisecond)
			} else {
				r.transitionRetryAt = time.Time{}
			}
		}
	}
}

func (r *room) handle(request roomRequest) {
	switch request.kind {
	case "join":
		r.join(request)
	case "authenticate":
		r.authenticate(request)
	case "disconnect":
		if request.session != nil {
			r.removeSubscriber(request.session.ID)
		}
	case "start":
		r.start(request)
	case "answer":
		r.answer(request)
	case "next":
		r.next(request)
	case "finish":
		r.finish(request)
	case "remove_player":
		r.removePlayer(request)
	case "leave":
		r.leave(request)
	default:
		r.respond(request, roomResponse{err: errors.New("unknown game command")})
	}
}

func (r *room) join(request roomRequest) {
	candidate := cloneGame(r.game)
	player, err := candidate.AddPlayer(request.participantID, request.nickname, request.ticketHash, request.now)
	if err == nil {
		err = r.persist(candidate)
	}
	if err != nil {
		r.respond(request, roomResponse{err: err})
		return
	}
	r.game = candidate
	session := r.addSubscriber(player.ID, RolePlayer)
	r.send(session, OutboundEvent{Type: "state", Sequence: r.game.Sequence, Payload: stateView(r.game)})
	r.broadcast(OutboundEvent{Type: "player_joined", Sequence: r.game.Sequence, Payload: playerView(player)})
	r.respond(request, roomResponse{session: session, player: player})
}

func (r *room) authenticate(request roomRequest) {
	valid := false
	if request.role == RoleHost && request.participantID == "host" {
		valid = equalHash(r.game.HostTicketHash, request.ticketHash)
	} else if request.role == RolePlayer {
		for _, player := range r.game.Players {
			if player.ID == request.participantID && equalHash(player.TicketHash, request.ticketHash) {
				valid = true
				break
			}
		}
	}
	if !valid {
		r.respond(request, roomResponse{err: game.ErrUnauthorized})
		return
	}
	session := r.addSubscriber(request.participantID, request.role)
	r.send(session, OutboundEvent{Type: "state", Sequence: r.game.Sequence, Payload: stateView(r.game)})
	r.respond(request, roomResponse{session: session})
}

func (r *room) start(request roomRequest) {
	if !r.isHost(request.session) {
		r.respond(request, roomResponse{err: game.ErrUnauthorized})
		return
	}
	if r.replay(request) {
		return
	}
	candidate := cloneGame(r.game)
	err := candidate.Start(request.now)
	if err == nil {
		r.remember(&candidate, request)
		err = r.persist(candidate)
	}
	if err == nil {
		r.game = candidate
		r.broadcast(OutboundEvent{Type: "countdown_started", Sequence: r.game.Sequence, Payload: stateView(r.game)})
	}
	r.respond(request, roomResponse{err: err})
}

func (r *room) answer(request roomRequest) {
	if !r.isPlayer(request.session) {
		r.respond(request, roomResponse{err: game.ErrUnauthorized})
		return
	}
	if r.replay(request) {
		return
	}
	candidate := cloneGame(r.game)
	submission, err := candidate.SubmitAnswer(request.session.ParticipantID, request.answerID, request.now)
	if err == nil {
		r.remember(&candidate, request)
		err = r.persist(candidate)
	}
	if err == nil {
		r.game = candidate
		r.send(request.session, OutboundEvent{Type: "answer_accepted", Sequence: r.game.Sequence, Payload: map[string]any{"question_id": submission.QuestionID}})
		r.broadcast(OutboundEvent{Type: "player_answered", Sequence: r.game.Sequence, Payload: map[string]string{"player_id": submission.PlayerID}})
	}
	r.respond(request, roomResponse{err: err})
}

func (r *room) next(request roomRequest) {
	if !r.isHost(request.session) {
		r.respond(request, roomResponse{err: game.ErrUnauthorized})
		return
	}
	if r.replay(request) {
		return
	}
	candidate := cloneGame(r.game)
	finished, err := candidate.Next(request.now)
	if err == nil {
		r.remember(&candidate, request)
		err = r.persist(candidate)
	}
	if err == nil {
		r.game = candidate
		eventType := "countdown_started"
		if finished {
			eventType = "game_finished"
		}
		r.broadcast(OutboundEvent{Type: eventType, Sequence: r.game.Sequence, Payload: stateView(r.game)})
	}
	r.respond(request, roomResponse{err: err})
}

func (r *room) finish(request roomRequest) {
	if !r.isHost(request.session) {
		r.respond(request, roomResponse{err: game.ErrUnauthorized})
		return
	}
	if r.replay(request) {
		return
	}
	candidate := cloneGame(r.game)
	err := candidate.Finish(request.now)
	if err == nil {
		r.remember(&candidate, request)
		err = r.persist(candidate)
	}
	if err == nil {
		r.game = candidate
		r.broadcast(OutboundEvent{Type: "game_finished", Sequence: r.game.Sequence, Payload: stateView(r.game)})
	}
	r.respond(request, roomResponse{err: err})
}

func (r *room) removePlayer(request roomRequest) {
	if !r.isHost(request.session) {
		r.respond(request, roomResponse{err: game.ErrUnauthorized})
		return
	}
	if r.replay(request) {
		return
	}
	candidate := cloneGame(r.game)
	removed, err := candidate.RemovePlayer(request.targetPlayerID, request.now)
	if err == nil {
		r.remember(&candidate, request)
		err = r.persist(candidate)
	}
	if err == nil {
		r.game = candidate
		r.broadcast(OutboundEvent{Type: "player_removed", Sequence: r.game.Sequence, Payload: playerView(removed)})
		r.removePlayerSubscribers(removed.ID, "")
	}
	r.respond(request, roomResponse{err: err})
}

func (r *room) leave(request roomRequest) {
	if !r.isPlayer(request.session) {
		r.respond(request, roomResponse{err: game.ErrUnauthorized})
		return
	}
	if r.replay(request) {
		return
	}
	candidate := cloneGame(r.game)
	removed, err := candidate.RemovePlayer(request.session.ParticipantID, request.now)
	if err == nil {
		r.remember(&candidate, request)
		err = r.persist(candidate)
	}
	if err == nil {
		r.game = candidate
		r.broadcast(OutboundEvent{Type: "player_left", Sequence: r.game.Sequence, Payload: playerView(removed)})
		// The WebSocket handler closes the command's own connection after it
		// acknowledges leave. Any other connection for this player is stale now.
		r.removePlayerSubscribers(removed.ID, request.session.ID)
	}
	r.respond(request, roomResponse{err: err})
}

func (r *room) transitionDeadline() *time.Time {
	switch r.game.Phase {
	case game.PhaseCountdown:
		return r.game.CountdownEndsAt
	case game.PhaseQuestionOpen:
		return r.game.QuestionClosesAt
	default:
		return nil
	}
}

func (r *room) advanceTimedPhase(now time.Time) error {
	switch r.game.Phase {
	case game.PhaseCountdown:
		return r.openExpiredCountdown(now)
	case game.PhaseQuestionOpen:
		return r.closeExpiredQuestion(now)
	default:
		return nil
	}
}

func (r *room) openExpiredCountdown(now time.Time) error {
	candidate := cloneGame(r.game)
	if err := candidate.OpenCurrentQuestion(now); err != nil {
		return err
	}
	if err := r.persist(candidate); err != nil {
		return err
	}
	r.game = candidate
	r.broadcast(OutboundEvent{Type: "question_opened", Sequence: r.game.Sequence, Payload: stateView(r.game)})
	return nil
}

func (r *room) closeExpiredQuestion(now time.Time) error {
	candidate := cloneGame(r.game)
	if err := candidate.CloseQuestion(now); err != nil {
		return err
	}
	if err := r.persist(candidate); err != nil {
		return err
	}
	r.game = candidate
	question, _ := r.game.CurrentQuestion()
	r.broadcast(OutboundEvent{Type: "question_closed", Sequence: r.game.Sequence, Payload: map[string]any{
		"phase": r.game.Phase, "question_id": question.ID, "correct_answer_ids": correctAnswerIDs(question), "players": leaderboard(r.game.Players),
	}})
	if r.game.Phase == game.PhaseFinished {
		r.broadcast(OutboundEvent{Type: "game_finished", Sequence: r.game.Sequence, Payload: stateView(r.game)})
	}
	return nil
}

func (r *room) persist(candidate game.Game) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return r.repository.Update(ctx, candidate, r.ttl)
}

func (r *room) replay(request roomRequest) bool {
	if request.requestID == "" || request.session == nil {
		return false
	}
	for _, receipt := range r.game.CommandReceipts {
		if receipt.ParticipantID == request.session.ParticipantID && receipt.Command == request.kind && receipt.RequestID == request.requestID {
			r.respond(request, roomResponse{duplicate: true})
			return true
		}
	}
	return false
}

func (r *room) remember(candidate *game.Game, request roomRequest) {
	if request.requestID == "" || request.session == nil {
		return
	}
	candidate.CommandReceipts = append(candidate.CommandReceipts, game.CommandReceipt{
		ParticipantID: request.session.ParticipantID,
		Command:       request.kind,
		RequestID:     request.requestID,
	})
	if overflow := len(candidate.CommandReceipts) - maxCommandReceipts; overflow > 0 {
		candidate.CommandReceipts = append([]game.CommandReceipt(nil), candidate.CommandReceipts[overflow:]...)
	}
}

func (r *room) addSubscriber(participantID string, role Role) *Session {
	for id, session := range r.subscribers {
		if session.ParticipantID == participantID && session.Role == role {
			close(session.replaced)
			r.removeSubscriber(id)
		}
	}
	session := &Session{
		ID: uuid.NewString(), GameID: r.game.ID, ParticipantID: participantID, Role: role,
		room: r, events: make(chan OutboundEvent, 32), done: make(chan struct{}), replaced: make(chan struct{}),
	}
	session.Events = session.events
	session.Done = session.done
	r.subscribers[session.ID] = session
	return session
}

func (r *room) removeSubscriber(id string) {
	if session := r.subscribers[id]; session != nil {
		delete(r.subscribers, id)
		close(session.done)
		close(session.events)
	}
}

func (r *room) removePlayerSubscribers(playerID, exceptSessionID string) {
	for id, session := range r.subscribers {
		if id != exceptSessionID && session.Role == RolePlayer && session.ParticipantID == playerID {
			r.removeSubscriber(id)
		}
	}
}

func (r *room) broadcast(event OutboundEvent) {
	for _, session := range r.subscribers {
		r.send(session, event)
	}
}

func (r *room) send(session *Session, event OutboundEvent) {
	select {
	case session.events <- event:
	default:
		r.removeSubscriber(session.ID)
	}
}

func (r *room) respond(request roomRequest, response roomResponse) {
	if request.response != nil {
		request.response <- response
	}
}

func (r *room) isHost(session *Session) bool {
	return session != nil && session.Role == RoleHost && r.subscribers[session.ID] == session
}

func (r *room) isPlayer(session *Session) bool {
	if session == nil || session.Role != RolePlayer || r.subscribers[session.ID] != session {
		return false
	}
	for _, player := range r.game.Players {
		if player.ID == session.ParticipantID {
			return true
		}
	}
	return false
}

func equalHash(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func cloneGame(value game.Game) game.Game {
	clone := value
	clone.Players = append([]game.Player(nil), value.Players...)
	clone.Submissions = append([]game.Submission(nil), value.Submissions...)
	clone.CommandReceipts = append([]game.CommandReceipt(nil), value.CommandReceipts...)
	clone.Quiz.Questions = append([]game.Question(nil), value.Quiz.Questions...)
	for index := range clone.Quiz.Questions {
		clone.Quiz.Questions[index].Answers = append([]game.Answer(nil), value.Quiz.Questions[index].Answers...)
	}
	if value.QuestionOpenedAt != nil {
		openedAt := *value.QuestionOpenedAt
		clone.QuestionOpenedAt = &openedAt
	}
	if value.CountdownEndsAt != nil {
		countdownEndsAt := *value.CountdownEndsAt
		clone.CountdownEndsAt = &countdownEndsAt
	}
	if value.QuestionClosesAt != nil {
		closesAt := *value.QuestionClosesAt
		clone.QuestionClosesAt = &closesAt
	}
	return clone
}

func stateView(value game.Game) StateView {
	view := StateView{
		GameID: value.ID, Code: value.Code, QuizTitle: value.Quiz.Title, Phase: value.Phase,
		Players: leaderboard(value.Players), CurrentQuestionIndex: value.CurrentQuestionIndex,
		CountdownEndsAt: value.CountdownEndsAt, QuestionClosesAt: value.QuestionClosesAt,
	}
	if question, ok := value.CurrentQuestion(); ok && (value.Phase == game.PhaseQuestionOpen || value.Phase == game.PhaseScoreboard || value.Phase == game.PhaseQuestionClosed) {
		answers := make([]AnswerView, len(question.Answers))
		for index, answer := range question.Answers {
			answers[index] = AnswerView{ID: answer.ID, Text: answer.Text}
		}
		view.CurrentQuestion = &QuestionView{
			ID: question.ID, Text: question.Text, ImageID: question.ImageID,
			TimeLimitSeconds: question.TimeLimitSeconds, Answers: answers,
		}
		if value.Phase == game.PhaseScoreboard || value.Phase == game.PhaseQuestionClosed {
			view.CorrectAnswerIDs = correctAnswerIDs(question)
		}
	}
	return view
}

func correctAnswerIDs(question game.Question) []int64 {
	result := make([]int64, 0, 1)
	for _, answer := range question.Answers {
		if answer.IsCorrect {
			result = append(result, answer.ID)
		}
	}
	return result
}

func leaderboard(players []game.Player) []PlayerView {
	result := make([]PlayerView, len(players))
	for index, player := range players {
		result[index] = playerView(player)
	}
	sort.SliceStable(result, func(left, right int) bool {
		if result[left].Score == result[right].Score {
			return result[left].Nickname < result[right].Nickname
		}
		return result[left].Score > result[right].Score
	})
	return result
}

func playerView(player game.Player) PlayerView {
	return PlayerView{ID: player.ID, Nickname: player.Nickname, Score: player.Score}
}
