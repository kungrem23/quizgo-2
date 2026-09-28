package game

import "time"

type Player struct {
	ID         string `json:"id"`
	Nickname   string `json:"nickname"`
	Score      int64  `json:"score"`
	TicketHash string `json:"ticket_hash"`
}

type Submission struct {
	PlayerID     string    `json:"player_id"`
	QuestionID   int64     `json:"question_id"`
	AnswerID     int64     `json:"answer_id"`
	IsCorrect    bool      `json:"is_correct"`
	ScoreAwarded int64     `json:"score_awarded"`
	AnsweredAt   time.Time `json:"answered_at"`
}
