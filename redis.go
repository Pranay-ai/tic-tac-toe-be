package main

import (
	"context"
	"log"
	"os"

	"github.com/go-redis/redis/v8"
)

var rdb *redis.Client
var ctx = context.Background()

func initRedis() {
	redisURL := os.Getenv("REDIS_URL")

	if redisURL == "" {
		redisURL = "redis://localhost:6379"
		log.Printf("[REDIS] REDIS_URL not set, defaulting to %s", redisURL)
	} else {
		log.Printf("[REDIS] Connecting to Redis at provided URL.")
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("[REDIS] Could not parse Redis URL: %v", err)
	}

	rdb = redis.NewClient(opt)

	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("[REDIS] Could not connect to Redis: %v", err)
	}

	log.Println("[REDIS] Successfully connected to Redis!")
}
