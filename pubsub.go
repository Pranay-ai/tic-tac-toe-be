package main

import (
	"context"
	"encoding/json"
	"log"
)

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

		gameData, _ := json.Marshal(gameUpdate.Payload)
		var game Game
		json.Unmarshal(gameData, &game)

		hub.direct <- &directMessage{playerID: game.PlayerX, message: []byte(msg.Payload)}
		hub.direct <- &directMessage{playerID: game.PlayerO, message: []byte(msg.Payload)}
	}
}
