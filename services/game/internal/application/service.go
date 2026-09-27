package application

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	game "github.com/kungrem23/quizgo/services/game/internal/domain"
)

type QuizCatalog interface {
	GetPlayableQuiz(context.Context, int64, string) (game.QuizSnapshot, error)
}

type GameRepository interface {
	Save(context.Context, game.Game, time.Duration) error
}

type Service struct {
	catalog QuizCatalog
	games   GameRepository
	ttl     time.Duration
	now     func() time.Time
	newID   func() string
	newCode func() (string, error)
}

type CreateGameRequest struct {
	QuizID      int64
	AccessToken string
}

func New(catalog QuizCatalog, games GameRepository, ttl time.Duration) *Service {
	return &Service{
		catalog: catalog, games: games, ttl: ttl, now: time.Now,
		newID: uuid.NewString, newCode: randomJoinCode,
	}
}

func (s *Service) CreateGame(ctx context.Context, request CreateGameRequest) (game.Game, error) {
	if request.QuizID < 1 || request.AccessToken == "" {
		return game.Game{}, errors.New("quiz id and access token are required")
	}
	snapshot, err := s.catalog.GetPlayableQuiz(ctx, request.QuizID, request.AccessToken)
	if err != nil {
		return game.Game{}, fmt.Errorf("load playable quiz: %w", err)
	}
	for attempt := 0; attempt < 5; attempt++ {
		code, err := s.newCode()
		if err != nil {
			return game.Game{}, fmt.Errorf("generate join code: %w", err)
		}
		created, err := game.NewGame(s.newID(), code, snapshot.OwnerUserID, snapshot, s.now())
		if err != nil {
			return game.Game{}, err
		}
		if err := s.games.Save(ctx, created, s.ttl); err != nil {
			if errors.Is(err, game.ErrJoinCodeTaken) {
				continue
			}
			return game.Game{}, fmt.Errorf("save game: %w", err)
		}
		return created, nil
	}
	return game.Game{}, errors.New("could not allocate a unique join code")
}

func randomJoinCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for index := range random {
		random[index] = alphabet[int(random[index])%len(alphabet)]
	}
	return string(random), nil
}
