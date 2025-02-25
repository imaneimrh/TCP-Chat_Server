package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {

	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected to server!")

	go func() {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			fmt.Println("Server:", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading from server:", err)
		}
		fmt.Println("Lost connection to server")
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		if message == "/quit" {
			fmt.Println("Disconnecting...")
			break
		}

		_, err := fmt.Fprintf(conn, "%s\n", message)
		if err != nil {
			fmt.Println("Error sending message:", err)
			break
		}
	}
}
