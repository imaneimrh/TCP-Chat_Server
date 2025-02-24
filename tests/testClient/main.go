package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"TCP-CHAT_SERVER/shared"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run main.go <host> <port> <username>")
		os.Exit(1)
	}

	host := os.Args[1]
	port := os.Args[2]
	username := os.Args[3]

	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("Connected to server at %s:%s\n", host, port)

	authMsg := shared.Message{
		Type:    shared.TextMessage,
		Sender:  username,
		Content: "AUTH " + username,
	}
	sendMessage(conn, authMsg)

	go func() {
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				fmt.Printf("Error reading from server: %v\n", err)
				os.Exit(1)
			}

			var msg shared.Message
			err = json.Unmarshal(line, &msg)
			if err != nil {
				fmt.Printf("Error parsing message: %v\n", err)
				continue
			}

			if msg.Type == shared.TextMessage {
				if msg.RoomName != "" {
					fmt.Printf("[%s] %s: %s\n", msg.RoomName, msg.Sender, msg.Content)
				} else {
					fmt.Printf("%s: %s\n", msg.Sender, msg.Content)
				}
			} else if msg.Type == shared.FileTransferRequest {
				fmt.Printf("File transfer request from %s: %s (%d bytes)\n",
					msg.Sender, msg.FileName, msg.FileSize)
			}
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter messages (or type '/help' for commands):")
	for scanner.Scan() {
		input := scanner.Text()

		if strings.HasPrefix(input, "/") {
			parts := strings.Fields(input)
			command := parts[0]

			if command == "/help" {
				fmt.Println("Available commands:")
				fmt.Println("  /join <room>     - Join a room")
				fmt.Println("  /leave <room>    - Leave a room")
				fmt.Println("  /create <room>   - Create a new room")
				fmt.Println("  /list            - List available rooms")
				fmt.Println("  /msg <user> <message> - Send a direct message")
				fmt.Println("  /file <user> <filename> - Send a file")
				fmt.Println("  /quit            - Exit the client")
				continue
			} else if command == "/quit" {
				fmt.Println("Goodbye!")
				return
			} else if command == "/file" {
				if len(parts) < 3 {
					fmt.Println("Usage: /file <username> <filepath>")
					continue
				}

				recipient := parts[1]
				filePath := parts[2]

				fileInfo, err := os.Stat(filePath)
				if err != nil {
					fmt.Printf("Error accessing file: %v\n", err)
					continue
				}

				if fileInfo.IsDir() {
					fmt.Println("Cannot send a directory, please specify a file")
					continue
				}

				fileRequest := shared.Message{
					Type:      shared.FileTransferRequest,
					Sender:    username,
					Recipient: recipient,
					FileName:  filepath.Base(filePath),
					FileSize:  int(fileInfo.Size()),
				}
				sendMessage(conn, fileRequest)

				file, err := os.Open(filePath)
				if err != nil {
					fmt.Printf("Error opening file: %v\n", err)
					continue
				}

				fmt.Printf("Sending file %s to %s...\n", filepath.Base(filePath), recipient)

				buffer := make([]byte, 8192) 
				offset := 0

				for {
					n, err := file.Read(buffer)
					if err != nil && err != io.EOF {
						fmt.Printf("Error reading file: %v\n", err)
						break
					}

					if n == 0 {
						break
					}

					fileChunk := shared.Message{
						Type:       shared.FileTransferData,
						Sender:     username,
						Recipient:  recipient,
						FileName:   filepath.Base(filePath),
						FileSize:   int(fileInfo.Size()),
						FileData:   buffer[:n],
						FileOffset: offset,
					}

					if err == io.EOF {
						fileChunk.Type = shared.FileTransferComplete
					}

					sendMessage(conn, fileChunk)

					offset += n

					progress := float64(offset) / float64(fileInfo.Size()) * 100
					fmt.Printf("\rProgress: %.1f%%", progress)

					if err == io.EOF {
						fmt.Println("\nFile transfer complete!")
						break
					}

					time.Sleep(10 * time.Millisecond)
				}

				file.Close()
				continue
			}
		}

		msg := shared.Message{
			Type:    shared.TextMessage,
			Sender:  username,
			Content: input,
		}
		sendMessage(conn, msg)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input: %v\n", err)
	}
}

func sendMessage(conn net.Conn, msg shared.Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Error encoding message: %v\n", err)
		return
	}

	data = append(data, '\n')

	_, err = conn.Write(data)
	if err != nil {
		fmt.Printf("Error sending message: %v\n", err)
		return
	}
}
