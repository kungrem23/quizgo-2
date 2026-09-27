package application_test

import . "github.com/kungrem23/quizgo/services/game/internal/application"

import (
	"context"
	"testing"
	"time"

	game "github.com/kungrem23/quizgo/services/game/internal/domain"
)

type catalogStub struct{ snapshot game.QuizSnapshot }

func (c catalogStub) GetPlayableQuiz(context.Context, int64, string) (game.QuizSnapshot, error) {
	return c.snapshot, nil
}

type repositoryStub struct{ saved game.Game }

func (r *repositoryStub) Save(_ context.Context, value game.Game, _ time.Duration) error {
	r.saved = value
	return nil
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
	if created.ID == "" || len(created.Code) != 6 || created.HostUserID != 42 || created.Quiz.Revision != 2 || repository.saved.ID != created.ID {
		t.Fatalf("unexpected game: %#v", created)
	}
}
