package server

import (
	"bufio"
	"log"
	"net"
	"sync"
)

type Server struct {
	Addr         string
	Clients      map[string]*Client
	ClientsMutex sync.RWMutex
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
		Addr:    addr,
		Clients: make(map[string]*Client),
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
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		message := scanner.Text()

		log.Printf("Received from %s: %s", client.ID, message)

		client.Send("Echo: " + message)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from client %s: %v", client.ID, err)
	}

	s.removeClient(client)
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
