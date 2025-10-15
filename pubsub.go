// pubsub.go
package main

import (
	"context"
	"encoding/json"
	"log"
)

// subscribeToGameUpdates listens for messages on game channels in Redis.
// pubsub.go
func subscribeToGameUpdates(ctx context.Context, hub *Hub) {
	pubsub := rdb.PSubscribe(ctx, "game:*")
	defer pubsub.Close()
	ch := pubsub.Channel()
	log.Println("Subscribed to game update channels")

	for msg := range ch {
		var gameUpdate Message
		if err := json.Unmarshal([]byte(msg.Payload), &gameUpdate); err != nil {
			log.Printf("Error unmarshalling game update: %v", err)
			continue
		}

		// Extract the game object from the payload
		gameData, _ := json.Marshal(gameUpdate.Payload)
		var game Game
		json.Unmarshal(gameData, &game)

		// Send the update to the two players in the game
		hub.direct <- &directMessage{clientID: game.PlayerX, message: []byte(msg.Payload)}
		hub.direct <- &directMessage{clientID: game.PlayerO, message: []byte(msg.Payload)}
	}
}
