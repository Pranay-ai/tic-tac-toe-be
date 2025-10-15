// message.go
package main

// Message defines the structure for all incoming WebSocket messages.
type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// MovePayload defines the payload for a "move" message.
type MovePayload struct {
	GameID string `json:"gameId"`
	Index  int    `json:"index"`
}

// MatchFoundPayload is sent to two clients when a match is made.
type MatchFoundPayload struct {
	GameID  string `json:"gameId"`
	PlayerX string `json:"playerX"` // Client ID of Player X
	PlayerO string `json:"playerO"` // Client ID of Player O
}
