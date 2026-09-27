// Package ws is the boundary for the future WebSocket protocol. The game loop
// and domain types must stay independent from a concrete socket library.
package ws

type ClientMessage struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Payload   any    `json:"payload,omitempty"`
}

type ServerMessage struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Sequence  uint64 `json:"sequence"`
	Payload   any    `json:"payload,omitempty"`
}
