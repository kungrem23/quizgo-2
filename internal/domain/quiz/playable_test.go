package quiz

import (
	"context"
	"errors"
	"testing"
)

type playableRepoStub struct {
	Repository
	content QuizContent
	err     error
}

func (r playableRepoStub) GetQuizContent(context.Context, int) (QuizContent, error) {
	return r.content, r.err
}

func validPlayableQuiz() QuizContent {
	return QuizContent{
		QuizSummary: QuizSummary{ID: 7, Title: "Geography", AuthorID: 42, Revision: 3},
		Questions: []ContentQuestion{{
			ID: 11, TextContent: "Capital?", ImageID: "image-1", ImageURL: "https://signed.example", TimeLimit: 20,
			Answers: []ContentAnswer{{ID: 21, TextContent: "Paris", IsCorrect: true}, {ID: 22, TextContent: "Rome"}},
		}},
	}
}

func TestGetPlayableQuizReturnsSnapshotWithoutExpiringURL(t *testing.T) {
	service := NewService(playableRepoStub{content: validPlayableQuiz()})
	got, err := service.GetPlayableQuiz(context.Background(), 7, 42)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != 3 || got.Questions[0].ImageID != "image-1" || got.Questions[0].ImageURL != "" {
		t.Fatalf("unexpected snapshot: %#v", got)
	}
}

func TestGetPlayableQuizRejectsDifferentOwner(t *testing.T) {
	service := NewService(playableRepoStub{content: validPlayableQuiz()})
	if _, err := service.GetPlayableQuiz(context.Background(), 7, 41); !errors.Is(err, ErrNotQuizOwner) {
		t.Fatalf("error = %v", err)
	}
}

func TestGetPlayableQuizRejectsEmptyQuiz(t *testing.T) {
	content := validPlayableQuiz()
	content.Questions = nil
	service := NewService(playableRepoStub{content: content})
	if _, err := service.GetPlayableQuiz(context.Background(), 7, 42); !errors.Is(err, ErrQuizNotPlayable) {
		t.Fatalf("error = %v", err)
	}
}
