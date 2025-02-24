package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

type Server struct {
	Addr         string
	Clients      map[string]*Client
	ClientsMutex sync.RWMutex
	AuthManager  *AuthManager
}

type Client struct {
	ID            string
	Conn          net.Conn
	Writer        *bufio.Writer
	Server        *Server
	Username      string
	Authenticated bool
}

func NewServer(addr string) *Server {
	return &Server{
		Addr:        addr,
		Clients:     make(map[string]*Client),
		AuthManager: NewAuthManager(),
	}
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("Server started on %s", s.Addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	clientID := conn.RemoteAddr().String()
	client := &Client{
		ID:            clientID,
		Conn:          conn,
		Writer:        bufio.NewWriter(conn),
		Server:        s,
		Authenticated: false,
	}
	s.registerClient(client)
	client.Send("Welcome to the TCP Chat Server!")
	client.Send("Please login with /login <username> <password> or register with /register <username> <password>")
	client.Send("Type /help for available commands")

	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		message := scanner.Text()

		s.processMessage(client, message)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from client %s: %v", client.ID, err)
	}

	s.removeClient(client)
}

func (s *Server) processMessage(client *Client, message string) {
	if strings.HasPrefix(message, "/") {
		s.handleCommand(client, message)
		return
	}

	if client.Authenticated {
		log.Printf("Message from %s: %s", client.Username, message)
		s.Broadcast(fmt.Sprintf("%s: %s", client.Username, message), client)
	} else {
		client.Send("You must login first. Use /login <username> <password> or register with /register <username> <password>")
	}
}

func (s *Server) registerClient(client *Client) {
	s.ClientsMutex.Lock()
	defer s.ClientsMutex.Unlock()

	s.Clients[client.ID] = client
	log.Printf("New client connected: %s", client.ID)
}

func (s *Server) removeClient(client *Client) {
	s.ClientsMutex.Lock()
	defer s.ClientsMutex.Unlock()

	if _, exists := s.Clients[client.ID]; exists {
		delete(s.Clients, client.ID)
		client.Conn.Close()
		log.Printf("Client disconnected: %s", client.ID)
	}
}

func (s *Server) Broadcast(message string, sender *Client) {
	s.ClientsMutex.RLock()
	defer s.ClientsMutex.RUnlock()

	for _, client := range s.Clients {
		if client.ID != sender.ID {
			client.Send(message)
		}
	}
}

func (c *Client) Send(message string) {
	_, err := c.Writer.WriteString(message + "\n")
	if err != nil {
		log.Printf("Error sending message to client %s: %v", c.ID, err)
		return
	}

	err = c.Writer.Flush()
	if err != nil {
		log.Printf("Error flushing message to client %s: %v", c.ID, err)
	}
}

func (s *Server) handleCommand(client *Client, message string) {
	parts := strings.Fields(message)
	if len(parts) == 0 {
		return
	}
	command := parts[0]
	switch command {
	case "/register":
		if len(parts) < 3 {
			client.Send("Usage: /register <username> <password>")
			return
		}

		username := parts[1]
		password := parts[2]

		if len(username) < 3 {
			client.Send("Username must be at least 3 characters long")
			return
		}

		if len(password) < 4 {
			client.Send("Password must be at least 4 characters long")
			return
		}

		err := s.AuthManager.Register(username, password)
		if err != nil {
			client.Send(fmt.Sprintf("Registration failed: %v", err))
			return
		}

		client.Send(fmt.Sprintf("Successfully registered user '%s'. You can now login with /login %s <password>", username, username))

	case "/login":
		if len(parts) < 3 {
			client.Send("Usage: /login <username> <password>")
			return
		}

		username := parts[1]
		password := parts[2]

		if client.Authenticated {
			client.Send(fmt.Sprintf("You are already logged in as %s", client.Username))
			return
		}

		err := s.AuthManager.Authenticate(username, password)
		if err != nil {
			client.Send(fmt.Sprintf("Login failed: %v", err))
			return
		}

		if s.isUserLoggedIn(username) {
			client.Send(fmt.Sprintf("User '%s' is already logged in from another session", username))
			return
		}

		client.Username = username
		client.Authenticated = true

		client.Send(fmt.Sprintf("Welcome, %s! You are now logged in.", username))
		s.Broadcast(fmt.Sprintf("User %s has joined the chat", username), client)

	case "/logout":
		if !client.Authenticated {
			client.Send("You are not logged in")
			return
		}

		oldUsername := client.Username
		client.Username = ""
		client.Authenticated = false

		client.Send("You have been logged out")
		s.Broadcast(fmt.Sprintf("User %s has left the chat", oldUsername), client)

	case "/whoami":
		if client.Authenticated {
			client.Send(fmt.Sprintf("You are logged in as %s", client.Username))
		} else {
			client.Send("You are not logged in")
		}

	case "/users":
		if !client.Authenticated {
			client.Send("You must be logged in to see online users")
			return
		}

		users := s.getOnlineUsers()
		if len(users) == 0 {
			client.Send("No users are online")
			return
		}

		client.Send("Online users:")
		for _, user := range users {
			client.Send("- " + user)
		}

	case "/help":
		client.Send("Available commands:")
		client.Send("/register <username> <password> - Register a new account")
		client.Send("/login <username> <password> - Login to your account")
		client.Send("/logout - Logout from your account")
		client.Send("/whoami - Display your username")
		client.Send("/users - Show online users")
		client.Send("/help - Show this help message")

	default:
		client.Send(fmt.Sprintf("Unknown command: %s. Type /help for available commands.", command))
	}
}

func (s *Server) isUserLoggedIn(username string) bool {
	s.ClientsMutex.RLock()
	defer s.ClientsMutex.RUnlock()

	for _, client := range s.Clients {
		if client.Authenticated && client.Username == username {
			return true
		}
	}
	return false
}

func (s *Server) getOnlineUsers() []string {
	s.ClientsMutex.RLock()
	defer s.ClientsMutex.RUnlock()

	users := []string{}
	for _, client := range s.Clients {
		if client.Authenticated {
			users = append(users, client.Username)
		}
	}
	return users
}
