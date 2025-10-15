package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
)

// The key for our Redis list that acts as the matchmaking queue.
const matchmakingQueueKey = "matchmaking:queue"

// handleFindMatch adds a client's ID to the matchmaking queue in Redis.
func handleFindMatch(client *Client) {
	log.Printf("Client %s is looking for a match", client.ID)

	// Check if the player is already in the queue to prevent duplicates.
	// This is an optional but good practice.
	// For simplicity in this step, we will assume no duplicates.

	// LPUSH adds the client's ID to the left (front) of the list.
	err := rdb.LPush(ctx, matchmakingQueueKey, client.ID).Err()
	if err != nil {
		log.Printf("Error adding client %s to matchmaking queue: %v", client.ID, err)
	}
}

// startMatchmaking runs a background process that continuously checks the queue for pairs of players.
func startMatchmaking(hub *Hub) {
	// A ticker sends a signal ("tick") on a channel at a regular interval.
	// This is an efficient way to run a recurring background task.
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// This `for` loop waits for the next tick from the ticker.
	for range ticker.C {
		// Check the number of players waiting in the queue.
		queueLength, err := rdb.LLen(ctx, matchmakingQueueKey).Result()
		if err != nil {
			log.Printf("Error getting matchmaking queue length: %v", err)
			continue // Skip this tick if we can't check the queue.
		}

		// If there are at least two players, we can make a match.
		if queueLength >= 2 {
			// RPOP removes and returns the right-most (oldest) element from the list.
			player1ID, err1 := rdb.RPop(ctx, matchmakingQueueKey).Result()
			player2ID, err2 := rdb.RPop(ctx, matchmakingQueueKey).Result()

			if err1 != nil || err2 != nil {
				log.Printf("Error popping players from matchmaking queue: %v, %v", err1, err2)
				// If we successfully popped one player but failed on the second, we should push the first one back.
				if err1 == nil {
					rdb.LPush(ctx, matchmakingQueueKey, player1ID)
				}
				continue
			}

			log.Printf("Match found! Pairing Player X (%s) and Player O (%s)", player1ID, player2ID)

			// Find the actual Client objects in the hub using their IDs.
			client1, ok1 := hub.clients[player1ID]
			client2, ok2 := hub.clients[player2ID]

			if !ok1 || !ok2 {
				log.Println("Matchmaking failed: one or both clients disconnected before match could be made.")
				// If one client is still connected, we can add them back to the queue.
				if ok1 {
					handleFindMatch(client1)
				}
				if ok2 {
					handleFindMatch(client2)
				}
				continue
			}

			// Create a new game for the matched players.
			newGame := &Game{
				ID:      uuid.NewString(),
				PlayerX: player1ID,
				PlayerO: player2ID,
				Board:   [9]string{}, // Board is initially empty.
				Turn:    "X",         // Player X always goes first.
				Status:  StatusPlaying,
			}

			// Save the initial state of the new game to Redis.
			if err := saveGame(ctx, newGame); err != nil {
				log.Printf("Error creating new game for %s and %s: %v", player1ID, player2ID, err)
				continue
			}

			// Associate the game ID with each client struct.
			// This is important for future operations, like handling disconnects.
			client1.GameID = newGame.ID
			client2.GameID = newGame.ID

			// Prepare the "match_found" notification message.
			response := Message{
				Type: "match_found",
				Payload: MatchFoundPayload{
					GameID:  newGame.ID,
					PlayerX: newGame.PlayerX,
					PlayerO: newGame.PlayerO,
				},
			}

			responseJSON, err := json.Marshal(response)
			if err != nil {
				log.Printf("Error marshalling match_found response: %v", err)
				continue
			}

			// Send the notification directly to each of the two matched players.
			hub.direct <- &directMessage{clientID: player1ID, message: responseJSON}
			hub.direct <- &directMessage{clientID: player2ID, message: responseJSON}
		}
	}
}
