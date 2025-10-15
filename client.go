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
		log.Println(err)
		return
	}
	client := &Client{
		ID:   uuid.NewString(),
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
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
				log.Printf("error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(rawMessage, &msg); err != nil {
			log.Printf("error unmarshalling message: %v", err)
			continue
		}

		switch msg.Type {
		case "move":
			handleMove(c, msg.Payload)
		case "find_match":
			handleFindMatch(c, msg.Payload)
		case "get_leaderboard":
			handleGetLeaderboard(c)
		default:
			log.Printf("unknown message type received: %s", msg.Type)
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
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

func handleMove(client *Client, payload interface{}) {
	moveData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("error marshalling move payload: %v", err)
		return
	}
	var move MovePayload
	if err := json.Unmarshal(moveData, &move); err != nil {
		log.Printf("error unmarshalling move payload: %v", err)
		return
	}

	ctx := context.Background()
	game, err := getGame(ctx, move.GameID)
	if err != nil {
		log.Printf("error getting game: %v", err)
		return
	}
	if game == nil {
		log.Printf("game not found: %s", move.GameID)
		return
	}

	var currentPlayerSymbol string
	if client.PlayerID == game.PlayerX {
		currentPlayerSymbol = "X"
	} else if client.PlayerID == game.PlayerO {
		currentPlayerSymbol = "O"
	} else {
		log.Printf("validation failed: client connection %s (PlayerID: %s) is not a player in game %s", client.ID, client.PlayerID, game.ID)
		return
	}

	if game.Status != StatusPlaying {
		log.Println("validation failed: game is already over")
		return
	}
	if game.Turn != currentPlayerSymbol {
		log.Printf("validation failed: not player %s's turn", currentPlayerSymbol)
		return
	}
	if move.Index < 0 || move.Index > 8 || game.Board[move.Index] != "" {
		log.Printf("validation failed: cell %d is invalid or not empty", move.Index)
		return
	}

	game.applyMove(move.Index, currentPlayerSymbol)

	if game.Status == StatusWinX {
		updateLeaderboard(game.PlayerX) // Use PlayerX (the ID)
		rdb.SRem(ctx, inGameKey, game.PlayerX, game.PlayerO)
	} else if game.Status == StatusWinO {
		updateLeaderboard(game.PlayerO) // Use PlayerO (the ID)
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
		log.Printf("error saving game: %v", err)
		return
	}
	log.Printf("move successful on game %s by player %s", game.ID, currentPlayerSymbol)

	response := Message{
		Type:    "game_update",
		Payload: game,
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		log.Printf("error marshalling game update: %v", err)
		return
	}

	channel := "game:" + game.ID
	if err := rdb.Publish(ctx, channel, responseJSON).Err(); err != nil {
		log.Printf("error publishing game update: %v", err)
	}
}

func handleGetLeaderboard(client *Client) {
	scores, err := getLeaderboard()
	if err != nil {
		log.Printf("Error getting leaderboard: %v", err)
		return
	}

	response := Message{
		Type:    "leaderboard_update",
		Payload: scores,
	}
	responseJSON, _ := json.Marshal(response)
	client.send <- responseJSON
}
