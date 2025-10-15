package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/rs/cors"
)

func main() {
	initRedis()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("[MAIN] PORT not set, defaulting to %s", port)

	hub := newHub()
	go hub.run()
	go startMatchmaking(hub)
	go subscribeToGameUpdates(context.Background(), hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	handler := cors.Default().Handler(mux)

	serverAddr := ":" + port
	log.Println("[MAIN] Server starting on", serverAddr)
	err := http.ListenAndServe(serverAddr, handler)
	if err != nil {
		log.Fatal("[MAIN] ListenAndServe Error: ", err)
	}
}
