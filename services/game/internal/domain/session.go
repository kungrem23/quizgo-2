package game

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidQuizSnapshot = errors.New("invalid quiz snapshot")
	ErrJoinCodeTaken       = errors.New("join code is already in use")
	ErrGameNotFound        = errors.New("game not found")
	ErrInvalidPhase        = errors.New("command is not allowed in the current phase")
	ErrInvalidPlayer       = errors.New("invalid player")
	ErrNicknameTaken       = errors.New("nickname is already in use")
	ErrAlreadyAnswered     = errors.New("player already answered this question")
	ErrInvalidAnswer       = errors.New("invalid answer")
	ErrQuestionClosed      = errors.New("question is closed")
	ErrCountdownActive     = errors.New("question countdown is still active")
	ErrUnauthorized        = errors.New("invalid participant credentials")
	ErrInvalidRequestID    = errors.New("invalid request id")
)

const CountdownDuration = 3 * time.Second

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

// CommandReceipt records an authenticated command that was already applied.
// Receipts are persisted with the game so retries remain idempotent after a
// reconnect or process restart.
type CommandReceipt struct {
	ParticipantID string `json:"participant_id"`
	Command       string `json:"command"`
	RequestID     string `json:"request_id"`
}

type Game struct {
	ID                   string           `json:"id"`
	Code                 string           `json:"code"`
	HostUserID           int64            `json:"host_user_id"`
	HostTicketHash       string           `json:"host_ticket_hash"`
	Quiz                 QuizSnapshot     `json:"quiz"`
	Players              []Player         `json:"players"`
	Submissions          []Submission     `json:"submissions"`
	CommandReceipts      []CommandReceipt `json:"command_receipts,omitempty"`
	Phase                Phase            `json:"phase"`
	CurrentQuestionIndex int              `json:"current_question_index"`
	CountdownEndsAt      *time.Time       `json:"countdown_ends_at,omitempty"`
	QuestionOpenedAt     *time.Time       `json:"question_opened_at,omitempty"`
	QuestionClosesAt     *time.Time       `json:"question_closes_at,omitempty"`
	Sequence             uint64           `json:"sequence"`
	CreatedAt            time.Time        `json:"created_at"`
	LastActivity         time.Time        `json:"last_activity"`
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
		Players: []Player{}, Submissions: []Submission{}, Phase: PhaseLobby,
		CurrentQuestionIndex: -1, Sequence: 1, CreatedAt: now.UTC(), LastActivity: now.UTC(),
	}, nil
}

func (g *Game) SetHostTicketHash(hash string) error {
	if g == nil || strings.TrimSpace(hash) == "" {
		return ErrUnauthorized
	}
	g.HostTicketHash = hash
	return nil
}

func (g *Game) AddPlayer(id, nickname, ticketHash string, now time.Time) (Player, error) {
	if g == nil || g.Phase != PhaseLobby {
		return Player{}, ErrInvalidPhase
	}
	nickname = strings.TrimSpace(nickname)
	if id == "" || ticketHash == "" || len([]rune(nickname)) < 1 || len([]rune(nickname)) > 30 || len(g.Players) >= 100 {
		return Player{}, ErrInvalidPlayer
	}
	for _, player := range g.Players {
		if strings.EqualFold(player.Nickname, nickname) {
			return Player{}, ErrNicknameTaken
		}
	}
	player := Player{ID: id, Nickname: nickname, TicketHash: ticketHash}
	g.Players = append(g.Players, player)
	g.touch(now)
	return player, nil
}

func (g *Game) RemovePlayer(playerID string, now time.Time) (Player, error) {
	if g == nil || g.Phase != PhaseLobby {
		return Player{}, ErrInvalidPhase
	}
	for index, player := range g.Players {
		if player.ID != playerID {
			continue
		}
		g.Players = append(g.Players[:index], g.Players[index+1:]...)
		g.touch(now)
		return player, nil
	}
	return Player{}, ErrInvalidPlayer
}

func (g *Game) Start(now time.Time) error {
	if g == nil || g.Phase != PhaseLobby || len(g.Players) == 0 {
		return ErrInvalidPhase
	}
	g.CurrentQuestionIndex = 0
	g.startCountdown(now)
	return nil
}

// OpenCurrentQuestion completes the countdown and opens the selected question.
func (g *Game) OpenCurrentQuestion(now time.Time) error {
	if g == nil || g.Phase != PhaseCountdown || g.CountdownEndsAt == nil || g.CurrentQuestionIndex < 0 || g.CurrentQuestionIndex >= len(g.Quiz.Questions) {
		return ErrInvalidPhase
	}
	if now.Before(*g.CountdownEndsAt) {
		return ErrCountdownActive
	}
	g.openCurrentQuestion(now)
	return nil
}

func (g *Game) SubmitAnswer(playerID string, answerID int64, now time.Time) (Submission, error) {
	if g == nil || g.Phase != PhaseQuestionOpen || g.CurrentQuestionIndex < 0 || g.CurrentQuestionIndex >= len(g.Quiz.Questions) {
		return Submission{}, ErrInvalidPhase
	}
	if g.QuestionClosesAt == nil || !now.Before(*g.QuestionClosesAt) {
		return Submission{}, ErrQuestionClosed
	}
	playerIndex := -1
	for index := range g.Players {
		if g.Players[index].ID == playerID {
			playerIndex = index
			break
		}
	}
	if playerIndex < 0 {
		return Submission{}, ErrInvalidPlayer
	}
	question := g.Quiz.Questions[g.CurrentQuestionIndex]
	for _, submission := range g.Submissions {
		if submission.PlayerID == playerID && submission.QuestionID == question.ID {
			return Submission{}, ErrAlreadyAnswered
		}
	}
	var selected *Answer
	for index := range question.Answers {
		if question.Answers[index].ID == answerID {
			selected = &question.Answers[index]
			break
		}
	}
	if selected == nil {
		return Submission{}, ErrInvalidAnswer
	}
	score := int64(0)
	if selected.IsCorrect {
		score = g.score(now)
		g.Players[playerIndex].Score += score
	}
	submission := Submission{
		PlayerID: playerID, QuestionID: question.ID, AnswerID: answerID,
		IsCorrect: selected.IsCorrect, ScoreAwarded: score, AnsweredAt: now.UTC(),
	}
	g.Submissions = append(g.Submissions, submission)
	g.touch(now)
	return submission, nil
}

// AllPlayersAnswered reports whether every player in the game has submitted an
// answer for the currently open question.
func (g *Game) AllPlayersAnswered() bool {
	if g == nil || g.Phase != PhaseQuestionOpen || len(g.Players) == 0 {
		return false
	}
	question, ok := g.CurrentQuestion()
	if !ok {
		return false
	}
	answered := make(map[string]struct{}, len(g.Players))
	for _, submission := range g.Submissions {
		if submission.QuestionID == question.ID {
			answered[submission.PlayerID] = struct{}{}
		}
	}
	for _, player := range g.Players {
		if _, ok := answered[player.ID]; !ok {
			return false
		}
	}
	return true
}

func (g *Game) CloseQuestion(now time.Time) error {
	if g == nil || g.Phase != PhaseQuestionOpen {
		return ErrInvalidPhase
	}
	if g.CurrentQuestionIndex+1 >= len(g.Quiz.Questions) {
		g.Phase = PhaseFinished
		g.QuestionOpenedAt = nil
	} else {
		g.Phase = PhaseScoreboard
	}
	g.QuestionClosesAt = nil
	g.touch(now)
	return nil
}

func (g *Game) Finish(now time.Time) error {
	if g == nil {
		return ErrInvalidPhase
	}
	switch g.Phase {
	case PhaseLobby, PhaseCountdown, PhaseQuestionOpen, PhaseQuestionClosed, PhaseScoreboard:
	default:
		return ErrInvalidPhase
	}
	g.Phase = PhaseFinished
	g.CountdownEndsAt = nil
	g.QuestionOpenedAt = nil
	g.QuestionClosesAt = nil
	g.touch(now)
	return nil
}

// Next starts the next question's countdown or finishes the game. It returns
// true when the game has reached its terminal state.
func (g *Game) Next(now time.Time) (bool, error) {
	if g == nil || (g.Phase != PhaseScoreboard && g.Phase != PhaseQuestionClosed) {
		return false, ErrInvalidPhase
	}
	if g.CurrentQuestionIndex+1 >= len(g.Quiz.Questions) {
		g.Phase = PhaseFinished
		g.CountdownEndsAt = nil
		g.QuestionOpenedAt = nil
		g.QuestionClosesAt = nil
		g.touch(now)
		return true, nil
	}
	g.CurrentQuestionIndex++
	g.startCountdown(now)
	return false, nil
}

func (g *Game) CurrentQuestion() (Question, bool) {
	if g == nil || g.CurrentQuestionIndex < 0 || g.CurrentQuestionIndex >= len(g.Quiz.Questions) {
		return Question{}, false
	}
	return g.Quiz.Questions[g.CurrentQuestionIndex], true
}

func (g *Game) startCountdown(now time.Time) {
	ends := now.UTC().Add(CountdownDuration)
	g.Phase = PhaseCountdown
	g.CountdownEndsAt = &ends
	g.QuestionOpenedAt = nil
	g.QuestionClosesAt = nil
	g.touch(now)
}

func (g *Game) openCurrentQuestion(now time.Time) {
	opened := now.UTC()
	closes := opened.Add(time.Duration(g.Quiz.Questions[g.CurrentQuestionIndex].TimeLimitSeconds) * time.Second)
	g.Phase = PhaseQuestionOpen
	g.CountdownEndsAt = nil
	g.QuestionOpenedAt = &opened
	g.QuestionClosesAt = &closes
	g.touch(now)
}

func (g *Game) touch(now time.Time) {
	g.Sequence++
	g.LastActivity = now.UTC()
}

func (g *Game) score(now time.Time) int64 {
	if g.QuestionOpenedAt == nil || g.QuestionClosesAt == nil {
		return 500
	}
	total := g.QuestionClosesAt.Sub(*g.QuestionOpenedAt)
	remaining := g.QuestionClosesAt.Sub(now)
	if total <= 0 || remaining <= 0 {
		return 500
	}
	bonus := int64(500 * remaining / total)
	if bonus < 0 {
		bonus = 0
	}
	if bonus > 500 {
		bonus = 500
	}
	return 500 + bonus
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
		if correct != 1 {
			return ErrInvalidQuizSnapshot
		}
	}
	return nil
}
