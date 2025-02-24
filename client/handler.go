package client

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"TCP-CHAT_SERVER/room"
	"TCP-CHAT_SERVER/shared"
)

type Handler struct {
	Clients      map[string]*shared.Client
	RoomManager  *room.Manager
	FileTransfer *FileTransfer
	Register     chan *shared.Client
	Unregister   chan *shared.Client
	Broadcast    chan shared.Message
	DirectMsg    chan shared.Message
	mu           sync.RWMutex
}

func NewHandler(roomManager *room.Manager) *Handler {
	return &Handler{
		Clients:      make(map[string]*shared.Client),
		RoomManager:  roomManager,
		FileTransfer: NewFileTransfer(),
		Register:     make(chan *shared.Client),
		Unregister:   make(chan *shared.Client),
		Broadcast:    make(chan shared.Message),
		DirectMsg:    make(chan shared.Message),
	}
}

func (h *Handler) Run() {
	for {
		select {
		case client := <-h.Register:
			h.registerClient(client)
		case client := <-h.Unregister:
			h.unregisterClient(client)
		case message := <-h.Broadcast:
			h.broadcastMessage(message)
		case message := <-h.DirectMsg:
			h.sendDirectMessage(message)
		}
	}
}

func (h *Handler) registerClient(client *shared.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.Clients[client.Username] = client

	h.RoomManager.JoinRoom("general", client)

	welcomeMsg := shared.Message{
		Type:    shared.TextMessage,
		Sender:  "Server",
		Content: fmt.Sprintf("Welcome to the chat server, %s! You've been added to the 'general' room.", client.Username),
	}

	client.Send <- welcomeMsg

	joinMsg := shared.Message{
		Type:     shared.TextMessage,
		Sender:   "Server",
		RoomName: "general",
		Content:  fmt.Sprintf("%s has joined the server.", client.Username),
	}

	h.RoomManager.BroadcastToRoom("general", joinMsg)
}

func (h *Handler) unregisterClient(client *shared.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.Clients[client.Username]; ok {
		for roomName := range client.Rooms {
			h.RoomManager.LeaveRoom(roomName, client)
		}

		delete(h.Clients, client.Username)
		close(client.Send)
	}
}

func (h *Handler) broadcastMessage(message shared.Message) {
	if message.RoomName == "" {
		message.RoomName = "general"
	}

	h.RoomManager.BroadcastToRoom(message.RoomName, message)
}

func (h *Handler) sendDirectMessage(message shared.Message) {
	h.mu.RLock()
	recipient, exists := h.Clients[message.Recipient]
	h.mu.RUnlock()

	if !exists {
		h.mu.RLock()
		sender, senderExists := h.Clients[message.Sender]
		h.mu.RUnlock()

		if senderExists {
			errorMsg := shared.Message{
				Type:    shared.TextMessage,
				Sender:  "Server",
				Content: fmt.Sprintf("User %s is not online.", message.Recipient),
			}
			sender.Send <- errorMsg
		}
		return
	}

	recipient.Send <- message

	h.mu.RLock()
	sender, senderExists := h.Clients[message.Sender]
	h.mu.RUnlock()

	if senderExists {
		confirmMsg := shared.Message{
			Type:    shared.TextMessage,
			Sender:  "Server",
			Content: fmt.Sprintf("(To %s): %s", message.Recipient, message.Content),
		}
		sender.Send <- confirmMsg
	}
}

func (h *Handler) HandleClient(conn net.Conn, username string) {
	client := shared.NewClient(conn)
	client.Username = username

	h.Register <- client

	reader := bufio.NewReader(conn)

	go func() {
		defer func() {
			h.Unregister <- client
			conn.Close()
		}()

		for {
			msg, err := ReadMessage(reader)
			if err != nil {
				if err != io.EOF {
					log.Printf("Error reading from client %s: %v", client.Username, err)
				}
				break
			}

			msg.Sender = client.Username

			if IsCommand(msg.Content) {
				cmdMsg := ProcessCommand(msg.Content)
				cmdMsg.Sender = client.Username

				switch cmdMsg.Type {
				case shared.JoinRoomMessage:
					err := h.RoomManager.JoinRoom(cmdMsg.RoomName, client)
					if err != nil {
						errorMsg := shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("Error joining room: %v", err),
						}
						client.Send <- errorMsg
					} else {
						successMsg := shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("You have joined room: %s", cmdMsg.RoomName),
						}
						client.Send <- successMsg
					}

				case shared.LeaveRoomMessage:
					err := h.RoomManager.LeaveRoom(cmdMsg.RoomName, client)
					if err != nil {
						errorMsg := shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("Error leaving room: %v", err),
						}
						client.Send <- errorMsg
					} else {
						successMsg := shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("You have left room: %s", cmdMsg.RoomName),
						}
						client.Send <- successMsg
					}

				case shared.CreateRoomMessage:
					_, err := h.RoomManager.CreateRoom(cmdMsg.RoomName)
					if err != nil {
						errorMsg := shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("Error creating room: %v", err),
						}
						client.Send <- errorMsg
					} else {
						successMsg := shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("Room created: %s", cmdMsg.RoomName),
						}
						client.Send <- successMsg
					}

				case shared.ListRoomsMessage:
					rooms := h.RoomManager.ListRooms()
					roomList := strings.Join(rooms, ", ")
					listMsg := shared.Message{
						Type:    shared.TextMessage,
						Sender:  "Server",
						Content: fmt.Sprintf("Available rooms: %s", roomList),
					}
					client.Send <- listMsg

				case shared.DirectMessage:
					h.DirectMsg <- cmdMsg

				case shared.TextMessage:
					client.Send <- cmdMsg
				}
			} else if msg.Type == shared.FileTransferRequest ||
				msg.Type == shared.FileTransferData ||
				msg.Type == shared.FileTransferComplete {
				HandleFileTransfer(msg, conn, h.FileTransfer)
			} else {
				var activeRoom string
				for room := range client.Rooms {
					activeRoom = room
					break
				}

				if activeRoom == "" {
					activeRoom = "general"
				}

				msg.RoomName = activeRoom
				h.Broadcast <- msg
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer func() {
			ticker.Stop()
			conn.Close()
		}()

		for {
			select {
			case message, ok := <-client.Send:
				if !ok {
					return
				}
				data, err := FormatMessage(message)
				if err != nil {
					log.Printf("Error formatting message: %v", err)
					return
				}
				_, err = conn.Write(data)
				if err != nil {
					log.Printf("Error writing to client %s: %v", client.Username, err)
					return
				}

			case <-ticker.C:
				pingMsg := shared.Message{
					Type:    shared.TextMessage,
					Sender:  "Server",
					Content: "PING",
				}

				data, err := FormatMessage(pingMsg)
				if err != nil {
					log.Printf("Error formatting ping message: %v", err)
					return
				}

				_, err = conn.Write(data)
				if err != nil {
					log.Printf("Error writing ping to client %s: %v", client.Username, err)
					return
				}
			}
		}
	}()
}
