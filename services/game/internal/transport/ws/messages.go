// Package ws is the WebSocket boundary. The game loop and domain types stay
// independent from the concrete socket library and wire protocol.
package ws

import (
	"encoding/json"
	"time"
)

type ClientMessage struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type ServerMessage struct {
	Type       string    `json:"type"`
	RequestID  string    `json:"request_id,omitempty"`
	Sequence   uint64    `json:"sequence"`
	ServerTime time.Time `json:"server_time"`
	Payload    any       `json:"payload,omitempty"`
}
