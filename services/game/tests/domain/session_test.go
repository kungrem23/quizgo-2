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
