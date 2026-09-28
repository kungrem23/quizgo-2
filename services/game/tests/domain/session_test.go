package game_test

import . "github.com/kungrem23/quizgo/services/game/internal/domain"

import (
	"errors"
	"testing"
	"time"
)

func validSnapshot() QuizSnapshot {
	return QuizSnapshot{
		ID: 1, Revision: 2, Title: "Go", OwnerUserID: 7,
		Questions: []Question{{
			ID: 3, Text: "What is a goroutine?", TimeLimitSeconds: 20,
			Answers: []Answer{{ID: 4, Text: "A lightweight task", IsCorrect: true}, {ID: 5, Text: "A database"}},
		}},
	}
}

func TestNewGameStartsInLobby(t *testing.T) {
	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.FixedZone("test", 3*60*60))
	game, err := NewGame("game-1", "ABC123", 7, validSnapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	if game.Phase != PhaseLobby || game.Sequence != 1 || game.CreatedAt.Location() != time.UTC {
		t.Fatalf("unexpected game: %#v", game)
	}
}

func TestNewGameRejectsQuizWithoutCorrectAnswer(t *testing.T) {
	snapshot := validSnapshot()
	snapshot.Questions[0].Answers[0].IsCorrect = false
	_, err := NewGame("game-1", "ABC123", 7, snapshot, time.Now())
	if !errors.Is(err, ErrInvalidQuizSnapshot) {
		t.Fatalf("error = %v", err)
	}
}

func TestNewGameRejectsQuizWithMultipleCorrectAnswers(t *testing.T) {
	snapshot := validSnapshot()
	snapshot.Questions[0].Answers[1].IsCorrect = true
	_, err := NewGame("game-1", "ABC123", 7, snapshot, time.Now())
	if !errors.Is(err, ErrInvalidQuizSnapshot) {
		t.Fatalf("error = %v", err)
	}
}

func TestGameLifecycleScoresOnlyFirstAnswer(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	session, err := NewGame("game-1", "ABC123", 7, validSnapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.AddPlayer("player-1", "Alice", "ticket-hash", now); err != nil {
		t.Fatal(err)
	}
	if err := session.Start(now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if session.Phase != PhaseCountdown || session.CountdownEndsAt == nil || !session.CountdownEndsAt.Equal(now.Add(time.Second+CountdownDuration)) {
		t.Fatalf("unexpected countdown state: %#v", session)
	}
	if _, err := session.SubmitAnswer("player-1", 4, now.Add(2*time.Second)); !errors.Is(err, ErrInvalidPhase) {
		t.Fatalf("answer during countdown error = %v", err)
	}
	if err := session.OpenCurrentQuestion(now.Add(2 * time.Second)); !errors.Is(err, ErrCountdownActive) {
		t.Fatalf("early countdown completion error = %v", err)
	}
	openedAt := *session.CountdownEndsAt
	if err := session.OpenCurrentQuestion(openedAt); err != nil {
		t.Fatal(err)
	}
	if session.Phase != PhaseQuestionOpen || session.CountdownEndsAt != nil {
		t.Fatalf("unexpected open question state: %#v", session)
	}
	submission, err := session.SubmitAnswer("player-1", 4, openedAt.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !submission.IsCorrect || submission.ScoreAwarded < 500 || session.Players[0].Score != submission.ScoreAwarded {
		t.Fatalf("unexpected submission: %#v player=%#v", submission, session.Players[0])
	}
	if _, err := session.SubmitAnswer("player-1", 5, openedAt.Add(2*time.Second)); !errors.Is(err, ErrAlreadyAnswered) {
		t.Fatalf("second answer error = %v", err)
	}
	if err := session.CloseQuestion(openedAt.Add(20 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if session.Phase != PhaseScoreboard {
		t.Fatalf("phase after close = %s", session.Phase)
	}
	finished, err := session.Next(openedAt.Add(21 * time.Second))
	if err != nil || !finished || session.Phase != PhaseFinished {
		t.Fatalf("finished=%v phase=%s err=%v", finished, session.Phase, err)
	}
}

func TestNextStartsCountdownForFollowingQuestion(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	snapshot := validSnapshot()
	second := snapshot.Questions[0]
	second.ID = 6
	second.Text = "What is a channel?"
	second.Answers = []Answer{{ID: 7, Text: "A communication primitive", IsCorrect: true}, {ID: 8, Text: "A database"}}
	snapshot.Questions = append(snapshot.Questions, second)
	session, err := NewGame("game-1", "ABC123", 7, snapshot, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.AddPlayer("player-1", "Alice", "ticket-hash", now); err != nil {
		t.Fatal(err)
	}
	if err := session.Start(now); err != nil {
		t.Fatal(err)
	}
	if err := session.OpenCurrentQuestion(now.Add(CountdownDuration)); err != nil {
		t.Fatal(err)
	}
	if err := session.CloseQuestion(now.Add(CountdownDuration + 20*time.Second)); err != nil {
		t.Fatal(err)
	}

	nextAt := now.Add(CountdownDuration + 21*time.Second)
	finished, err := session.Next(nextAt)
	if err != nil {
		t.Fatal(err)
	}
	if finished || session.Phase != PhaseCountdown || session.CurrentQuestionIndex != 1 || session.CountdownEndsAt == nil || !session.CountdownEndsAt.Equal(nextAt.Add(CountdownDuration)) {
		t.Fatalf("unexpected next-question state: %#v", session)
	}
}

func TestGameRejectsDuplicateNicknameCaseInsensitively(t *testing.T) {
	session, err := NewGame("game-1", "ABC123", 7, validSnapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.AddPlayer("player-1", "Alice", "hash-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := session.AddPlayer("player-2", " alice ", "hash-2", time.Now()); !errors.Is(err, ErrNicknameTaken) {
		t.Fatalf("error = %v", err)
	}
}
