package game

// Phase is the authoritative lifecycle state of a game session.
type Phase string

const (
	PhaseLobby          Phase = "lobby"
	PhaseCountdown      Phase = "countdown"
	PhaseQuestionOpen   Phase = "question_open"
	PhaseQuestionClosed Phase = "question_closed"
	PhaseScoreboard     Phase = "scoreboard"
	PhaseFinished       Phase = "finished"
)
