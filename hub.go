package main

import (
	"log"
)

// A message intended for a specific player.
type directMessage struct {
	playerID string
	message  []byte
}

type Hub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	direct     chan *directMessage
}

func newHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[string]*Client),
		direct:     make(chan *directMessage),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client.ID] = client
			log.Printf("Client %s connected. Total clients: %d", client.ID, len(h.clients))
		case client := <-h.unregister:
			if _, ok := h.clients[client.ID]; ok {
				// If the disconnecting client had a PlayerID, remove them from the queue.
				if client.PlayerID != "" {
					rdb.LRem(ctx, matchmakingQueueKey, 0, client.PlayerID)
					rdb.SRem(ctx, inQueueKey, client.PlayerID)
					log.Printf("Player %s removed from matchmaking queue due to disconnect.", client.PlayerID)
				}

				delete(h.clients, client.ID)
				close(client.send)
				log.Printf("Client %s disconnected. Total clients: %d", client.ID, len(h.clients))
			}
		case dm := <-h.direct:
			// Loop through all connected clients to find the recipient by their PlayerID.
			for _, client := range h.clients {
				if client.PlayerID == dm.playerID {
					select {
					case client.send <- dm.message:
					default:
						close(client.send)
						delete(h.clients, client.ID)
					}
					break // Found the client, no need to check others.
				}
			}
		}
	}
}
