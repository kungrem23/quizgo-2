package application_test

import . "github.com/kungrem23/quizgo/services/game/internal/application"

import (
	"context"
	"errors"
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
	mu    sync.Mutex
	saved game.Game
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
	if _, err := hub.AuthenticatePlayer(context.Background(), created.Game.ID, joined.Player.ID, joined.Ticket); err != nil {
		t.Fatalf("reconnect player: %v", err)
	}
	host, err := hub.AuthenticateHost(context.Background(), created.Game.ID, created.HostTicket)
	if err != nil {
		t.Fatal(err)
	}
	if err := hub.Dispatch(context.Background(), joined.Session, "start", 0); !errors.Is(err, game.ErrUnauthorized) {
		t.Fatalf("player start error = %v", err)
	}
	if err := hub.Dispatch(context.Background(), host, "start", 0); err != nil {
		t.Fatal(err)
	}
	countdown := waitForEvent(t, joined.Session.Events, "countdown_started", 2*time.Second)
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
	waitForEvent(t, joined.Session.Events, "question_opened", 5*time.Second)
	if err := hub.Dispatch(context.Background(), joined.Session, "answer", 1); err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, joined.Session.Events, "answer_accepted", 2*time.Second)
	waitForEvent(t, joined.Session.Events, "question_closed", 3*time.Second)
	reconnectedAtScoreboard, err := hub.AuthenticatePlayer(context.Background(), created.Game.ID, joined.Player.ID, joined.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	reconnectedScoreboardState := waitForEvent(t, reconnectedAtScoreboard.Events, "state", 2*time.Second)
	if state, ok := reconnectedScoreboardState.Payload.(StateView); !ok || state.Phase != game.PhaseScoreboard || len(state.CorrectAnswerIDs) != 1 || state.CorrectAnswerIDs[0] != 1 {
		t.Fatalf("unexpected reconnected scoreboard state: %#v", reconnectedScoreboardState.Payload)
	}
	if err := hub.Dispatch(context.Background(), host, "next", 0); err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, joined.Session.Events, "game_finished", 2*time.Second)

	persisted, err := repository.GetByID(context.Background(), created.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Phase != game.PhaseFinished || len(persisted.Submissions) != 1 || persisted.Players[0].Score < 500 {
		t.Fatalf("unexpected persisted game: %#v", persisted)
	}
}

func TestHubRecoversPersistedTimedPhases(t *testing.T) {
	snapshot := game.QuizSnapshot{
		ID: 8, Revision: 2, Title: "Go", OwnerUserID: 42,
		Questions: []game.Question{{
			ID: 1, Text: "Q", TimeLimitSeconds: 10,
			Answers: []game.Answer{{ID: 1, Text: "A", IsCorrect: true}, {ID: 2, Text: "B"}},
		}},
	}

	t.Run("expired countdown opens question before reconnect state", func(t *testing.T) {
		repository := &repositoryStub{}
		service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
		created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
		if err != nil {
			t.Fatal(err)
		}
		recovered := created.Game
		startedAt := time.Now().Add(-game.CountdownDuration - time.Second)
		if _, err := recovered.AddPlayer("player-1", "Alice", "ticket-hash", startedAt); err != nil {
			t.Fatal(err)
		}
		if err := recovered.Start(startedAt); err != nil {
			t.Fatal(err)
		}
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
		if !ok || state.Phase != game.PhaseQuestionOpen || state.CountdownEndsAt != nil || state.QuestionClosesAt == nil || state.CurrentQuestion == nil {
			t.Fatalf("unexpected recovered state: %#v", event.Payload)
		}
	})

	t.Run("scoreboard includes results after reconnect", func(t *testing.T) {
		repository := &repositoryStub{}
		service := New(catalogStub{snapshot: snapshot}, repository, time.Hour)
		created, err := service.CreateGame(context.Background(), CreateGameRequest{QuizID: 8, AccessToken: "token"})
		if err != nil {
			t.Fatal(err)
		}
		recovered := created.Game
		startedAt := time.Now().Add(-time.Minute)
		if _, err := recovered.AddPlayer("player-1", "Alice", "ticket-hash", startedAt); err != nil {
			t.Fatal(err)
		}
		if err := recovered.Start(startedAt); err != nil {
			t.Fatal(err)
		}
		if err := recovered.OpenCurrentQuestion(startedAt.Add(game.CountdownDuration)); err != nil {
			t.Fatal(err)
		}
		if err := recovered.CloseQuestion(startedAt.Add(game.CountdownDuration + 10*time.Second)); err != nil {
			t.Fatal(err)
		}
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
		if !ok || state.Phase != game.PhaseScoreboard || len(state.CorrectAnswerIDs) != 1 || state.CorrectAnswerIDs[0] != 1 || len(state.Players) != 1 {
			t.Fatalf("unexpected recovered scoreboard: %#v", event.Payload)
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
