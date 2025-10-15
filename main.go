package main

import (
	"context"
	"flag"
	"log"
	"net/http"
)

var addr = flag.String("addr", ":8080", "http service address")

func main() {
	flag.Parse()
	// We will create the hub here later
	initRedis()
	hub := newHub()
	go hub.run()

	go startMatchmaking(hub)

	go subscribeToGameUpdates(context.Background(), hub) // Add this

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// We will pass the hub in here later
		serveWs(hub, w, r)
		log.Println("New connection attempt to /ws")
	})

	log.Println("Server starting on", *addr)
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
