package game

import "time"

// Event is the envelope that will be persisted and broadcast by the realtime loop.
// Payload stays transport-neutral; WebSocket DTOs belong to transport/ws.
type Event struct {
	GameID    string    `json:"game_id"`
	Sequence  uint64    `json:"sequence"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	Payload   any       `json:"payload,omitempty"`
}
