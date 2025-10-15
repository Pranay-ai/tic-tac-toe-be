package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"
)

const (
	StatusPlaying = "playing"
	StatusWinX    = "win_x"
	StatusWinO    = "win_o"
	StatusDraw    = "draw"
)

// Game represents the state of a single Tic-Tac-Toe game.
type Game struct {
	ID      string    `json:"id"`
	PlayerX string    `json:"playerX"` // Will store client ID
	PlayerO string    `json:"playerO"` // Will store client ID
	Board   [9]string `json:"board"`   // Represents the 3x3 grid
	Turn    string    `json:"turn"`    // "X" or "O"
	Status  string    `json:"status"`
}

var winningCombinations = [][3]int{
	// Rows
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8},
	// Columns
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8},
	// Diagonals
	{0, 4, 8}, {2, 4, 6},
}

// applyMove updates the board and checks for a win or draw.
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

// checkForWin checks if the specified player has won the game.
func (g *Game) checkForWin(player string) bool {
	for _, combo := range winningCombinations {
		if g.Board[combo[0]] == player && g.Board[combo[1]] == player && g.Board[combo[2]] == player {
			return true
		}
	}
	return false
}

// checkForDraw checks if the game is a draw.
func (g *Game) checkForDraw() bool {
	for _, cell := range g.Board {
		if cell == "" {
			return false // If any cell is empty, it's not a draw
		}
	}
	return true
}

func saveGame(ctx context.Context, game *Game) error {
	// Use a key with a prefix for good practice, e.g., "game:<id>"
	key := fmt.Sprintf("game:%s", game.ID)

	// Convert the Game struct to JSON
	jsonData, err := json.Marshal(game)
	if err != nil {
		return fmt.Errorf("error marshalling game state: %w", err)
	}

	// Save the JSON string in Redis. 0 means no expiration.
	err = rdb.Set(ctx, key, jsonData, 0).Err()
	if err != nil {
		return fmt.Errorf("error saving game state to redis: %w", err)
	}
	return nil
}

// getGame retrieves a game state from Redis and deserializes it from JSON.
func getGame(ctx context.Context, gameID string) (*Game, error) {
	key := fmt.Sprintf("game:%s", gameID)

	// Get the JSON string from Redis
	jsonData, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Game not found is not a system error
	} else if err != nil {
		return nil, fmt.Errorf("error retrieving game state from redis: %w", err)
	}

	// Convert the JSON string back to a Game struct
	var game Game
	err = json.Unmarshal([]byte(jsonData), &game)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling game state: %w", err)
	}
	return &game, nil
}
