package client

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"TCP-CHAT_SERVER/shared"
)

const (
	MaxChunkSize = 8192
)

type FileTransfer struct {
	pendingTransfers map[string]*FileTransferInfo
	activeTransfers  map[string]map[string]*FileTransferInfo
	mu               sync.Mutex
}

type FileTransferInfo struct {
	Sender     string
	Recipient  string
	FileName   string
	FilePath   string
	FileSize   int64
	BytesSent  int64
	BytesRead  int64
	File       *os.File
	IsComplete bool
}

func NewFileTransfer() *FileTransfer {
	return &FileTransfer{
		pendingTransfers: make(map[string]*FileTransferInfo),
		activeTransfers:  make(map[string]map[string]*FileTransferInfo),
	}
}

func (ft *FileTransfer) InitiateTransfer(sender, recipient, filePath string) (*shared.Message, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}

	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	fileName := filepath.Base(filePath)

	transferID := fmt.Sprintf("%s-%s-%s", sender, recipient, fileName)

	ft.mu.Lock()
	transfer := &FileTransferInfo{
		Sender:     sender,
		Recipient:  recipient,
		FileName:   fileName,
		FilePath:   filePath,
		FileSize:   fileInfo.Size(),
		BytesSent:  0,
		File:       file,
		IsComplete: false,
	}

	if _, exists := ft.activeTransfers[recipient]; !exists {
		ft.activeTransfers[recipient] = make(map[string]*FileTransferInfo)
	}
	ft.activeTransfers[recipient][fileName] = transfer
	ft.pendingTransfers[transferID] = transfer
	ft.mu.Unlock()

	msg := &shared.Message{
		Type:      shared.FileTransferRequest,
		Sender:    sender,
		Recipient: recipient,
		FileName:  fileName,
		FileSize:  int(fileInfo.Size()),
	}

	return msg, nil
}

func (ft *FileTransfer) SendNextChunk(transferID string) (*shared.Message, error) {
	ft.mu.Lock()
	transfer, exists := ft.pendingTransfers[transferID]
	ft.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("transfer %s not found", transferID)
	}

	if transfer.IsComplete {
		return nil, fmt.Errorf("transfer %s is already complete", transferID)
	}

	buffer := make([]byte, MaxChunkSize)

	n, err := transfer.File.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	transfer.BytesRead += int64(n)

	isComplete := err == io.EOF

	if isComplete {
		transfer.File.Close()
		transfer.IsComplete = true
	}

	msg := &shared.Message{
		Type:       shared.FileTransferData,
		Sender:     transfer.Sender,
		Recipient:  transfer.Recipient,
		FileName:   transfer.FileName,
		FileData:   buffer[:n],
		FileSize:   int(transfer.FileSize),
		FileOffset: int(transfer.BytesSent),
	}

	transfer.BytesSent += int64(n)

	if isComplete {
		msg.Type = shared.FileTransferComplete
	}

	return msg, nil
}

func (ft *FileTransfer) ReceiveChunk(msg shared.Message) error {
	sender := msg.Sender
	fileName := msg.FileName

	ft.mu.Lock()
	var transfer *FileTransferInfo
	senderTransfers, exists := ft.activeTransfers[sender]

	if !exists {
		senderTransfers = make(map[string]*FileTransferInfo)
		ft.activeTransfers[sender] = senderTransfers
	}

	transfer, exists = senderTransfers[fileName]

	if !exists {
		err := os.MkdirAll("downloads", 0755)
		if err != nil {
			ft.mu.Unlock()
			return fmt.Errorf("failed to create downloads directory: %v", err)
		}

		filePath := filepath.Join("downloads", fileName)
		file, err := os.Create(filePath)
		if err != nil {
			ft.mu.Unlock()
			return fmt.Errorf("failed to create file: %v", err)
		}

		transfer = &FileTransferInfo{
			Sender:     sender,
			Recipient:  msg.Recipient,
			FileName:   fileName,
			FilePath:   filePath,
			FileSize:   int64(msg.FileSize),
			BytesRead:  0,
			File:       file,
			IsComplete: false,
		}

		senderTransfers[fileName] = transfer
	}
	ft.mu.Unlock()

	_, err := transfer.File.WriteAt(msg.FileData, int64(msg.FileOffset))
	if err != nil {
		return fmt.Errorf("error writing to file: %v", err)
	}

	transfer.BytesRead += int64(len(msg.FileData))

	if msg.Type == shared.FileTransferComplete {
		transfer.File.Close()
		transfer.IsComplete = true
	}

	return nil
}

func (ft *FileTransfer) GetTransferProgress(sender, fileName string) (int, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	senderTransfers, exists := ft.activeTransfers[sender]
	if !exists {
		return 0, fmt.Errorf("no transfers from %s", sender)
	}

	transfer, exists := senderTransfers[fileName]
	if !exists {
		return 0, fmt.Errorf("no transfer of %s from %s", fileName, sender)
	}

	if transfer.FileSize == 0 {
		return 100, nil
	}

	return int((transfer.BytesRead * 100) / transfer.FileSize), nil
}
