package memory

import (
	"strings"
	"sync"

	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/chat"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type Store struct {
	mu           sync.RWMutex
	users        map[string]domain.User
	usersByEmail map[string]string
	sessions     map[string]domain.Session
	chats        map[string]domain.Chat
	messages     map[string][]domain.Message
}

func NewStore() *Store {
	return &Store{
		users:        make(map[string]domain.User),
		usersByEmail: make(map[string]string),
		sessions:     make(map[string]domain.Session),
		chats:        make(map[string]domain.Chat),
		messages:     make(map[string][]domain.Message),
	}
}

func (s *Store) CreateUser(user domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	emailKey := strings.ToLower(user.Email)
	if _, exists := s.usersByEmail[emailKey]; exists {
		return auth.ErrEmailAlreadyExists
	}

	s.users[user.ID] = user
	s.usersByEmail[emailKey] = user.ID
	return nil
}

func (s *Store) FindUserByEmail(email string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, exists := s.usersByEmail[strings.ToLower(email)]
	if !exists {
		return domain.User{}, auth.ErrNotFound
	}

	user, exists := s.users[userID]
	if !exists {
		return domain.User{}, auth.ErrNotFound
	}

	return user, nil
}

func (s *Store) FindUserByID(id string) (domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.users[id]
	if !exists {
		return domain.User{}, auth.ErrNotFound
	}

	return user, nil
}

func (s *Store) SaveSession(session domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.Token] = session
	return nil
}

func (s *Store) FindSessionByToken(token string) (domain.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[token]
	if !exists {
		return domain.Session{}, auth.ErrNotFound
	}

	return session, nil
}

func (s *Store) CreateChat(chatRecord domain.Chat) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.chats[chatRecord.ID] = chatRecord
	return nil
}

func (s *Store) FindChatByID(id string) (domain.Chat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chatRecord, exists := s.chats[id]
	if !exists {
		return domain.Chat{}, chat.ErrChatNotFound
	}

	return chatRecord, nil
}

func (s *Store) UpdateChat(chatRecord domain.Chat) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.chats[chatRecord.ID]; !exists {
		return chat.ErrChatNotFound
	}

	s.chats[chatRecord.ID] = chatRecord
	return nil
}

func (s *Store) CreateMessage(message domain.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages[message.ChatID] = append(s.messages[message.ChatID], message)
	return nil
}

func (s *Store) ListMessagesByChatID(chatID string) ([]domain.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stored := s.messages[chatID]
	cloned := make([]domain.Message, len(stored))
	copy(cloned, stored)
	return cloned, nil
}
