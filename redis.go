// redis.go
package main

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
)

// A global variable to hold our Redis client.
var rdb *redis.Client
var ctx = context.Background()

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Default Redis address
		Password: "",               // No password set
		DB:       0,                // Use default DB
	})

	// Ping the server to check the connection.
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	log.Println("Successfully connected to Redis!")
}
