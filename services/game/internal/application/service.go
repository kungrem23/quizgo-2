package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	Update(context.Context, game.Game, time.Duration) error
	GetByID(context.Context, string) (game.Game, error)
	GetByCode(context.Context, string) (game.Game, error)
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

type CreatedGame struct {
	Game       game.Game
	HostTicket string
}

func New(catalog QuizCatalog, games GameRepository, ttl time.Duration) *Service {
	return &Service{
		catalog: catalog, games: games, ttl: ttl, now: time.Now,
		newID: uuid.NewString, newCode: randomJoinCode,
	}
}

func (s *Service) CreateGame(ctx context.Context, request CreateGameRequest) (CreatedGame, error) {
	if request.QuizID < 1 || request.AccessToken == "" {
		return CreatedGame{}, errors.New("quiz id and access token are required")
	}
	snapshot, err := s.catalog.GetPlayableQuiz(ctx, request.QuizID, request.AccessToken)
	if err != nil {
		return CreatedGame{}, fmt.Errorf("load playable quiz: %w", err)
	}
	hostTicket, err := randomTicket()
	if err != nil {
		return CreatedGame{}, fmt.Errorf("generate host ticket: %w", err)
	}
	for attempt := 0; attempt < 5; attempt++ {
		code, err := s.newCode()
		if err != nil {
			return CreatedGame{}, fmt.Errorf("generate join code: %w", err)
		}
		created, err := game.NewGame(s.newID(), code, snapshot.OwnerUserID, snapshot, s.now())
		if err != nil {
			return CreatedGame{}, err
		}
		if err := created.SetHostTicketHash(ticketHash(hostTicket)); err != nil {
			return CreatedGame{}, err
		}
		if err := s.games.Save(ctx, created, s.ttl); err != nil {
			if errors.Is(err, game.ErrJoinCodeTaken) {
				continue
			}
			return CreatedGame{}, fmt.Errorf("save game: %w", err)
		}
		return CreatedGame{Game: created, HostTicket: hostTicket}, nil
	}
	return CreatedGame{}, errors.New("could not allocate a unique join code")
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

func randomTicket() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func ticketHash(ticket string) string {
	hash := sha256.Sum256([]byte(ticket))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
