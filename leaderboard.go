package main

import (
	"context"
	"log"
)

const leaderboardKey = "leaderboard:wins"

type LeaderboardEntry struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

func updateLeaderboard(winnerID string) {
	_, err := rdb.ZIncrBy(context.Background(), leaderboardKey, 1, winnerID).Result()
	if err != nil {
		log.Printf("Error updating leaderboard: %v", err)
	}
}

func getLeaderboard() ([]LeaderboardEntry, error) {
	idScores, err := rdb.ZRevRangeWithScores(context.Background(), leaderboardKey, 0, 9).Result()
	if err != nil {
		return nil, err
	}

	if len(idScores) == 0 {
		return []LeaderboardEntry{}, nil
	}

	var playerIDs []string
	for _, idScore := range idScores {
		playerIDs = append(playerIDs, idScore.Member.(string))
	}

	names, err := rdb.HMGet(context.Background(), "player:names", playerIDs...).Result()
	if err != nil {
		return nil, err
	}

	var leaderboard []LeaderboardEntry
	for i, idScore := range idScores {
		var playerName string
		if names[i] != nil {
			playerName = names[i].(string)
		} else {
			playerName = "Unknown Player"
		}
		leaderboard = append(leaderboard, LeaderboardEntry{
			Name:  playerName,
			Score: idScore.Score,
		})
	}

	return leaderboard, nil
}
