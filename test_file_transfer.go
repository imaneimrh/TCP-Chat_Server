package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/imaneimrh/TCP-Chat_Server/client"
	"github.com/imaneimrh/TCP-Chat_Server/shared"
)

func main2() {
	testFilePath := "test_file.txt"
	if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
		f, err := os.Create(testFilePath)
		if err != nil {
			fmt.Printf("Error creating test file: %v\n", err)
			return
		}

		f.WriteString("This is a test file for the file transfer functionality.\n")
		f.WriteString("It contains multiple lines of text.\n")
		for i := 0; i < 100; i++ {
			f.WriteString(fmt.Sprintf("Line %d: testing file transfer.\n", i))
		}
		f.Close()

		fmt.Println("Created test file:", testFilePath)
	}

	fileTransfer := client.NewFileTransfer()

	msg, err := fileTransfer.InitiateTransfer("sender", "recipient", testFilePath)
	if err != nil {
		fmt.Printf("Error initiating transfer: %v\n", err)
		return
	}

	fmt.Printf("Initiated transfer of %s (%d bytes)\n", msg.FileName, msg.FileSize)

	transferID := "sender-recipient-" + filepath.Base(testFilePath)
	chunkCount := 0

	for {
		chunkMsg, err := fileTransfer.SendNextChunk(transferID)
		if err != nil {
			fmt.Printf("Error sending chunk: %v\n", err)
			break
		}

		chunkCount++
		fmt.Printf("Sent chunk %d: %d bytes (offset: %d)\n",
			chunkCount, len(chunkMsg.FileData), chunkMsg.FileOffset)

		err = fileTransfer.ReceiveChunk(*chunkMsg)
		if err != nil {
			fmt.Printf("Error receiving chunk: %v\n", err)
			break
		}

		progress, err := fileTransfer.GetTransferProgress("sender", msg.FileName)
		if err != nil {
			fmt.Printf("Error getting progress: %v\n", err)
		} else {
			fmt.Printf("Progress: %d%%\n", progress)
		}

		if chunkMsg.Type == shared.FileTransferComplete {
			fmt.Println("Transfer complete!")
			break
		}
	}

	receivedPath := filepath.Join("downloads", msg.FileName)
	if _, err := os.Stat(receivedPath); err == nil {
		fmt.Println("Success! File was saved to:", receivedPath)

		originalInfo, _ := os.Stat(testFilePath)
		receivedInfo, _ := os.Stat(receivedPath)

		fmt.Printf("Original size: %d bytes\n", originalInfo.Size())
		fmt.Printf("Received size: %d bytes\n", receivedInfo.Size())
	} else {
		fmt.Printf("Error: Received file not found: %v\n", err)
	}
}
