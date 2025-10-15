package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
	ID         string
	PlayerID   string
	PlayerName string
	GameID     string
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[CLIENT] Error upgrading connection: %v", err)
		return
	}
	client := &Client{
		ID:   uuid.NewString(),
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
	log.Printf("[CLIENT] New WebSocket connection established with ID: %s", client.ID)
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, rawMessage, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[CLIENT] readPump error: %v", err)
			}
			break
		}
		log.Printf("[CLIENT] Received raw message from %s: %s", c.ID, string(rawMessage))

		var msg Message
		if err := json.Unmarshal(rawMessage, &msg); err != nil {
			log.Printf("[CLIENT] Error unmarshalling message: %v", err)
			continue
		}

		log.Printf("[CLIENT] Parsed message type '%s' from client %s", msg.Type, c.ID)
		switch msg.Type {
		case "move":
			handleMove(c, msg.Payload)
		case "find_match":
			handleFindMatch(c, msg.Payload)
		case "get_leaderboard":
			handleGetLeaderboard(c)
		case "reconnect":
			handleReconnect(c, msg.Payload)
		default:
			log.Printf("[CLIENT] Unknown message type received: %s", msg.Type)
		}
	}
}

func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				log.Printf("[CLIENT] writePump: Hub closed channel for client %s.", c.ID)
				return
			}
			log.Printf("[CLIENT] Sending message to client %s (PlayerID: %s): %s", c.ID, c.PlayerID, string(message))
			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func handleReconnect(client *Client, payload interface{}) {
	log.Printf("[RECONNECT] Handling reconnect request...")
	var reconnectPayload ReconnectPayload
	payloadData, _ := json.Marshal(payload)
	json.Unmarshal(payloadData, &reconnectPayload)

	client.PlayerID = reconnectPayload.PlayerID
	client.GameID = reconnectPayload.GameID

	log.Printf("[RECONNECT] Player %s attempting to reconnect to game %s", client.PlayerID, client.GameID)

	game, err := getGame(ctx, client.GameID)
	if err != nil || game == nil {
		log.Printf("[RECONNECT] Failed: Game %s not found.", client.GameID)
		return
	}

	isValidReconnect := (game.PlayerX == client.PlayerID && game.Status == StatusDisconnectedX) ||
		(game.PlayerO == client.PlayerID && game.Status == StatusDisconnectedO)

	if isValidReconnect {
		log.Printf("[RECONNECT] Player %s reconnected successfully to game %s.", client.PlayerID, client.GameID)
		game.Status = StatusPlaying
		saveGame(ctx, game)

		response := Message{Type: "game_update", Payload: game}
		responseJSON, _ := json.Marshal(response)
		channel := "game:" + game.ID
		rdb.Publish(ctx, channel, responseJSON)
	} else {
		log.Printf("[RECONNECT] Invalid reconnect attempt by Player %s for game %s with status %s.", client.PlayerID, client.GameID, game.Status)
	}
}

func handleMove(client *Client, payload interface{}) {
	log.Printf("[MOVE] Handling move request from PlayerID: %s", client.PlayerID)
	moveData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[MOVE] error marshalling move payload: %v", err)
		return
	}
	var move MovePayload
	if err := json.Unmarshal(moveData, &move); err != nil {
		log.Printf("[MOVE] error unmarshalling move payload: %v", err)
		return
	}

	ctx := context.Background()
	game, err := getGame(ctx, move.GameID)
	if err != nil {
		log.Printf("[MOVE] error getting game: %v", err)
		return
	}
	if game == nil {
		log.Printf("[MOVE] game not found: %s", move.GameID)
		return
	}

	var currentPlayerSymbol string
	if client.PlayerID == game.PlayerX {
		currentPlayerSymbol = "X"
	} else if client.PlayerID == game.PlayerO {
		currentPlayerSymbol = "O"
	} else {
		log.Printf("[MOVE] validation failed: client (PlayerID: %s) is not a player in game %s", client.PlayerID, game.ID)
		return
	}

	if game.Status != StatusPlaying {
		log.Printf("[MOVE] validation failed: game is already over (status: %s)", game.Status)
		return
	}
	if game.Turn != currentPlayerSymbol {
		log.Printf("[MOVE] validation failed: not player %s's turn", currentPlayerSymbol)
		return
	}
	if move.Index < 0 || move.Index > 8 || game.Board[move.Index] != "" {
		log.Printf("[MOVE] validation failed: cell %d is invalid or not empty", move.Index)
		return
	}

	game.applyMove(move.Index, currentPlayerSymbol)

	if game.Status == StatusWinX {
		updateLeaderboard(game.PlayerX)
		rdb.SRem(ctx, inGameKey, game.PlayerX, game.PlayerO)
	} else if game.Status == StatusWinO {
		updateLeaderboard(game.PlayerO)
		rdb.SRem(ctx, inGameKey, game.PlayerX, game.PlayerO)
	} else if game.Status == StatusDraw {
		rdb.SRem(ctx, inGameKey, game.PlayerX, game.PlayerO)
	} else if game.Status == StatusPlaying {
		if game.Turn == "X" {
			game.Turn = "O"
		} else {
			game.Turn = "X"
		}
	}

	if err := saveGame(ctx, game); err != nil {
		log.Printf("[MOVE] error saving game: %v", err)
		return
	}
	log.Printf("[MOVE] move successful on game %s by player %s", game.ID, currentPlayerSymbol)

	response := Message{
		Type:    "game_update",
		Payload: game,
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		log.Printf("[MOVE] error marshalling game update: %v", err)
		return
	}

	channel := "game:" + game.ID
	if err := rdb.Publish(ctx, channel, responseJSON).Err(); err != nil {
		log.Printf("[MOVE] error publishing game update: %v", err)
	}
}

func handleGetLeaderboard(client *Client) {
	log.Printf("[LEADERBOARD] Handling get_leaderboard request from PlayerID: %s", client.PlayerID)
	scores, err := getLeaderboard()
	if err != nil {
		log.Printf("[LEADERBOARD] Error getting leaderboard: %v", err)
		return
	}

	response := Message{
		Type:    "leaderboard_update",
		Payload: scores,
	}
	responseJSON, _ := json.Marshal(response)
	client.send <- responseJSON
}
