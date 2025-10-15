package main

import (
	"log"
)

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
	log.Println("[HUB] Hub is running...")
	for {
		select {
		case client := <-h.register:
			h.clients[client.ID] = client
			log.Printf("[HUB] Client %s registered. Total clients: %d", client.ID, len(h.clients))

		case client := <-h.unregister:
			if _, ok := h.clients[client.ID]; ok {
				log.Printf("[HUB] Unregistering client %s (PlayerID: %s)", client.ID, client.PlayerID)

				if client.GameID != "" {
					log.Printf("[HUB] In-game player %s disconnected from game %s. Starting forfeit timer.", client.PlayerID, client.GameID)
					go handleGameDisconnect(client.PlayerID, client.GameID)
				}

				if client.PlayerID != "" {
					rdb.LRem(ctx, matchmakingQueueKey, 0, client.PlayerID)
					rdb.SRem(ctx, inQueueKey, client.PlayerID)
					log.Printf("[HUB] Player %s removed from matchmaking queue due to disconnect.", client.PlayerID)
				}

				delete(h.clients, client.ID)
				close(client.send)
				log.Printf("[HUB] Client %s connection closed. Total clients: %d", client.ID, len(h.clients))
			}

		case dm := <-h.direct:
			log.Printf("[HUB] Routing direct message to PlayerID: %s", dm.playerID)
			var foundClient bool
			for _, client := range h.clients {
				if client.PlayerID == dm.playerID {
					select {
					case client.send <- dm.message:
						log.Printf("[HUB] Message sent successfully to PlayerID: %s (ConnectionID: %s)", dm.playerID, client.ID)
					default:
						log.Printf("[HUB] Send buffer full for PlayerID: %s. Closing connection.", dm.playerID)
						close(client.send)
						delete(h.clients, client.ID)
					}
					foundClient = true
					break
				}
			}
			if !foundClient {
				log.Printf("[HUB] Could not find an active client for PlayerID: %s", dm.playerID)
			}
		}
	}
}
