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

	"github.com/imaneimrh/TCP-Chat_Server/shared"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("╔═══════════════════════════════════════════════════════╗")
		fmt.Println("║              TCP Chat Client Usage                    ║")
		fmt.Println("╠═══════════════════════════════════════════════════════╣")
		fmt.Println("║ Usage: go run client.go <host> <port>                 ║")
		fmt.Println("║ Example: go run client.go localhost 8080              ║")
		fmt.Println("╚═══════════════════════════════════════════════════════╝")
		os.Exit(1)
	}

	host := os.Args[1]
	port := os.Args[2]

	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		fmt.Printf("Error connecting to server: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("╔═══════════════════════════════════════════════════════╗\n")
	fmt.Printf("║              TCP Chat Client Connected                ║\n")
	fmt.Printf("╠═══════════════════════════════════════════════════════╣\n")
	fmt.Printf("║ Server: %-45s ║\n", host+":"+port)
	fmt.Printf("║                                                       ║\n")
	fmt.Printf("║ Type /help for available commands                     ║\n")
	fmt.Printf("╚═══════════════════════════════════════════════════════╝\n")

	go func() {
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					fmt.Println("\n[Connection closed by server]")
				} else {
					fmt.Printf("\n[Error reading from server: %v]\n", err)
				}
				os.Exit(1)
			}

			var msg shared.Message
			err = json.Unmarshal(line, &msg)
			if err != nil {
				fmt.Println(strings.TrimSpace(string(line)))
				continue
			}

			if msg.Content == "PING" {
				continue
			}

			if msg.Type == shared.TextMessage {
				if msg.RoomName != "" && msg.Sender != "Server" {
					fmt.Printf("\n[%s] %s: %s\n", msg.RoomName, msg.Sender, msg.Content)
				} else {
					fmt.Printf("\n%s: %s\n", msg.Sender, msg.Content)
				}
			} else if msg.Type == shared.FileTransferRequest {
				fmt.Printf("\n[File Transfer Request] From %s: %s (%d bytes)\n",
					msg.Sender, msg.FileName, msg.FileSize)
			}
		}
	}()

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")
	for scanner.Scan() {
		input := scanner.Text()

		if strings.HasPrefix(input, "/file") {
			parts := strings.Fields(input)
			if len(parts) < 3 {
				fmt.Println("Usage: /file <username> <filepath>")
				fmt.Print("> ")
				continue
			}

			recipient := parts[1]
			filePath := parts[2]

			fileInfo, err := os.Stat(filePath)
			if err != nil {
				fmt.Printf("Error accessing file: %v\n", err)
				fmt.Print("> ")
				continue
			}

			if fileInfo.IsDir() {
				fmt.Println("Cannot send a directory, please specify a file")
				fmt.Print("> ")
				continue
			}

			fileRequest := shared.Message{
				Type:      shared.FileTransferRequest,
				Sender:    "",
				Recipient: recipient,
				FileName:  filepath.Base(filePath),
				FileSize:  int(fileInfo.Size()),
			}

			data, err := json.Marshal(fileRequest)
			if err != nil {
				fmt.Printf("Error encoding file request: %v\n", err)
				fmt.Print("> ")
				continue
			}
			_, err = conn.Write(append(data, '\n'))
			if err != nil {
				fmt.Printf("Error sending file request: %v\n", err)
				fmt.Print("> ")
				continue
			}

			file, err := os.Open(filePath)
			if err != nil {
				fmt.Printf("Error opening file: %v\n", err)
				fmt.Print("> ")
				continue
			}

			fmt.Printf("\n[Sending file %s to %s...]\n", filepath.Base(filePath), recipient)

			buffer := make([]byte, 8192)
			offset := 0

			var isLastChunk bool

			for {
				n, err := file.Read(buffer)
				if err != nil && err != io.EOF {
					fmt.Printf("Error reading file: %v\n", err)
					break
				}

				if n == 0 {
					break
				}

				isLastChunk = (err == io.EOF)

				fileChunk := shared.Message{
					Type:       shared.FileTransferData,
					Sender:     "",
					Recipient:  recipient,
					FileName:   filepath.Base(filePath),
					FileSize:   int(fileInfo.Size()),
					FileData:   buffer[:n],
					FileOffset: offset,
				}

				if isLastChunk {
					fileChunk.Type = shared.FileTransferComplete
				}

				chunkData, err := json.Marshal(fileChunk)
				if err != nil {
					fmt.Printf("Error encoding file chunk: %v\n", err)
					break
				}
				_, err = conn.Write(append(chunkData, '\n'))
				if err != nil {
					fmt.Printf("Error sending file chunk: %v\n", err)
					break
				}

				offset += n

				progress := float64(offset) / float64(fileInfo.Size()) * 100
				progressBar := generateProgressBar(int(progress), 40)
				fmt.Printf("\r[Progress: %.1f%% %s]", progress, progressBar)

				if isLastChunk {
					fmt.Println("\n[File transfer complete!]")
					break
				}

				time.Sleep(10 * time.Millisecond)
			}
			if !isLastChunk {
				completeMsg := shared.Message{
					Type:       shared.FileTransferComplete,
					Sender:     "",
					Recipient:  recipient,
					FileName:   filepath.Base(filePath),
					FileSize:   int(fileInfo.Size()),
					FileOffset: offset,
				}

				completeData, err := json.Marshal(completeMsg)
				if err != nil {
					fmt.Printf("Error encoding completion message: %v\n", err)
				} else {
					_, err = conn.Write(append(completeData, '\n'))
					if err != nil {
						fmt.Printf("Error sending completion message: %v\n", err)
					}
					time.Sleep(50 * time.Millisecond)
				}
			}

			file.Close()
			fmt.Print("> ")
			continue
		}

		_, err := fmt.Fprintf(conn, "%s\n", input)
		if err != nil {
			fmt.Printf("Error sending message: %v\n", err)
			break
		}

		if input == "/quit" {
			fmt.Println("\nDisconnecting from chat server...")
			break
		}

		fmt.Print("> ")
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input: %v\n", err)
	}
}

func generateProgressBar(progress int, width int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	completed := width * progress / 100
	remaining := width - completed

	bar := "["
	for i := 0; i < completed; i++ {
		bar += "="
	}

	if completed < width {
		bar += ">"
		remaining--
	}

	for i := 0; i < remaining; i++ {
		bar += " "
	}

	bar += "]"

	return bar
}
