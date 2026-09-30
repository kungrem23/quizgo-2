package application_test

import . "github.com/kungrem23/quizgo/services/game/internal/application"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	game "github.com/kungrem23/quizgo/services/game/internal/domain"
)

type catalogStub struct{ snapshot game.QuizSnapshot }

func (c catalogStub) GetPlayableQuiz(context.Context, int64, string) (game.QuizSnapshot, error) {
	return c.snapshot, nil
}

type repositoryStub struct {
	mu      sync.Mutex
	saved   game.Game
	updates int
}

func (r *repositoryStub) Save(_ context.Context, value game.Game, _ time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved = value
	return nil
}

func (r *repositoryStub) Update(_ context.Context, value game.Game, _ time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved = value
	r.updates++
	return nil
}

func (r *repositoryStub) GetByID(_ context.Context, id string) (game.Game, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saved.ID != id {
		return game.Game{}, game.ErrGameNotFound
	}
	return r.saved, nil
}

func (r *repositoryStub) GetByCode(_ context.Context, code string) (game.Game, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saved.Code != code {
		return game.Game{}, game.ErrGameNotFound
	}
	return r.saved, nil
}

func (r *repositoryStub) updateCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.updates
}

func TestCreateGameUsesImmutableQuizSnapshot(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{ID: 1, Text: "Q", TimeLimitSeconds: 10, Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}}}},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)

	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Game.ID == "" || len(created.Game.Code) != 6 || created.Game.HostUserID != 42 || created.Game.Quiz.Revision != 2 || repository.saved.ID != created.Game.ID || created.HostTicket == "" || repository.saved.HostTicketHash == "" || repository.saved.HostTicketHash == created.HostTicket {
		t.Fatalf("unexpected game: %#v", created)
	}
}

func TestHubRunsGameLifecycleAndPersistsEveryTransition(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 1,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(repository, time.Hour)
	defer hub.Close()

	joined, err := hub.Join(context.Background(), created.Game.Code, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.AuthenticateHost(context.Background(), created.Game.ID, "wrong-ticket"); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("wrong host ticket error = %v", err)
	}
	playerSession, err := hub.AuthenticatePlayer(context.Background(), created.Game.ID, joined.Player.ID, joined.Ticket)
	if err != nil {
		t.Fatalf("reconnect player: %v", err)
	}
	host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}
	if err := hub.Dispatch(context.Background(), playerSession, "start", 0); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("player start error = %v", err)
	}
	if err := hub.Dispatch(context.Background(), host, "start", 0); err != nil {
		t.Fatal(err)
	}
	countdown := waitForEvent(t, playerSession.Events, "countdown_started", 2*time.Second)
	countdownState, ok := countdown.Payload.(StateView)
	if !ok || countdownState.Phase != game.PhaseCountdown || countdownState.CountdownEndsAt == nil || countdownState.CurrentQuestion != nil {
		t.Fatalf("unexpected countdown payload: %#v", countdown.Payload)
	}
	reconnectedDuringCountdown, err := hub.AuthenticatePlayer(context.Background(), created.Game.ID, joined.Player.ID, joined.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	reconnectedCountdownState := waitForEvent(t, reconnectedDuringCountdown.Events, "state", 2*time.Second)
	if state, ok := reconnectedCountdownState.Payload.(StateView); !ok || state.Phase != game.PhaseCountdown || state.CountdownEndsAt == nil {
		t.Fatalf("unexpected reconnected countdown state: %#v", reconnectedCountdownState.Payload)
	}
	waitForEvent(t, reconnectedDuringCountdown.Events, "question_opened", 5*time.Second)
	if err := hub.Dispatch(context.Background(), reconnectedDuringCountdown, "answer", 1); err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, reconnectedDuringCountdown.Events, "answer_accepted", 2*time.Second)
	answered, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if answered.Phase != game.PhaseQuestionOpen || len(answered.Submissions) != 1 || answered.QuestionClosesAt == nil {
		t.Fatalf("all players answering closed the question before its deadline: %#v", answered)
	}
	waitForEvent(t, reconnectedDuringCountdown.Events, "question_closed", 3*time.Second)
	waitForEvent(t, reconnectedDuringCountdown.Events, "game_finished", 2*time.Second)
	reconnectedAfterFinish, err := hub.AuthenticatePlayer(context.Background(), created.Game.ID, joined.Player.ID, joined.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	reconnectedFinishedState := waitForEvent(t, reconnectedAfterFinish.Events, "state", 2*time.Second)
	if state, ok := reconnectedFinishedState.Payload.(StateView); !ok || state.Phase != game.PhaseFinished || len(state.Players) != 1 {
		t.Fatalf("unexpected reconnected finished state: %#v", reconnectedFinishedState.Payload)
	}

	persisted, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Phase != game.PhaseFinished || len(persisted.Submissions) != 1 || persisted.Players[0].Score < 500 {
		t.Fatalf("unexpected persisted game: %#v", persisted)
	}
}

func TestHubReconnectReplacesOldPlayerSession(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(repository, time.Hour)
	defer hub.Close()

	joined, err := hub.Join(context.Background(), created.Game.Code, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	reconnected, err := hub.AuthenticatePlayer(context.Background(), created.Game.ID, joined.Player.ID, joined.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	waitForDone(t, joined.Session.Done, 2*time.Second)
	if _, err := hub.DispatchCommand(context.Background(), joined.Session, Command{Type: "answer", AnswerID: 1}); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("old session command error = %v", err)
	}
	if _, err := hub.DispatchCommand(context.Background(), reconnected, Command{Type: "answer", AnswerID: 1}); !errors.Is(err, game.ErrInvalidPhase) {
		t.Fatalf("replacement session command error = %v", err)
	}

	hub.Disconnect(joined.Session)
	if _, err := hub.DispatchCommand(context.Background(), reconnected, Command{Type: "answer", AnswerID: 1}); !errors.Is(err, game.ErrInvalidPhase) {
		t.Fatalf("old disconnect invalidated replacement: %v", err)
	}
	persisted, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Players) != 1 || persisted.Players[0].ID != joined.Player.ID {
		t.Fatalf("reconnect changed persisted player state: %#v", persisted.Players)
	}
}

func TestHubPlayerReconnectReturnsCurrentStateInEveryPhase(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{
			{ID: 1, Text: "Q1", TimeLimitSeconds: 60, Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}}},
			{ID: 2, Text: "Q2", TimeLimitSeconds: 60, Answers: []game.Answer{{ID: 3, Text: "A", IsCorrect: true}, {ID: 4, Text: "B"}}},
		},
	}
	tests := []struct {
		name    string
		phase   game.Phase
		prepare func(*game.Game, time.Time) error
	}{
		{name: "lobby", phase: game.PhaseLobby, prepare: func(*game.Game, time.Time) error { return nil }},
		{name: "countdown", phase: game.PhaseCountdown, prepare: func(value *game.Game, now time.Time) error {
			return value.Start(now)
		}},
		{name: "question", phase: game.PhaseQuestionOpen, prepare: func(value *game.Game, now time.Time) error {
			if err := value.Start(now.Add(-game.CountdownDuration)); err != nil {
				return err
			}
			return value.OpenCurrentQuestion(now)
		}},
		{name: "scoreboard", phase: game.PhaseScoreboard, prepare: func(value *game.Game, now time.Time) error {
			if err := value.Start(now.Add(-game.CountdownDuration)); err != nil {
				return err
			}
			if err := value.OpenCurrentQuestion(now); err != nil {
				return err
			}
			return value.CloseQuestion(now.Add(time.Second))
		}},
		{name: "finished", phase: game.PhaseFinished, prepare: func(value *game.Game, now time.Time) error {
			return value.Finish(now)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &repositoryStub{}
			service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
			created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
			if err != nil {
				t.Fatal(err)
			}
			initialHub := NewHub(repository, time.Hour)
			joined, err := initialHub.Join(context.Background(), created.Game.Code, "Alice")
			if err != nil {
				initialHub.Close()
				t.Fatal(err)
			}
			initialHub.Close()

			prepared, err := repository.GetByID(context.Background(), created.Game.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err := test.prepare(&prepared, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			if err := repository.Update(context.Background(), prepared, time.Hour); err != nil {
				t.Fatal(err)
			}

			hub := NewHub(repository, time.Hour)
			defer hub.Close()
			session, err := hub.AuthenticatePlayer(context.Background(), created.Game.ID, joined.Player.ID, joined.Ticket)
			if err != nil {
				t.Fatal(err)
			}
			event := waitForEvent(t, session.Events, "state", 2*time.Second)
			state, ok := event.Payload.(StateView)
			if !ok || state.Phase != test.phase || len(state.Players) != 1 || state.Players[0].ID != joined.Player.ID {
				t.Fatalf("unexpected reconnect state: %#v", event.Payload)
			}
			switch test.phase {
			case game.PhaseCountdown:
				if state.CountdownEndsAt == nil || state.CurrentQuestion != nil {
					t.Fatalf("unexpected countdown state: %#v", state)
				}
			case game.PhaseQuestionOpen:
				if state.QuestionClosesAt == nil || state.CurrentQuestion == nil || len(state.CorrectAnswerIDs) != 0 {
					t.Fatalf("unexpected question state: %#v", state)
				}
			case game.PhaseScoreboard:
				if state.CurrentQuestion == nil || len(state.CorrectAnswerIDs) != 1 || state.CorrectAnswerIDs[0] != 1 {
					t.Fatalf("unexpected scoreboard state: %#v", state)
				}
			}
		})
	}
}

func TestHubLobbyCommandsEnforceOwnershipAndPersistRemoval(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(repository, time.Hour)
	defer hub.Close()

	alice, err := hub.Join(context.Background(), created.Game.Code, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := hub.Join(context.Background(), created.Game.Code, "Bob")
	if err != nil {
		t.Fatal(err)
	}
	host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}

	if err := hub.DispatchPlayer(context.Background(), alice.Session, "remove_player", bob.Player.ID); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("player removing another player error = %v", err)
	}
	removedResult, err := hub.DispatchCommand(context.Background(), host, Command{Type: "remove_player", RequestID: "remove-alice", PlayerID: alice.Player.ID})
	if err != nil || removedResult.Duplicate {
		t.Fatalf("first remove player = %#v, %v", removedResult, err)
	}
	removed := waitForEvent(t, bob.Session.Events, "player_removed", 2*time.Second)
	if player, ok := removed.Payload.(PlayerView); !ok || player.ID != alice.Player.ID {
		t.Fatalf("unexpected player_removed payload: %#v", removed.Payload)
	}
	waitForEvent(t, alice.Session.Events, "player_removed", 2*time.Second)
	waitForClosed(t, alice.Session.Events, 2*time.Second)
	repeatedRemove, err := hub.DispatchCommand(context.Background(), host, Command{Type: "remove_player", RequestID: "remove-alice", PlayerID: alice.Player.ID})
	if err != nil || !repeatedRemove.Duplicate {
		t.Fatalf("repeated remove player = %#v, %v", repeatedRemove, err)
	}
	if _, err := hub.AuthenticatePlayer(context.Background(), created.Game.ID, alice.Player.ID, alice.Ticket); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("removed player reconnect error = %v", err)
	}
	persisted, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Players) != 1 || persisted.Players[0].ID != bob.Player.ID {
		t.Fatalf("unexpected persisted lobby: %#v", persisted.Players)
	}

	if err := hub.Dispatch(context.Background(), host, "start", 0); err != nil {
		t.Fatal(err)
	}
	if err := hub.DispatchPlayer(context.Background(), host, "remove_player", bob.Player.ID); !errors.Is(err, game.ErrInvalidPhase) {
		t.Fatalf("remove after lobby error = %v", err)
	}
	if err := hub.Dispatch(context.Background(), bob.Session, "leave", 0); !errors.Is(err, game.ErrInvalidPhase) {
		t.Fatalf("leave after lobby error = %v", err)
	}
}

func TestHubLeaveRemovesOnlyCurrentPlayerAndInvalidatesReconnect(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(repository, time.Hour)
	defer hub.Close()

	alice, err := hub.Join(context.Background(), created.Game.Code, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := hub.Join(context.Background(), created.Game.Code, "Bob")
	if err != nil {
		t.Fatal(err)
	}
	if err := hub.Dispatch(context.Background(), alice.Session, "leave", 0); err != nil {
		t.Fatal(err)
	}
	left := waitForEvent(t, bob.Session.Events, "player_left", 2*time.Second)
	if player, ok := left.Payload.(PlayerView); !ok || player.ID != alice.Player.ID {
		t.Fatalf("unexpected player_left payload: %#v", left.Payload)
	}
	if err := hub.Dispatch(context.Background(), alice.Session, "leave", 0); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("stale player session error = %v", err)
	}
	if _, err := hub.AuthenticatePlayer(context.Background(), created.Game.ID, alice.Player.ID, alice.Ticket); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("left player reconnect error = %v", err)
	}
	persisted, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Players) != 1 || persisted.Players[0].ID != bob.Player.ID {
		t.Fatalf("leave affected another player: %#v", persisted.Players)
	}
}

func TestHubHostFinishIsAuthorizedAndPhaseChecked(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(repository, time.Hour)
	defer hub.Close()
	player, err := hub.Join(context.Background(), created.Game.Code, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}

	if err := hub.Dispatch(context.Background(), player.Session, "finish", 0); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("player finish error = %v", err)
	}
	if err := hub.Dispatch(context.Background(), player.Session, "next", 0); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("player next error = %v", err)
	}
	if err := hub.Dispatch(context.Background(), host, "start", 0); err != nil {
		t.Fatal(err)
	}
	if err := hub.Dispatch(context.Background(), host, "finish", 0); err != nil {
		t.Fatal(err)
	}
	finished := waitForEvent(t, player.Session.Events, "game_finished", 2*time.Second)
	if state, ok := finished.Payload.(StateView); !ok || state.Phase != game.PhaseFinished {
		t.Fatalf("unexpected finish payload: %#v", finished.Payload)
	}
	if err := hub.Dispatch(context.Background(), host, "finish", 0); !errors.Is(err, game.ErrInvalidPhase) {
		t.Fatalf("second finish error = %v", err)
	}
	if err := hub.Dispatch(context.Background(), host, "start", 0); !errors.Is(err, game.ErrInvalidPhase) {
		t.Fatalf("start after finish error = %v", err)
	}
	persisted, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Phase != game.PhaseFinished {
		t.Fatalf("finish was not persisted: %#v", persisted)
	}
}

func TestHubDeduplicatesStartAndTreatsDifferentRequestIDAsNew(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(repository, time.Hour)
	defer hub.Close()
	if _, err := hub.Join(context.Background(), created.Game.Code, "Alice"); err != nil {
		t.Fatal(err)
	}
	host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}

	first, err := hub.DispatchCommand(context.Background(), host, Command{Type: "start", RequestID: "start-1"})
	if err != nil || first.Duplicate {
		t.Fatalf("first start = %#v, %v", first, err)
	}
	started, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := hub.DispatchCommand(context.Background(), host, Command{Type: "start", RequestID: "start-1"})
	if err != nil || !repeated.Duplicate {
		t.Fatalf("repeated start = %#v, %v", repeated, err)
	}
	afterRepeat, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterRepeat.Sequence != started.Sequence || len(afterRepeat.CommandReceipts) != 1 {
		t.Fatalf("duplicate start changed state: before=%#v after=%#v", started, afterRepeat)
	}

	if result, err := hub.DispatchCommand(context.Background(), host, Command{Type: "start", RequestID: "start-2"}); !errors.Is(err, game.ErrInvalidPhase) || result.Duplicate {
		t.Fatalf("different request id start = %#v, %v", result, err)
	}
}

func TestHubDeduplicatesAnswerAfterReconnectAndRecovery(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 60,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(repository, time.Hour)
	player, err := hub.Join(context.Background(), created.Game.Code, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.DispatchCommand(context.Background(), host, Command{Type: "start", RequestID: "start-1"}); err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, player.Session.Events, "question_opened", 5*time.Second)
	first, err := hub.DispatchCommand(context.Background(), player.Session, Command{Type: "answer", RequestID: "answer-1", AnswerID: 1})
	if err != nil || first.Duplicate {
		t.Fatalf("first answer = %#v, %v", first, err)
	}
	answered, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(answered.Submissions) != 1 {
		t.Fatalf("submissions after first answer = %d", len(answered.Submissions))
	}
	hub.Close()

	recoveredHub := NewHub(repository, time.Hour)
	defer recoveredHub.Close()
	reconnected, err := recoveredHub.AuthenticatePlayer(context.Background(), created.Game.ID, player.Player.ID, player.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, reconnected.Events, "state", 2*time.Second)
	repeated, err := recoveredHub.DispatchCommand(context.Background(), reconnected, Command{Type: "answer", RequestID: "answer-1", AnswerID: 2})
	if err != nil || !repeated.Duplicate {
		t.Fatalf("repeated answer after recovery = %#v, %v", repeated, err)
	}
	afterRepeat, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterRepeat.Sequence != answered.Sequence || len(afterRepeat.Submissions) != 1 || afterRepeat.Submissions[0].AnswerID != 1 {
		t.Fatalf("duplicate answer changed state: before=%#v after=%#v", answered, afterRepeat)
	}
	if result, err := recoveredHub.DispatchCommand(context.Background(), reconnected, Command{Type: "answer", RequestID: "answer-2", AnswerID: 2}); !errors.Is(err, game.ErrAlreadyAnswered) || result.Duplicate {
		t.Fatalf("different request id answer = %#v, %v", result, err)
	}
}

func TestHubDeduplicatesNext(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{
			{ID: 1, Text: "Q1", TimeLimitSeconds: 10, Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}}},
			{ID: 2, Text: "Q2", TimeLimitSeconds: 10, Answers: []game.Answer{{ID: 3, Text: "A", IsCorrect: true}, {ID: 4, Text: "B"}}},
		},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(-time.Minute)
	prepared := created.Game
	if _, err := prepared.AddPlayer("player-1", "Alice", "ticket-hash", now); err != nil {
		t.Fatal(err)
	}
	if err := prepared.Start(now); err != nil {
		t.Fatal(err)
	}
	if err := prepared.OpenCurrentQuestion(now.Add(game.CountdownDuration)); err != nil {
		t.Fatal(err)
	}
	if err := prepared.CloseQuestion(now.Add(game.CountdownDuration + time.Second)); err != nil {
		t.Fatal(err)
	}
	repository.saved = prepared

	hub := NewHub(repository, time.Hour)
	defer hub.Close()
	host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}
	first, err := hub.DispatchCommand(context.Background(), host, Command{Type: "next", RequestID: "next-1"})
	if err != nil || first.Duplicate {
		t.Fatalf("first next = %#v, %v", first, err)
	}
	advanced, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.CurrentQuestionIndex != 1 {
		t.Fatalf("question index after next = %d", advanced.CurrentQuestionIndex)
	}
	repeated, err := hub.DispatchCommand(context.Background(), host, Command{Type: "next", RequestID: "next-1"})
	if err != nil || !repeated.Duplicate {
		t.Fatalf("repeated next = %#v, %v", repeated, err)
	}
	afterRepeat, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterRepeat.Sequence != advanced.Sequence || afterRepeat.CurrentQuestionIndex != 1 {
		t.Fatalf("duplicate next changed state: before=%#v after=%#v", advanced, afterRepeat)
	}
}

func TestHubBoundsPersistedCommandReceipts(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	prepared := created.Game
	for index := 0; index < 1100; index++ {
		prepared.CommandReceipts = append(prepared.CommandReceipts, game.CommandReceipt{
			ParticipantID: "host", Command: "finish", RequestID: fmt.Sprintf("old-%d", index),
		})
	}
	repository.saved = prepared

	hub := NewHub(repository, time.Hour)
	defer hub.Close()
	host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hub.DispatchCommand(context.Background(), host, Command{Type: "finish", RequestID: "newest"}); err != nil {
		t.Fatal(err)
	}
	persisted, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.CommandReceipts) != 1024 || persisted.CommandReceipts[len(persisted.CommandReceipts)-1].RequestID != "newest" {
		t.Fatalf("unexpected bounded receipts: count=%d last=%#v", len(persisted.CommandReceipts), persisted.CommandReceipts[len(persisted.CommandReceipts)-1])
	}
}

func TestHubDisconnectedRemovedPlayerCannotReconnect(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{ID: 1, Text: "Q", TimeLimitSeconds: 10, Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}}}},
	}
	repository := &repositoryStub{}
	service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
	created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(repository, time.Hour)
	defer hub.Close()
	player, err := hub.Join(context.Background(), created.Game.Code, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}
	hub.Disconnect(player.Session)
	if err := hub.DispatchPlayer(context.Background(), host, "remove_player", player.Player.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := hub.AuthenticatePlayer(context.Background(), created.Game.ID, player.Player.ID, player.Ticket); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("disconnected removed player reconnect error = %v", err)
	}
}

func TestHubRecoversPersistedTimedPhases(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{
			{ID: 1, Text: "Q", TimeLimitSeconds: 10, Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}}},
			{ID: 2, Text: "Q2", TimeLimitSeconds: 10, Answers: []game.Answer{{ID: 3, Text: "A2", IsCorrect: true}, {ID: 4, Text: "B2"}}},
		},
	}

	t.Run("restores countdown timer from absolute deadline", func(t *testing.T) {
		repository := &repositoryStub{}
		service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
		created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
		if err != nil {
			t.Fatal(err)
		}
		recovered := created.Game
		startedAt := time.Now().UTC().Add(-game.CountdownDuration + time.Second)
		if _, err := recovered.AddPlayer("player-1", "Alice", "ticket-hash", startedAt); err != nil {
			t.Fatal(err)
		}
		if err := recovered.Start(startedAt); err != nil {
			t.Fatal(err)
		}
		countdownEndsAt := *recovered.CountdownEndsAt
		if err := repository.Update(context.Background(), recovered, time.Hour); err != nil {
			t.Fatal(err)
		}
		updatesBeforeRecovery := repository.updateCount()

		hub := NewHub(repository, time.Hour)
		defer hub.Close()
		host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
		if err != nil {
			t.Fatal(err)
		}
		event := waitForEvent(t, host.Events, "state", 2*time.Second)
		state, ok := event.Payload.(StateView)
		if !ok || state.Phase != game.PhaseCountdown || state.CountdownEndsAt == nil || !state.CountdownEndsAt.Equal(countdownEndsAt) {
			t.Fatalf("unexpected recovered state: %#v", event.Payload)
		}
		waitForEvent(t, host.Events, "question_opened", 2*time.Second)

		persisted, err := repository.GetByID(context.Background(), created.Game.ID)
		if err != nil {
			t.Fatal(err)
		}
		expectedClose := countdownEndsAt.Add(10 * time.Second)
		if persisted.Phase != game.PhaseQuestionOpen || persisted.QuestionOpenedAt == nil || !persisted.QuestionOpenedAt.Equal(countdownEndsAt) || persisted.QuestionClosesAt == nil || !persisted.QuestionClosesAt.Equal(expectedClose) {
			t.Fatalf("countdown did not resume from its deadline: %#v", persisted)
		}
		time.Sleep(100 * time.Millisecond)
		if updates := repository.updateCount(); updates != updatesBeforeRecovery+1 {
			t.Fatalf("countdown transition persisted %d times, want 1", updates-updatesBeforeRecovery)
		}
	})

	t.Run("restores current question timer", func(t *testing.T) {
		repository := &repositoryStub{}
		service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
		created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
		if err != nil {
			t.Fatal(err)
		}
		recovered := created.Game
		openedAt := time.Now().UTC().Add(-9500 * time.Millisecond)
		startedAt := openedAt.Add(-game.CountdownDuration)
		if _, err := recovered.AddPlayer("player-1", "Alice", "ticket-hash", startedAt); err != nil {
			t.Fatal(err)
		}
		if err := recovered.Start(startedAt); err != nil {
			t.Fatal(err)
		}
		if err := recovered.OpenCurrentQuestion(openedAt); err != nil {
			t.Fatal(err)
		}
		questionClosesAt := *recovered.QuestionClosesAt
		if err := repository.Update(context.Background(), recovered, time.Hour); err != nil {
			t.Fatal(err)
		}
		updatesBeforeRecovery := repository.updateCount()

		hub := NewHub(repository, time.Hour)
		defer hub.Close()
		host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
		if err != nil {
			t.Fatal(err)
		}
		event := waitForEvent(t, host.Events, "state", 2*time.Second)
		state, ok := event.Payload.(StateView)
		if !ok || state.Phase != game.PhaseQuestionOpen || state.QuestionClosesAt == nil || !state.QuestionClosesAt.Equal(questionClosesAt) {
			t.Fatalf("unexpected recovered question: %#v", event.Payload)
		}
		waitForEvent(t, host.Events, "question_closed", 2*time.Second)

		persisted, err := repository.GetByID(context.Background(), created.Game.ID)
		if err != nil {
			t.Fatal(err)
		}
		if persisted.Phase != game.PhaseScoreboard || !persisted.LastActivity.Equal(questionClosesAt) {
			t.Fatalf("question did not close at its deadline: %#v", persisted)
		}
		time.Sleep(100 * time.Millisecond)
		if updates := repository.updateCount(); updates != updatesBeforeRecovery+1 {
			t.Fatalf("question transition persisted %d times, want 1", updates-updatesBeforeRecovery)
		}
	})

	t.Run("catches up countdown and question before reconnect", func(t *testing.T) {
		repository := &repositoryStub{}
		service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
		created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
		if err != nil {
			t.Fatal(err)
		}
		recovered := created.Game
		startedAt := time.Now().UTC().Add(-time.Minute)
		if _, err := recovered.AddPlayer("player-1", "Alice", "ticket-hash", startedAt); err != nil {
			t.Fatal(err)
		}
		if err := recovered.Start(startedAt); err != nil {
			t.Fatal(err)
		}
		countdownEndsAt := *recovered.CountdownEndsAt
		expectedQuestionClose := countdownEndsAt.Add(10 * time.Second)
		expectedSequence := recovered.Sequence + 2
		if err := repository.Update(context.Background(), recovered, time.Hour); err != nil {
			t.Fatal(err)
		}
		updatesBeforeRecovery := repository.updateCount()

		hub := NewHub(repository, time.Hour)
		defer hub.Close()
		host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
		if err != nil {
			t.Fatal(err)
		}
		event := waitForEvent(t, host.Events, "state", 2*time.Second)
		state, ok := event.Payload.(StateView)
		if !ok || state.Phase != game.PhaseScoreboard || state.CountdownEndsAt != nil || state.QuestionClosesAt != nil || len(state.CorrectAnswerIDs) != 1 {
			t.Fatalf("unexpected caught-up state: %#v", event.Payload)
		}

		persisted, err := repository.GetByID(context.Background(), created.Game.ID)
		if err != nil {
			t.Fatal(err)
		}
		if persisted.Sequence != expectedSequence || persisted.QuestionOpenedAt == nil || !persisted.QuestionOpenedAt.Equal(countdownEndsAt) || !persisted.LastActivity.Equal(expectedQuestionClose) {
			t.Fatalf("timed phases were not replayed deterministically: %#v", persisted)
		}
		if updates := repository.updateCount(); updates != updatesBeforeRecovery+2 {
			t.Fatalf("catch-up persisted %d transitions, want 2", updates-updatesBeforeRecovery)
		}
	})

	t.Run("long downtime finishes the last question", func(t *testing.T) {
		lastQuestionSnapshot := snapshot
		lastQuestionSnapshot.Questions = append([]game.Question(nil), snapshot.Questions[:1]...)
		repository := &repositoryStub{}
		service := New(catalogStub{snapshot: lastQuestionSnapshot}, repository, time.Hour)
		created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
		if err != nil {
			t.Fatal(err)
		}
		recovered := created.Game
		startedAt := time.Now().UTC().Add(-time.Minute)
		if _, err := recovered.AddPlayer("player-1", "Alice", "ticket-hash", startedAt); err != nil {
			t.Fatal(err)
		}
		if err := recovered.Start(startedAt); err != nil {
			t.Fatal(err)
		}
		expectedFinishedAt := recovered.CountdownEndsAt.Add(10 * time.Second)
		if err := repository.Update(context.Background(), recovered, time.Hour); err != nil {
			t.Fatal(err)
		}

		hub := NewHub(repository, time.Hour)
		defer hub.Close()
		host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
		if err != nil {
			t.Fatal(err)
		}
		event := waitForEvent(t, host.Events, "state", 2*time.Second)
		state, ok := event.Payload.(StateView)
		if !ok || state.Phase != game.PhaseFinished || state.CountdownEndsAt != nil || state.QuestionClosesAt != nil {
			t.Fatalf("unexpected recovered final state: %#v", event.Payload)
		}
		persisted, err := repository.GetByID(context.Background(), created.Game.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !persisted.LastActivity.Equal(expectedFinishedAt) {
			t.Fatalf("finished at %s, want %s", persisted.LastActivity, expectedFinishedAt)
		}
	})

	t.Run("lobby and scoreboard remain idle", func(t *testing.T) {
		for _, phase := range []game.Phase{game.PhaseLobby, game.PhaseScoreboard} {
			t.Run(string(phase), func(t *testing.T) {
				repository := &repositoryStub{}
				service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
				created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
				if err != nil {
					t.Fatal(err)
				}
				recovered := created.Game
				if phase == game.PhaseScoreboard {
					startedAt := time.Now().UTC().Add(-time.Minute)
					if _, err := recovered.AddPlayer("player-1", "Alice", "ticket-hash", startedAt); err != nil {
						t.Fatal(err)
					}
					if err := recovered.Start(startedAt); err != nil {
						t.Fatal(err)
					}
					openedAt := *recovered.CountdownEndsAt
					if err := recovered.OpenCurrentQuestion(openedAt); err != nil {
						t.Fatal(err)
					}
					if err := recovered.CloseQuestion(*recovered.QuestionClosesAt); err != nil {
						t.Fatal(err)
					}
				}
				if err := repository.Update(context.Background(), recovered, time.Hour); err != nil {
					t.Fatal(err)
				}
				updatesBeforeRecovery := repository.updateCount()

				hub := NewHub(repository, time.Hour)
				host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
				if err != nil {
					hub.Close()
					t.Fatal(err)
				}
				event := waitForEvent(t, host.Events, "state", 2*time.Second)
				state, ok := event.Payload.(StateView)
				if !ok || state.Phase != phase {
					hub.Close()
					t.Fatalf("unexpected restored phase: %#v", event.Payload)
				}
				time.Sleep(150 * time.Millisecond)
				hub.Close()
				if updates := repository.updateCount(); updates != updatesBeforeRecovery {
					t.Fatalf("%s scheduled an unexpected transition", phase)
				}
			})
		}
	})
}

func waitForEvent(t *testing.T, events <-chan OutboundEvent, eventType string, timeout time.Duration) OutboundEvent {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("event stream closed before %q", eventType)
			}
			if event.Type == eventType {
				return event
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for %q", eventType)
		}
	}
}

func waitForClosed(t *testing.T, events <-chan OutboundEvent, timeout time.Duration) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("received an event while waiting for event stream to close")
		}
	case <-timer.C:
		t.Fatal("timed out waiting for event stream to close")
	}
}

func waitForDone(t *testing.T, done <-chan struct{}, timeout time.Duration) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
		t.Fatal("timed out waiting for session to end")
	}
}
