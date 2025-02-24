package server

import (
	"log"
	"net"
)

type Server struct {
	Addr string
}

func NewServer(addr string) *Server {
	return &Server{
		Addr: addr,
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
	clientAddr := conn.RemoteAddr().String()
	log.Printf("New connection from %s", clientAddr)

	conn.Write([]byte("Welcome to the TCP Chat Server!\n"))

	buffer := make([]byte, 1024)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			log.Printf("Client %s disconnected: %v", clientAddr, err)
			break
		}

		message := string(buffer[:n])
		log.Printf("Received from %s: %s", clientAddr, message)
		conn.Write([]byte("Echo: " + message))
	}

	conn.Close()
	log.Printf("Connection closed: %s", clientAddr)
}
