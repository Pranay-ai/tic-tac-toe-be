package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
)

const matchmakingQueueKey = "matchmaking:queue"
const inQueueKey = "matchmaking:in_queue"
const inGameKey = "players_in_game"
const playerNamesKey = "player:names"

func handleFindMatch(client *Client, payload interface{}) {
	payloadData, _ := json.Marshal(payload)
	var findMatchPayload FindMatchPayload
	if err := json.Unmarshal(payloadData, &findMatchPayload); err != nil {
		log.Printf("Error unmarshalling find_match payload: %v", err)
		return
	}

	client.PlayerID = findMatchPayload.PlayerID
	client.PlayerName = findMatchPayload.PlayerName

	rdb.HSet(ctx, playerNamesKey, client.PlayerID, client.PlayerName)

	isAlreadyInGame, _ := rdb.SIsMember(ctx, inGameKey, client.PlayerID).Result()
	if isAlreadyInGame {
		log.Printf("Player %s tried to queue while already in a game.", client.PlayerID)
		return
	}

	isAlreadyInQueue, _ := rdb.SIsMember(ctx, inQueueKey, client.PlayerID).Result()
	if isAlreadyInQueue {
		log.Printf("Player %s is already in the matchmaking queue.", client.PlayerID)
		return
	}

	if err := rdb.SAdd(ctx, inQueueKey, client.PlayerID).Err(); err != nil {
		log.Printf("Error adding player to in_queue set: %v", err)
		return
	}

	if err := rdb.LPush(ctx, matchmakingQueueKey, client.PlayerID).Err(); err != nil {
		log.Printf("Error adding client to matchmaking queue: %v", err)
		rdb.SRem(ctx, inQueueKey, client.PlayerID)
		return
	}

	log.Printf("Player %s (Name: %s) successfully added to matchmaking queue.", client.PlayerID, client.PlayerName)
}

func startMatchmaking(hub *Hub) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		pipe := rdb.TxPipeline()
		queueLength := pipe.LLen(ctx, matchmakingQueueKey)
		pipe.Exec(ctx)

		if queueLength.Val() >= 2 {
			log.Println("Attempting to create a match...")
			player1ID, err1 := rdb.RPop(ctx, matchmakingQueueKey).Result()
			player2ID, err2 := rdb.RPop(ctx, matchmakingQueueKey).Result()

			if err1 != nil || err2 != nil {
				log.Printf("Error popping players from matchmaking queue: %v, %v", err1, err2)
				if err1 == nil {
					rdb.LPush(ctx, matchmakingQueueKey, player1ID)
				}
				continue
			}

			rdb.SRem(ctx, inQueueKey, player1ID, player2ID)
			rdb.SAdd(ctx, inGameKey, player1ID, player2ID)

			log.Printf("Match found! Pairing Player X (%s) and Player O (%s)", player1ID, player2ID)

			var client1, client2 *Client
			for _, client := range hub.clients {
				if client.PlayerID == player1ID {
					client1 = client
				}
				if client.PlayerID == player2ID {
					client2 = client
				}
			}

			if client1 == nil || client2 == nil {
				log.Println("Matchmaking failed: one or more clients disconnected. Rolling back.")
				rdb.SRem(ctx, inGameKey, player1ID, player2ID)
				if client1 != nil {
					rdb.SAdd(ctx, inQueueKey, client1.PlayerID)
					rdb.LPush(ctx, matchmakingQueueKey, client1.PlayerID)
				}
				if client2 != nil {
					rdb.SAdd(ctx, inQueueKey, client2.PlayerID)
					rdb.LPush(ctx, matchmakingQueueKey, client2.PlayerID)
				}
				continue
			}

			newGame := &Game{
				ID:          uuid.NewString(),
				PlayerX:     player1ID,
				PlayerO:     player2ID,
				PlayerXName: client1.PlayerName,
				PlayerOName: client2.PlayerName,
				Board:       [9]string{},
				Turn:        "X",
				Status:      StatusPlaying,
			}

			saveGame(ctx, newGame)

			client1.GameID = newGame.ID
			client2.GameID = newGame.ID

			response := Message{Type: "match_found", Payload: newGame}
			responseJSON, _ := json.Marshal(response)
			hub.direct <- &directMessage{playerID: client1.PlayerID, message: responseJSON}
			hub.direct <- &directMessage{playerID: client2.PlayerID, message: responseJSON}
		}
	}
}
