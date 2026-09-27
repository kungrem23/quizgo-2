package quiz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrContentInvalid = errors.New("invalid quiz content")
var ErrContentConflict = errors.New("quiz has changed")
var ErrImageInUse = errors.New("image is in use")
var ErrQuizNotPlayable = errors.New("quiz is not playable")
var ErrNotQuizOwner = errors.New("quiz belongs to another user")

type QuizSummary struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	AuthorID      int       `json:"author_id"`
	QuestionCount int       `json:"question_count"`
	UpdatedAt     time.Time `json:"updated_at"`
	Revision      int64     `json:"revision"`
}

type QuizContent struct {
	QuizSummary
	Questions []ContentQuestion `json:"questions"`
}

type ContentQuestion struct {
	ID          int             `json:"id"`
	TextContent string          `json:"text_content"`
	ImageID     string          `json:"image_id"`
	ImageURL    string          `json:"image_url,omitempty" readonly:"true"`
	TimeLimit   int             `json:"time_limit"`
	Answers     []ContentAnswer `json:"answers"`
}

type ContentAnswer struct {
	ID          int    `json:"id"`
	TextContent string `json:"text_content"`
	IsCorrect   bool   `json:"is_correct"`
}

func (v QuizContent) ValidatePlayable() error {
	if strings.TrimSpace(v.Title) == "" || len(v.Questions) == 0 || len(v.Questions) > 100 {
		return ErrQuizNotPlayable
	}
	for _, question := range v.Questions {
		if question.ID < 1 || strings.TrimSpace(question.TextContent) == "" || len(question.Answers) < 2 || len(question.Answers) > 8 {
			return ErrQuizNotPlayable
		}
		switch question.TimeLimit {
		case 5, 10, 15, 20, 30, 45, 60, 90, 120:
		default:
			return ErrQuizNotPlayable
		}
		correct := 0
		for _, answer := range question.Answers {
			if answer.ID < 1 || strings.TrimSpace(answer.TextContent) == "" {
				return ErrQuizNotPlayable
			}
			if answer.IsCorrect {
				correct++
			}
		}
		if correct != 1 {
			return ErrQuizNotPlayable
		}
	}
	return nil
}

// Array order is the persisted order; zero IDs represent new items.
type SaveQuizContent struct {
	Title     string            `json:"title"`
	Revision  int64             `json:"revision"`
	Questions []ContentQuestion `json:"questions"`
}

func (v *SaveQuizContent) Validate() error {
	v.Title = strings.TrimSpace(v.Title)
	if v.Title == "" || utf8.RuneCountInString(v.Title) > 50 || v.Revision < 1 || v.Questions == nil || len(v.Questions) > 100 {
		return fmt.Errorf("%w: title, revision or questions", ErrContentInvalid)
	}
	questions, answers := map[int]bool{}, map[int]bool{}
	for i := range v.Questions {
		q := &v.Questions[i]
		q.TextContent = strings.TrimSpace(q.TextContent)
		if q.ID < 0 || (q.ID > 0 && questions[q.ID]) || q.TextContent == "" || utf8.RuneCountInString(q.TextContent) > 200 || len(q.ImageID) > 40 {
			return fmt.Errorf("%w: question %d", ErrContentInvalid, i+1)
		}
		questions[q.ID] = true
		switch q.TimeLimit {
		case 5, 10, 15, 20, 30, 45, 60, 90, 120:
		default:
			return fmt.Errorf("%w: time limit", ErrContentInvalid)
		}
		if len(q.Answers) < 2 || len(q.Answers) > 8 {
			return fmt.Errorf("%w: 2–8 answers required", ErrContentInvalid)
		}
		correct := 0
		for j := range q.Answers {
			a := &q.Answers[j]
			a.TextContent = strings.TrimSpace(a.TextContent)
			if a.ID < 0 || (a.ID > 0 && answers[a.ID]) || a.TextContent == "" || utf8.RuneCountInString(a.TextContent) > 200 {
				return fmt.Errorf("%w: answer", ErrContentInvalid)
			}
			answers[a.ID] = true
			if a.IsCorrect {
				correct++
			}
		}
		if correct != 1 {
			return fmt.Errorf("%w: exactly one correct answer required", ErrContentInvalid)
		}
	}
	return nil
}

type UploadedImage struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func (s *Service) GetQuizContent(ctx context.Context, id int) (QuizContent, error) {
	v, err := s.repo.GetQuizContent(ctx, id)
	if err != nil {
		return v, err
	}
	return v, s.resolveImages(ctx, v.Questions)
}

// GetPlayableQuiz returns a database-consistent snapshot for a new game. Images
// remain references by ID so the snapshot never embeds an expiring signed URL.
func (s *Service) GetPlayableQuiz(ctx context.Context, id, userID int) (QuizContent, error) {
	v, err := s.repo.GetQuizContent(ctx, id)
	if err != nil {
		return v, err
	}
	if v.AuthorID != userID {
		return QuizContent{}, ErrNotQuizOwner
	}
	if err := v.ValidatePlayable(); err != nil {
		return QuizContent{}, err
	}
	for i := range v.Questions {
		v.Questions[i].ImageURL = ""
	}
	return v, nil
}
func (s *Service) ListQuizSummaries(ctx context.Context, userID int) ([]QuizSummary, error) {
	return s.repo.ListQuizSummaries(ctx, userID)
}
func (s *Service) SaveQuizContent(ctx context.Context, id, userID int, content SaveQuizContent) (QuizContent, error) {
	if err := content.Validate(); err != nil {
		return QuizContent{}, err
	}
	// Resolve before writing: a signing failure must not report a failed save
	// after the quiz has already committed.
	if err := s.resolveImages(ctx, content.Questions); err != nil {
		return QuizContent{}, err
	}
	v, err := s.repo.SaveQuizContent(ctx, id, userID, content)
	if err != nil {
		return v, err
	}
	urls := map[string]string{}
	for _, q := range content.Questions {
		urls[q.ImageID] = q.ImageURL
	}
	for i := range v.Questions {
		v.Questions[i].ImageURL = urls[v.Questions[i].ImageID]
	}
	return v, nil
}
func (s *Service) DeleteQuizAsAuthor(ctx context.Context, id, userID int) error {
	return s.repo.DeleteQuizAsAuthor(ctx, id, userID)
}
