package game

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidQuizSnapshot = errors.New("invalid quiz snapshot")
	ErrJoinCodeTaken       = errors.New("join code is already in use")
)

type QuizSnapshot struct {
	ID          int64      `json:"id"`
	Revision    int64      `json:"revision"`
	Title       string     `json:"title"`
	OwnerUserID int64      `json:"owner_user_id"`
	Questions   []Question `json:"questions"`
}

type Question struct {
	ID               int64    `json:"id"`
	Text             string   `json:"text"`
	ImageID          string   `json:"image_id,omitempty"`
	TimeLimitSeconds uint32   `json:"time_limit_seconds"`
	Answers          []Answer `json:"answers"`
}

type Answer struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	IsCorrect bool   `json:"is_correct"`
}

type Game struct {
	ID           string       `json:"id"`
	Code         string       `json:"code"`
	HostUserID   int64        `json:"host_user_id"`
	Quiz         QuizSnapshot `json:"quiz"`
	Phase        Phase        `json:"phase"`
	Sequence     uint64       `json:"sequence"`
	CreatedAt    time.Time    `json:"created_at"`
	LastActivity time.Time    `json:"last_activity"`
}

func NewGame(id, code string, hostUserID int64, snapshot QuizSnapshot, now time.Time) (Game, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(code) == "" || hostUserID < 1 {
		return Game{}, errors.New("invalid game identity")
	}
	if err := snapshot.Validate(); err != nil {
		return Game{}, err
	}
	return Game{
		ID: id, Code: code, HostUserID: hostUserID, Quiz: snapshot,
		Phase: PhaseLobby, Sequence: 1, CreatedAt: now.UTC(), LastActivity: now.UTC(),
	}, nil
}

func (q QuizSnapshot) Validate() error {
	if q.ID < 1 || q.Revision < 1 || q.OwnerUserID < 1 || strings.TrimSpace(q.Title) == "" || len(q.Questions) == 0 {
		return ErrInvalidQuizSnapshot
	}
	for _, question := range q.Questions {
		if question.ID < 1 || strings.TrimSpace(question.Text) == "" || question.TimeLimitSeconds == 0 || len(question.Answers) < 2 {
			return ErrInvalidQuizSnapshot
		}
		correct := 0
		for _, answer := range question.Answers {
			if answer.ID < 1 || strings.TrimSpace(answer.Text) == "" {
				return ErrInvalidQuizSnapshot
			}
			if answer.IsCorrect {
				correct++
			}
		}
		if correct == 0 {
			return ErrInvalidQuizSnapshot
		}
	}
	return nil
}
