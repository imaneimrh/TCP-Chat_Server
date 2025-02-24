package room

import (
	"fmt"
	"sync"

	"github.com/imaneimrh/TCP-Chat_Server/shared"
)

type Manager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

func NewManager() *Manager {
	manager := &Manager{
		rooms: make(map[string]*Room),
	}

	manager.CreateRoom("general")

	return manager
}

func (m *Manager) CreateRoom(name string) (*Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rooms[name]; exists {
		return nil, fmt.Errorf("Room %s already exists", name)
	}

	newRoom := NewRoom(name)
	m.rooms[name] = newRoom

	go newRoom.Run()

	return newRoom, nil
}

func (m *Manager) GetRoom(name string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	room, exists := m.rooms[name]
	return room, exists
}

func (m *Manager) DeleteRoom(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, exists := m.rooms[name]
	if !exists {
		return fmt.Errorf("Room %s does not exist", name)
	}

	if room.GetClientCount() > 0 {
		return fmt.Errorf("cannot delete room %s: room is not empty", name)
	}

	if name == "general" {
		return fmt.Errorf("cannot delete the general room")
	}

	delete(m.rooms, name)
	return nil
}

func (m *Manager) JoinRoom(roomName string, client *shared.Client) error {
	room, exists := m.GetRoom(roomName)
	if !exists {
		return fmt.Errorf("Room %s does not exist", roomName)
	}

	room.Register <- client
	return nil
}

func (m *Manager) LeaveRoom(roomName string, client *shared.Client) error {
	room, exists := m.GetRoom(roomName)
	if !exists {
		return fmt.Errorf("Room %s does not exist", roomName)
	}

	room.Unregister <- client
	return nil
}

func (m *Manager) BroadcastToRoom(roomName string, message shared.Message) error {
	room, exists := m.GetRoom(roomName)
	if !exists {
		return fmt.Errorf("Room %s does not exist", roomName)
	}

	room.Broadcast <- message
	return nil
}

func (m *Manager) ListRooms() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	roomList := make([]string, 0, len(m.rooms))
	for name := range m.rooms {
		roomList = append(roomList, name)
	}

	return roomList
}
