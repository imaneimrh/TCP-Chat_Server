package shared

import (
	"net"
	"sync"
)

type MessageType int

const (
	TextMessage MessageType = iota
	JoinRoomMessage
	LeaveRoomMessage
	CreateRoomMessage
	ListRoomsMessage
	DirectMessage
	FileTransferRequest
	FileTransferData
	FileTransferComplete
)

type Message struct {
	Type       MessageType
	Sender     string
	Recipient  string
	RoomName   string
	Content    string
	FileData   []byte
	FileName   string
	FileSize   int
	FileOffset int
}

type Client struct {
	Conn     net.Conn
	Username string
	Rooms    map[string]bool
	Send     chan Message
	mu       sync.Mutex
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		Conn:     conn,
		Username: "",
		Rooms:    make(map[string]bool),
		Send:     make(chan Message, 100),
	}
}

func (c *Client) AddRoom(roomName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Rooms[roomName] = true
}

func (c *Client) RemoveRoom(roomName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Rooms, roomName)
}

func (c *Client) IsInRoom(roomName string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Rooms[roomName]
}
