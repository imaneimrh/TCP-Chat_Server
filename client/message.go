package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/imaneimrh/TCP-Chat_Server/shared"
)

func ParseMessage(data []byte) (shared.Message, error) {
	var msg shared.Message
	err := json.Unmarshal(data, &msg)
	if err != nil {

		msg = shared.Message{
			Type:    shared.TextMessage,
			Content: string(data),
		}
	}
	return msg, nil
}

func FormatMessage(msg shared.Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func ReadMessage(reader *bufio.Reader) (shared.Message, error) {
	data, err := reader.ReadBytes('\n')
	if err != nil {
		return shared.Message{}, err
	}
	data = bytes.TrimSpace(data)
	msg, err := ParseMessage(data)
	if err != nil {
		return shared.Message{}, err
	}

	return msg, nil
}

func IsCommand(content string) bool {
	return len(content) > 0 && content[0] == '/'
}

func ProcessCommand(content string) shared.Message {
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return shared.Message{Type: shared.TextMessage, Content: content}
	}

	command := parts[0]

	switch command {
	case "/join":
		if len(parts) < 2 {
			return shared.Message{
				Type:    shared.TextMessage,
				Sender:  "Server",
				Content: "Usage: /join <room>",
			}
		}
		roomName := parts[1]
		return shared.Message{
			Type:     shared.JoinRoomMessage,
			RoomName: roomName,
		}

	case "/leave":
		if len(parts) < 2 {
			return shared.Message{
				Type:    shared.TextMessage,
				Sender:  "Server",
				Content: "Usage: /leave <room>",
			}
		}
		roomName := parts[1]
		return shared.Message{
			Type:     shared.LeaveRoomMessage,
			RoomName: roomName,
		}

	case "/create":
		if len(parts) < 2 {
			return shared.Message{
				Type:    shared.TextMessage,
				Sender:  "Server",
				Content: "Usage: /create <room>",
			}
		}
		roomName := parts[1]
		return shared.Message{
			Type:     shared.CreateRoomMessage,
			RoomName: roomName,
		}

	case "/list":
		return shared.Message{
			Type: shared.ListRoomsMessage,
		}

	case "/msg":
		if len(parts) < 3 {
			return shared.Message{
				Type:    shared.TextMessage,
				Sender:  "Server",
				Content: "Usage: /msg <username> <message>",
			}
		}
		recipient := parts[1]
		content := strings.Join(parts[2:], " ")
		return shared.Message{
			Type:      shared.DirectMessage,
			Recipient: recipient,
			Content:   content,
		}

	case "/room":
		if len(parts) < 3 {
			return shared.Message{
				Type:    shared.TextMessage,
				Sender:  "Server",
				Content: "Usage: /room <roomname> <message>",
			}
		}
		roomName := parts[1]
		content := strings.Join(parts[2:], " ")
		return shared.Message{
			Type:     shared.TextMessage,
			RoomName: roomName,
			Content:  content,
		}

	case "/logout":
		return shared.Message{
			Type:    shared.TextMessage,
			Content: "You have been logged out.",
		}

	case "/whoami":
		return shared.Message{
			Type:    shared.TextMessage,
			Content: "Current username",
		}

	case "/users":
		return shared.Message{
			Type:    shared.TextMessage,
			Content: "List of online users",
		}

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

		return shared.Message{
			Type:    shared.TextMessage,
			Sender:  "Server",
			Content: helpMsg,
		}

	default:
		return shared.Message{
			Type:    shared.TextMessage,
			Content: content,
		}
	}
}

func HandleFileTransfer(msg shared.Message, writer io.Writer, ft *FileTransfer) error {
	switch msg.Type {
	case shared.FileTransferRequest:
		response := shared.Message{
			Type:   shared.TextMessage,
			Sender: "Server",
			Content: fmt.Sprintf("╔════════════════════════════════════════════════════╗\n"+
				"║           File Transfer Request                    ║\n"+
				"╠════════════════════════════════════════════════════╣\n"+
				"║ File: %-43s ║\n"+
				"║ Size: %-7.2f KB                                 ║\n"+
				"║ From: %-43s ║\n"+
				"║                                                ║\n"+
				"║ Transfer initiated, please wait...             ║\n"+
				"╚════════════════════════════════════════════════════╝",
				msg.FileName, float64(msg.FileSize)/1024, msg.Sender),
			Recipient: msg.Recipient,
		}

		err := os.MkdirAll("downloads", 0755)
		if err != nil {
			return fmt.Errorf("failed to create downloads directory: %v", err)
		}

		data, err := FormatMessage(response)
		if err != nil {
			return err
		}

		_, err = writer.Write(data)
		if err != nil {
			return err
		}

	case shared.FileTransferData:
		err := ft.ReceiveChunk(msg)
		if err != nil {
			return fmt.Errorf("error receiving file chunk: %v", err)
		}

		progress, err := ft.GetTransferProgress(msg.Sender, msg.FileName)
		if err != nil {
			log.Printf("Error getting transfer progress: %v", err)
		} else {
			progressBar := generateProgressBar(progress, 40)

			response := shared.Message{
				Type:      shared.TextMessage,
				Sender:    "Server",
				Recipient: msg.Sender,
				Content:   fmt.Sprintf("File transfer progress: %d%% %s", progress, progressBar),
			}

			data, err := FormatMessage(response)
			if err != nil {
				return err
			}

			_, err = writer.Write(data)
			if err != nil {
				return err
			}
		}

	case shared.FileTransferComplete:
		err := ft.ReceiveChunk(msg)
		if err != nil {
			return fmt.Errorf("error receiving final file chunk: %v", err)
		}

		recipientMsg := shared.Message{
			Type:      shared.TextMessage,
			Sender:    "Server",
			Recipient: msg.Recipient,
			Content: fmt.Sprintf("╔════════════════════════════════════════════════════╗\n"+
				"║           File Transfer Complete                   ║\n"+
				"╠════════════════════════════════════════════════════╣\n"+
				"║ File: %-43s ║\n"+
				"║ Size: %-7.2f KB                                 ║\n"+
				"║ From: %-43s ║\n"+
				"║                                                ║\n"+
				"║ File saved to: downloads/%-27s ║\n"+
				"╚════════════════════════════════════════════════════╝",
				msg.FileName, float64(msg.FileSize)/1024, msg.Sender, msg.FileName),
		}

		data, err := FormatMessage(recipientMsg)
		if err != nil {
			return err
		}

		_, err = writer.Write(data)
		if err != nil {
			return err
		}

		senderMsg := shared.Message{
			Type:      shared.TextMessage,
			Sender:    "Server",
			Recipient: msg.Sender,
			Content: fmt.Sprintf("╔════════════════════════════════════════════════════╗\n"+
				"║           File Transfer Complete                   ║\n"+
				"╠════════════════════════════════════════════════════╣\n"+
				"║ File: %-43s ║\n"+
				"║ Size: %-7.2f KB                                 ║\n"+
				"║ To:   %-43s ║\n"+
				"║                                                ║\n"+
				"║ File successfully transferred                   ║\n"+
				"╚════════════════════════════════════════════════════╝",
				msg.FileName, float64(msg.FileSize)/1024, msg.Recipient),
		}

		data, err = FormatMessage(senderMsg)
		if err != nil {
			return err
		}

		_, err = writer.Write(data)
		if err != nil {
			return err
		}
	}

	return nil
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
