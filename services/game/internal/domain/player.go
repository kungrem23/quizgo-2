package game

type Player struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Score    int64  `json:"score"`
}
