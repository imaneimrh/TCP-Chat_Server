package server

import (
	"log"
	"net"
	"sync"

	"github.com/imaneimrh/TCP-Chat_Server/auth"
	"github.com/imaneimrh/TCP-Chat_Server/room"
)

type ClientHandler interface {
	HandleClient(conn net.Conn)
	Run()
}

type Server struct {
	Addr        string
	Handler     ClientHandler
	RoomManager *room.Manager
	AuthManager *auth.Manager
	mu          sync.RWMutex
}

func NewServerWithHandler(addr string, handler ClientHandler, roomManager *room.Manager, authManager *auth.Manager) *Server {
	return &Server{
		Addr:        addr,
		Handler:     handler,
		RoomManager: roomManager,
		AuthManager: authManager,
	}
}

func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("TCP Chat Server started on %s", s.Addr)
	log.Printf("The server supports authentication, rooms, direct messaging, and file transfers")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		log.Printf("New connection from: %s", conn.RemoteAddr().String())

		go s.Handler.HandleClient(conn)
	}
}
