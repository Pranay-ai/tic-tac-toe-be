package main

import (
	"context"
	"encoding/json"
	"log"
)

func subscribeToGameUpdates(ctx context.Context, hub *Hub) {
	log.Println("[PUBSUB] Subscribing to game update channels (game:*)")
	pubsub := rdb.PSubscribe(ctx, "game:*")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		log.Printf("[PUBSUB] Received message from Redis on channel %s", msg.Channel)
		var gameUpdate Message
		if err := json.Unmarshal([]byte(msg.Payload), &gameUpdate); err != nil {
			log.Printf("[PUBSUB] Error unmarshalling game update: %v", err)
			continue
		}

		gameData, _ := json.Marshal(gameUpdate.Payload)
		var game Game
		json.Unmarshal(gameData, &game)

		hub.direct <- &directMessage{playerID: game.PlayerX, message: []byte(msg.Payload)}
		hub.direct <- &directMessage{playerID: game.PlayerO, message: []byte(msg.Payload)}
	}
}
