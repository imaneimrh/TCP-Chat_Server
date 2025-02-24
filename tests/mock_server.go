package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"TCP-CHAT_SERVER/client"
	"TCP-CHAT_SERVER/room"
)

func main() {
	roomManager := room.NewManager()
	handler := client.NewHandler(roomManager)
	go handler.Run()

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Error creating listener: %v", err)
	}
	defer listener.Close()

	fmt.Println("Mock server started on :8080")
	fmt.Println("No authentication is implemented - users will be auto-assigned a username")

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-shutdown
		fmt.Println("\nShutting down server...")
		listener.Close()
		os.Exit(0)
	}()

	userCount := 0

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		userCount++
		username := fmt.Sprintf("user%d", userCount)

		fmt.Printf("New connection from %s, assigned username: %s\n",
			conn.RemoteAddr().String(), username)

		go handler.HandleClient(conn, username)
	}
}
