package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/rs/cors" // Corrected import path
)

func main() {
	initRedis()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Println("PORT not set, defaulting to 8080")
	}

	hub := newHub()
	go hub.run()
	go startMatchmaking(hub)
	go subscribeToGameUpdates(context.Background(), hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	// Add the CORS middleware to allow connections from your frontend
	handler := cors.Default().Handler(mux)

	serverAddr := ":" + port
	log.Println("Server starting on", serverAddr)
	err := http.ListenAndServe(serverAddr, handler)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
