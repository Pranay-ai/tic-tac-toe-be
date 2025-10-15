// client.go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// upgrader upgrades HTTP connections to the WebSocket protocol.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// In production, you should check the origin of the request.
	// For development, we can allow any origin.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	ID     string
	GameID string
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	// Create a new client with a unique ID.
	client := &Client{
		ID:   uuid.NewString(),
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
	client.hub.register <- client

	// Start goroutines for reading and writing to this client.
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the websocket connection to the hub.
func (c *Client) readPump() {
	// Ensure cleanup happens when the function exits.
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		// Read a message from the connection.
		_, rawMessage, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break // Exit loop on error
		}

		// Parse the incoming message into our standard Message struct.
		var msg Message
		if err := json.Unmarshal(rawMessage, &msg); err != nil {
			log.Printf("error unmarshalling message: %v", err)
			continue
		}

		// Handle the message based on its type.
		switch msg.Type {
		case "move":
			handleMove(c, msg.Payload)
		case "find_match": // Add this case
			handleFindMatch(c)
		default:
			log.Printf("unknown message type received: %s", msg.Type)
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			// Write the message to the client.
			c.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

// handleMove processes a move request from a client.
func handleMove(client *Client, payload interface{}) {
	// 1. Unmarshal the generic payload into our specific MovePayload struct
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

	// 2. Fetch the current game state from Redis
	ctx := context.Background()
	game, err := getGame(ctx, move.GameID)
	if err != nil {
		log.Printf("error getting game from redis: %v", err)
		return
	}
	if game == nil {
		log.Printf("game not found: %s", move.GameID)
		return
	}

	// 3. VALIDATE THE MOVE (Server-Authoritative Logic)
	var currentPlayerSymbol string
	if client.ID == game.PlayerX {
		currentPlayerSymbol = "X"
	} else if client.ID == game.PlayerO {
		currentPlayerSymbol = "O"
	} else {
		log.Printf("validation failed: client %s is not a player in game %s", client.ID, game.ID)
		return // Not a player in this game
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

	// 4. APPLY the move and SAVE the new state
	game.applyMove(move.Index, currentPlayerSymbol)
	// Switch turn if the game is still playing
	if game.Status == StatusPlaying {
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

	// 5. BROADCAST the new state to all clients
	response := Message{
		Type:    "game_update",
		Payload: game,
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		log.Printf("error marshalling game update: %v", err)
		return
	}

	// Send the update to the hub's broadcast channel
	channel := "game:" + game.ID
	if err := rdb.Publish(ctx, channel, responseJSON).Err(); err != nil {
		log.Printf("error publishing game update: %v", err)
	}
}
