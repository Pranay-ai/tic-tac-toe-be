package main

import (
	"context"
	"log"
	"os" // Import the 'os' package

	"github.com/go-redis/redis/v8"
)

var rdb *redis.Client
var ctx = context.Background()

func initRedis() {
	// 1. Read the REDIS_URL from the environment variable.
	redisURL := os.Getenv("REDIS_URL")

	// 2. If it's not set, fall back to the default for local development.
	if redisURL == "" {
		redisURL = "localhost:6379"
		log.Println("REDIS_URL not set, defaulting to localhost:6379")
	} else {
		log.Println("Connecting to Redis at", redisURL)
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Could not parse Redis URL: %v", err)
	}

	rdb = redis.NewClient(opt)

	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	log.Println("Successfully connected to Redis!")
}
