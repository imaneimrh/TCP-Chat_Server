package room

import (
	"sync"

	"TCP-CHAT_SERVER/shared"
)

type Room struct {
	Name       string
	Clients    map[*shared.Client]bool
	Broadcast  chan shared.Message
	Register   chan *shared.Client
	Unregister chan *shared.Client
	mu         sync.Mutex
}

func NewRoom(name string) *Room {
	return &Room{
		Name:       name,
		Clients:    make(map[*shared.Client]bool),
		Broadcast:  make(chan shared.Message, 100),
		Register:   make(chan *shared.Client),
		Unregister: make(chan *shared.Client),
	}
}

func (r *Room) Run() {
	for {
		select {
		case client := <-r.Register:
			r.registerClient(client)
		case client := <-r.Unregister:
			r.unregisterClient(client)
		case message := <-r.Broadcast:
			r.broadcastMessage(message)
		}
	}
}

func (r *Room) registerClient(client *shared.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Clients[client] = true
	client.AddRoom(r.Name)

	joinMsg := shared.Message{
		Type:     shared.TextMessage,
		Sender:   "Server",
		RoomName: r.Name,
		Content:  client.Username + " has joined the room.",
	}

	r.broadcastToClients(joinMsg)
}

func (r *Room) unregisterClient(client *shared.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.Clients[client]; ok {
		delete(r.Clients, client)
		client.RemoveRoom(r.Name)

		leaveMsg := shared.Message{
			Type:     shared.TextMessage,
			Sender:   "Server",
			RoomName: r.Name,
			Content:  client.Username + " has left the room.",
		}

		r.broadcastToClients(leaveMsg)
	}
}

func (r *Room) broadcastMessage(message shared.Message) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.broadcastToClients(message)
}

func (r *Room) broadcastToClients(message shared.Message) {
	for client := range r.Clients {
		select {
		case client.Send <- message:
		default:
			delete(r.Clients, client)
			client.RemoveRoom(r.Name)
		}
	}
}

func (r *Room) GetClientCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Clients)
}
