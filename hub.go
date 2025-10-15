// hub.go
package main

import "log"

// A message intended for a specific client.
type directMessage struct {
	clientID string
	message  []byte
}

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	// Registered clients. The key is the client's unique string ID.
	clients map[string]*Client // CHANGED: The map key is now a string.

	// Inbound messages from the clients.
	direct chan *directMessage // CHANGED: This is now for direct messages.

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		direct:     make(chan *directMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[string]*Client), // CHANGED: Initializing the correct map type.
	}
}

// hub.go
func (h *Hub) run() {
	// We will add the pubsub listener call here later
	for {
		select {
		case client := <-h.register:
			h.clients[client.ID] = client // Use client ID as key
			log.Printf("Client %s connected. Total clients: %d", client.ID, len(h.clients))
		case client := <-h.unregister:
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.send)
				log.Printf("Client %s disconnected. Total clients: %d", client.ID, len(h.clients))
			}
		case dm := <-h.direct:
			if client, ok := h.clients[dm.clientID]; ok {
				select {
				case client.send <- dm.message:
				default:
					close(client.send)
					delete(h.clients, client.ID)
				}
			}
		}
	}
}
