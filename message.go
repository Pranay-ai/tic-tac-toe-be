package main

type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type MovePayload struct {
	GameID string `json:"gameId"`
	Index  int    `json:"index"`
}

type FindMatchPayload struct {
	PlayerID   string `json:"playerId"`
	PlayerName string `json.com:"playerName"`
}

type ReconnectPayload struct {
	PlayerID string `json:"playerId"`
	GameID   string `json:"gameId"`
}
