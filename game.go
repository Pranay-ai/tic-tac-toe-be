package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	StatusPlaying       = "playing"
	StatusWinX          = "win_x"
	StatusWinO          = "win_o"
	StatusDraw          = "draw"
	StatusDisconnectedX = "disconnected_x"
	StatusDisconnectedO = "disconnected_o"
)

type Game struct {
	ID          string    `json:"id"`
	PlayerX     string    `json:"playerX"`
	PlayerO     string    `json:"playerO"`
	PlayerXName string    `json:"playerXName"`
	PlayerOName string    `json:"playerOName"`
	Board       [9]string `json:"board"`
	Turn        string    `json:"turn"`
	Status      string    `json:"status"`
}

var winningCombinations = [][3]int{
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8},
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8},
	{0, 4, 8}, {2, 4, 6},
}

func handleGameDisconnect(playerID string, gameID string) {
	log.Printf("[GAME] Player %s disconnected. Starting 30s forfeit timer for game %s.", playerID, gameID)
	game, err := getGame(ctx, gameID)
	if err != nil || game == nil || game.Status != StatusPlaying {
		log.Printf("[GAME] Forfeit timer cancelled for game %s: game already over or not found.", gameID)
		return
	}

	var disconnectedPlayerSymbol string
	if playerID == game.PlayerX {
		disconnectedPlayerSymbol = "X"
		game.Status = StatusDisconnectedX
	} else {
		disconnectedPlayerSymbol = "O"
		game.Status = StatusDisconnectedO
	}
	saveGame(ctx, game)

	response := Message{Type: "game_update", Payload: game}
	responseJSON, _ := json.Marshal(response)
	channel := "game:" + game.ID
	rdb.Publish(ctx, channel, responseJSON)

	time.Sleep(30 * time.Second)

	game, err = getGame(ctx, gameID)
	if err != nil || game == nil {
		return
	}

	if game.Status == StatusDisconnectedX || game.Status == StatusDisconnectedO {
		log.Printf("[GAME] Forfeit timer ended. Player %s did not reconnect in time. Game %s forfeited.", playerID, gameID)
		if disconnectedPlayerSymbol == "X" {
			game.Status = StatusWinO
			updateLeaderboard(game.PlayerO)
		} else {
			game.Status = StatusWinX
			updateLeaderboard(game.PlayerX)
		}
		saveGame(ctx, game)
		rdb.SRem(ctx, inGameKey, game.PlayerX, game.PlayerO)

		finalResponse := Message{Type: "game_update", Payload: game}
		finalResponseJSON, _ := json.Marshal(finalResponse)
		rdb.Publish(ctx, channel, finalResponseJSON)
	} else {
		log.Printf("[GAME] Forfeit timer ended. Player %s appears to have reconnected to game %s. No action taken.", playerID, gameID)
	}
}

func (g *Game) applyMove(index int, player string) {
	g.Board[index] = player
	if g.checkForWin(player) {
		if player == "X" {
			g.Status = StatusWinX
		} else {
			g.Status = StatusWinO
		}
		return
	}
	if g.checkForDraw() {
		g.Status = StatusDraw
		return
	}
}

func (g *Game) checkForWin(player string) bool {
	for _, combo := range winningCombinations {
		if g.Board[combo[0]] == player && g.Board[combo[1]] == player && g.Board[combo[2]] == player {
			return true
		}
	}
	return false
}

func (g *Game) checkForDraw() bool {
	for _, cell := range g.Board {
		if cell == "" {
			return false
		}
	}
	return true
}

func saveGame(ctx context.Context, game *Game) error {
	key := fmt.Sprintf("game:%s", game.ID)
	jsonData, err := json.Marshal(game)
	if err != nil {
		log.Printf("[GAME] ERROR marshalling game state for game %s: %v", game.ID, err)
		return err
	}
	err = rdb.Set(ctx, key, jsonData, 0).Err()
	if err != nil {
		log.Printf("[GAME] ERROR saving game state to Redis for game %s: %v", game.ID, err)
		return err
	}
	log.Printf("[GAME] Game state saved for game %s. Status: %s, Turn: %s", game.ID, game.Status, game.Turn)
	return nil
}

func getGame(ctx context.Context, gameID string) (*Game, error) {
	key := fmt.Sprintf("game:%s", gameID)
	jsonData, err := rdb.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			log.Printf("[GAME] ERROR retrieving game state from Redis for game %s: %v", gameID, err)
		}
		return nil, err
	}
	log.Printf("[GAME] Game state retrieved for game %s.", gameID)
	var game Game
	err = json.Unmarshal([]byte(jsonData), &game)
	if err != nil {
		log.Printf("[GAME] ERROR unmarshalling game state for game %s: %v", gameID, err)
		return nil, err
	}
	return &game, nil
}
