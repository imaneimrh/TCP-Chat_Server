package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/imaneimrh/TCP-Chat_Server/auth"
	"github.com/imaneimrh/TCP-Chat_Server/client"
	"github.com/imaneimrh/TCP-Chat_Server/room"
	"github.com/imaneimrh/TCP-Chat_Server/server"
)

func main() {

	authManager := auth.NewManager()
	roomManager := room.NewManager()

	handler := client.NewHandler(roomManager, authManager)

	go handler.Run()

	chatServer := server.NewServerWithHandler(":8080", handler, roomManager, authManager)

	go func() {
		log.Println("Starting integrated TCP Chat Server...")
		err := chatServer.Start()
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutting down server...")

	time.Sleep(time.Second)
	log.Println("Server stopped")
}
