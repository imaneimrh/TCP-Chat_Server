package auth

import (
	"fmt"
	"log"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username     string
	PasswordHash string
}

type Manager struct {
	Users      map[string]User
	UsersMutex sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		Users: make(map[string]User),
	}
}

func (am *Manager) Register(username, password string) error {
	am.UsersMutex.Lock()
	defer am.UsersMutex.Unlock()

	if _, exists := am.Users[username]; exists {
		return fmt.Errorf("username '%s' already exists", username)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error hashing password: %w", err)
	}

	am.Users[username] = User{
		Username:     username,
		PasswordHash: string(hashedPassword),
	}

	log.Printf("User '%s' registered successfully", username)
	return nil
}

func (am *Manager) Authenticate(username, password string) error {
	am.UsersMutex.RLock()
	defer am.UsersMutex.RUnlock()

	user, exists := am.Users[username]
	if !exists {
		return fmt.Errorf("invalid username or password")
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return fmt.Errorf("invalid username or password")
	}

	return nil
}

func (am *Manager) GetUser(username string) (User, bool) {
	am.UsersMutex.RLock()
	defer am.UsersMutex.RUnlock()

	user, exists := am.Users[username]
	return user, exists
}

func (am *Manager) ListUsers() []string {
	am.UsersMutex.RLock()
	defer am.UsersMutex.RUnlock()

	users := make([]string, 0, len(am.Users))
	for username := range am.Users {
		users = append(users, username)
	}

	return users
}

func (am *Manager) IsUserLoggedIn(username string, clients map[string]interface{}) bool {
	am.UsersMutex.RLock()
	defer am.UsersMutex.RUnlock()

	_, exists := am.Users[username]
	return exists && clients != nil
}
