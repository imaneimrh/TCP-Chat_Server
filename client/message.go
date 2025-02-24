package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"TCP-CHAT_SERVER/shared"
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
	parts := bytes.Fields([]byte(content))
	if len(parts) == 0 {
		return shared.Message{Type: shared.TextMessage, Content: content}
	}

	command := string(parts[0])

	switch command {
	case "/join":
		if len(parts) < 2 {
			return shared.Message{
				Type:    shared.TextMessage,
				Sender:  "Server",
				Content: "Usage: /join <room>",
			}
		}
		roomName := string(parts[1])
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
		roomName := string(parts[1])
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
		roomName := string(parts[1])
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
		recipient := string(parts[1])
		content := string(bytes.Join(parts[2:], []byte(" ")))
		return shared.Message{
			Type:      shared.DirectMessage,
			Recipient: recipient,
			Content:   content,
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
			Content: fmt.Sprintf("File transfer initiated: %s (%.2f KB)",
				msg.FileName, float64(msg.FileSize)/1024),
			Recipient: msg.Sender,
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
			response := shared.Message{
				Type:      shared.TextMessage,
				Sender:    "Server",
				Recipient: msg.Sender,
				Content:   fmt.Sprintf("File transfer progress: %d%%", progress),
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

		response := shared.Message{
			Type:      shared.TextMessage,
			Sender:    "Server",
			Recipient: msg.Recipient,
			Content:   fmt.Sprintf("File received: %s", msg.FileName),
		}

		data, err := FormatMessage(response)
		if err != nil {
			return err
		}

		_, err = writer.Write(data)
		if err != nil {
			return err
		}

		senderNotify := shared.Message{
			Type:      shared.TextMessage,
			Sender:    "Server",
			Recipient: msg.Sender,
			Content:   fmt.Sprintf("File transfer complete: %s", msg.FileName),
		}

		data, err = FormatMessage(senderNotify)
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
