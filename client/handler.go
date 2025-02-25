package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/imaneimrh/TCP-Chat_Server/auth"
	"github.com/imaneimrh/TCP-Chat_Server/room"
	"github.com/imaneimrh/TCP-Chat_Server/shared"
)

type Handler struct {
	Clients      map[string]*shared.Client
	RoomManager  *room.Manager
	FileTransfer *FileTransfer
	AuthManager  *auth.Manager
	Register     chan *shared.Client
	Unregister   chan *shared.Client
	Broadcast    chan shared.Message
	DirectMsg    chan shared.Message
	mu           sync.RWMutex
}

func NewHandler(roomManager *room.Manager, authManager *auth.Manager) *Handler {
	return &Handler{
		Clients:      make(map[string]*shared.Client),
		RoomManager:  roomManager,
		FileTransfer: NewFileTransfer(),
		AuthManager:  authManager,
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

	if client.Username != "" {
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
}

func (h *Handler) unregisterClient(client *shared.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.Clients[client.Username]; ok {
		for roomName := range client.Rooms {
			h.RoomManager.LeaveRoom(roomName, client)
		}

		if client.Username != "" {
			leaveMsg := shared.Message{
				Type:    shared.TextMessage,
				Sender:  "Server",
				Content: fmt.Sprintf("%s has left the server.", client.Username),
			}

			h.RoomManager.BroadcastToRoom("general", leaveMsg)
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

	directMsg := shared.Message{
		Type:      shared.TextMessage,
		Sender:    message.Sender,
		Recipient: message.Recipient,
		Content:   message.Content,
	}
	recipient.Send <- directMsg

	serverMsgToRecipient := shared.Message{
		Type:      shared.TextMessage,
		Sender:    "Server",
		Recipient: message.Recipient,
		Content:   fmt.Sprintf("Direct message from %s: %s", message.Sender, message.Content),
	}
	recipient.Send <- serverMsgToRecipient

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

func (h *Handler) getOnlineUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	users := []string{}
	for _, client := range h.Clients {
		if client.Username != "" {
			users = append(users, client.Username)
		}
	}
	return users
}

func (h *Handler) HandleClient(conn net.Conn) {
	client := shared.NewClient(conn)
	tempID := conn.RemoteAddr().String()

	h.mu.Lock()
	h.Clients[tempID] = client
	h.mu.Unlock()

	welcomeMsg := shared.Message{
		Type:   shared.TextMessage,
		Sender: "Server",
		Content: "╔══════════════════════════════════════════════╗\n" +
			"║       Welcome to the TCP Chat Server!         ║\n" +
			"╠══════════════════════════════════════════════╣\n" +
			"║ Please authenticate:                          ║\n" +
			"║   /register <username> <password>             ║\n" +
			"║   /login <username> <password>                ║\n" +
			"║                                              ║\n" +
			"║ Type /help for more commands                  ║\n" +
			"╚══════════════════════════════════════════════╝",
	}

	client.Send <- welcomeMsg

	reader := bufio.NewReader(conn)

	go func() {
		defer func() {
			h.Unregister <- client
			conn.Close()
		}()

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					log.Printf("Error reading from client %s: %v", client.Username, err)
				}
				break
			}

			line = []byte(strings.TrimSpace(string(line)))

			if len(line) > 0 && line[0] == '/' {
				command := strings.Fields(string(line))

				if len(command) > 0 {
					switch command[0] {
					case "/register":
						if len(command) < 3 {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: "Usage: /register <username> <password>",
							}
							continue
						}

						username := command[1]
						password := command[2]

						if len(username) < 3 {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: "Username must be at least 3 characters long",
							}
							continue
						}

						if len(password) < 4 {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: "Password must be at least 4 characters long",
							}
							continue
						}

						err := h.AuthManager.Register(username, password)
						if err != nil {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: fmt.Sprintf("Registration failed: %v", err),
							}
						} else {
							client.Send <- shared.Message{
								Type:   shared.TextMessage,
								Sender: "Server",
								Content: fmt.Sprintf("╔═════════════════════════════════════════╗\n"+
									"║ Registration Successful!                 ║\n"+
									"╠═════════════════════════════════════════╣\n"+
									"║ Username: %-30s ║\n"+
									"║                                         ║\n"+
									"║ You can now login with:                 ║\n"+
									"║ /login %s <password>                   ║\n"+
									"╚═════════════════════════════════════════╝",
									username, username),
							}
						}
						continue

					case "/login":
						if len(command) < 3 {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: "Usage: /login <username> <password>",
							}
							continue
						}

						username := command[1]
						password := command[2]

						err := h.AuthManager.Authenticate(username, password)
						if err != nil {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: fmt.Sprintf("Login failed: %v", err),
							}
							continue
						}

						isLoggedIn := false
						h.mu.RLock()
						for _, c := range h.Clients {
							if c.Username == username && c != client {
								isLoggedIn = true
								break
							}
						}
						h.mu.RUnlock()

						if isLoggedIn {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: fmt.Sprintf("User '%s' is already logged in", username),
							}
							continue
						}

						h.mu.Lock()
						delete(h.Clients, tempID)
						h.mu.Unlock()

						client.Username = username

						h.Register <- client

						client.Send <- shared.Message{
							Type:   shared.TextMessage,
							Sender: "Server",
							Content: fmt.Sprintf("╔═════════════════════════════════════════╗\n"+
								"║           Login Successful!               ║\n"+
								"╠═════════════════════════════════════════╣\n"+
								"║ Welcome back, %-27s ║\n"+
								"║                                         ║\n"+
								"║ You've been added to the 'general' room  ║\n"+
								"║ Type /help to see available commands     ║\n"+
								"╚═════════════════════════════════════════╝",
								username),
						}
						continue

					case "/logout":
						if client.Username == "" {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: "You are not logged in",
							}
							continue
						}

						oldUsername := client.Username

						h.Unregister <- client

						client = shared.NewClient(conn)
						tempID = conn.RemoteAddr().String() + "-" + fmt.Sprintf("%d", time.Now().UnixNano())

						h.mu.Lock()
						h.Clients[tempID] = client
						h.mu.Unlock()

						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("You have been logged out from account: %s", oldUsername),
						}
						continue

					case "/whoami":
						if client.Username == "" {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: "You are not logged in",
							}
						} else {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: fmt.Sprintf("You are logged in as %s", client.Username),
							}
						}
						continue

					case "/users":
						if client.Username == "" {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: "You must be logged in to see online users",
							}
							continue
						}

						users := h.getOnlineUsers()
						if len(users) == 0 {
							client.Send <- shared.Message{
								Type:    shared.TextMessage,
								Sender:  "Server",
								Content: "No users are online",
							}
							continue
						}

						var usersStr strings.Builder
						usersStr.WriteString("╔══════════════════════════════════════════╗\n")
						usersStr.WriteString("║            Online Users                  ║\n")
						usersStr.WriteString("╠══════════════════════════════════════════╣\n")

						for _, user := range users {
							if user == client.Username {
								usersStr.WriteString(fmt.Sprintf("║ %-39s ║\n", user+" (you)"))
							} else {
								usersStr.WriteString(fmt.Sprintf("║ %-39s ║\n", user))
							}
						}

						usersStr.WriteString("╚══════════════════════════════════════════╝")

						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: usersStr.String(),
						}
						continue

					case "/help":
						helpMsg := "╔══════════════════════════════════════════════════════════════╗\n" +
							"║                    Available Commands                         ║\n" +
							"╠══════════════════════════════════════════════════════════════╣\n" +
							"║ Authentication:                                              ║\n" +
							"║   /register <username> <password>  - Register a new account   ║\n" +
							"║   /login <username> <password>     - Login to your account    ║\n" +
							"║   /logout                         - Logout from your account  ║\n" +
							"║   /whoami                         - Display your username     ║\n" +
							"║                                                              ║\n" +
							"║ Room Management:                                             ║\n" +
							"║   /join <room>                    - Join a chat room          ║\n" +
							"║   /leave <room>                   - Leave a chat room         ║\n" +
							"║   /create <room>                  - Create a new room         ║\n" +
							"║   /list                           - List available rooms      ║\n" +
							"║                                                              ║\n" +
							"║ Messaging:                                                   ║\n" +
							"║   /msg <username> <message>       - Send a direct message     ║\n" +
							"║   /room <roomname> <message>      - Send to specific room     ║\n" +
							"║   /users                          - Show online users         ║\n" +
							"║                                                              ║\n" +
							"║ File Transfer:                                               ║\n" +
							"║   /file <username> <filepath>     - Send a file to a user     ║\n" +
							"║                                                              ║\n" +
							"║ Other:                                                       ║\n" +
							"║   /help                           - Show this help message    ║\n" +
							"║   /quit                           - Exit the chat client      ║\n" +
							"╚══════════════════════════════════════════════════════════════╝"

						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: helpMsg,
						}
						continue
					}
				}
			}

			var msg shared.Message
			err = json.Unmarshal(line, &msg)

			if err != nil {
				msg = shared.Message{
					Type:    shared.TextMessage,
					Content: string(line),
				}
			}

			msg.Sender = client.Username

			if client.Username == "" {
				client.Send <- shared.Message{
					Type:    shared.TextMessage,
					Sender:  "Server",
					Content: "You must login first. Use /login <username> <password> or register with /register <username> <password>",
				}
				continue
			}

			if IsCommand(msg.Content) {
				cmdMsg := ProcessCommand(msg.Content)
				cmdMsg.Sender = client.Username

				switch cmdMsg.Type {
				case shared.JoinRoomMessage:

					if client.IsInRoom(cmdMsg.RoomName) {
						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("You are already in room: %s", cmdMsg.RoomName),
						}
						continue
					}

					err := h.RoomManager.JoinRoom(cmdMsg.RoomName, client)
					if err != nil {
						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("Error joining room: %v", err),
						}
					} else {
						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("You have joined room: %s", cmdMsg.RoomName),
						}

						h.RoomManager.BroadcastToRoom(cmdMsg.RoomName, shared.Message{
							Type:     shared.TextMessage,
							Sender:   "Server",
							RoomName: cmdMsg.RoomName,
							Content:  fmt.Sprintf("%s has joined the room", client.Username),
						})
					}

				case shared.LeaveRoomMessage:
					if !client.IsInRoom(cmdMsg.RoomName) {
						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("You are not in room: %s", cmdMsg.RoomName),
						}
						continue
					}

					h.RoomManager.BroadcastToRoom(cmdMsg.RoomName, shared.Message{
						Type:     shared.TextMessage,
						Sender:   "Server",
						RoomName: cmdMsg.RoomName,
						Content:  fmt.Sprintf("%s has left the room", client.Username),
					})

					err := h.RoomManager.LeaveRoom(cmdMsg.RoomName, client)
					if err != nil {
						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("Error leaving room: %v", err),
						}
					} else {
						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("You have left room: %s", cmdMsg.RoomName),
						}
					}

				case shared.CreateRoomMessage:
					_, err := h.RoomManager.CreateRoom(cmdMsg.RoomName)
					if err != nil {
						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("Error creating room: %v", err),
						}
					} else {
						client.Send <- shared.Message{
							Type:    shared.TextMessage,
							Sender:  "Server",
							Content: fmt.Sprintf("Room created: %s", cmdMsg.RoomName),
						}
					}

				case shared.ListRoomsMessage:
					rooms := h.RoomManager.ListRooms()
					var roomsStr strings.Builder

					roomsStr.WriteString("╔══════════════════════════════════════════╗\n")
					roomsStr.WriteString("║            Available Rooms               ║\n")
					roomsStr.WriteString("╠══════════════════════════════════════════╣\n")

					for _, room := range rooms {
						isIn := ""
						if client.IsInRoom(room) {
							isIn = " (joined)"
						}
						roomsStr.WriteString(fmt.Sprintf("║ %-39s ║\n", room+isIn))
					}

					roomsStr.WriteString("╚══════════════════════════════════════════╝")

					client.Send <- shared.Message{
						Type:    shared.TextMessage,
						Sender:  "Server",
						Content: roomsStr.String(),
					}

				case shared.DirectMessage:
					h.DirectMsg <- cmdMsg

				case shared.TextMessage:

					if cmdMsg.RoomName != "" {

						if !client.IsInRoom(cmdMsg.RoomName) {
							client.Send <- shared.Message{
								Type:   shared.TextMessage,
								Sender: "Server",
								Content: fmt.Sprintf("You are not in room %s. Join it first with /join %s",
									cmdMsg.RoomName, cmdMsg.RoomName),
							}
							continue
						}

						h.Broadcast <- cmdMsg
					} else {
						client.Send <- cmdMsg
					}
				}
			} else if msg.Type == shared.FileTransferRequest ||
				msg.Type == shared.FileTransferData ||
				msg.Type == shared.FileTransferComplete {
				HandleFileTransfer(msg, conn, h.FileTransfer)
			} else {

				if msg.RoomName == "" {
					var activeRoom string
					for room := range client.Rooms {
						activeRoom = room
						break
					}

					if activeRoom == "" {
						activeRoom = "general"
					}

					msg.RoomName = activeRoom
				}

				if !client.IsInRoom(msg.RoomName) {
					client.Send <- shared.Message{
						Type:   shared.TextMessage,
						Sender: "Server",
						Content: fmt.Sprintf("You are not in room %s. Join it first with /join %s",
							msg.RoomName, msg.RoomName),
					}
					continue
				}

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
