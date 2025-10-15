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
		return fmt.Errorf("error marshalling game state: %w", err)
	}
	err = rdb.Set(ctx, key, jsonData, 0).Err()
	if err != nil {
		return fmt.Errorf("error saving game state to redis: %w", err)
	}
	return nil
}

func getGame(ctx context.Context, gameID string) (*Game, error) {
	key := fmt.Sprintf("game:%s", gameID)
	jsonData, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error retrieving game state from redis: %w", err)
	}
	var game Game
	err = json.Unmarshal([]byte(jsonData), &game)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling game state: %w", err)
	}
	return &game, nil
}
