package services

import (
	"context"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type ListMessagesService struct {
	chats    chatFinder
	messages messageLister
}

func NewListMessagesService(chats chatFinder, messages messageLister) *ListMessagesService {
	return &ListMessagesService{
		chats:    chats,
		messages: messages,
	}
}

func (s *ListMessagesService) ListMessages(ctx context.Context, chatID, userID string) ([]domain.Message, error) {
	if _, err := authorizeChat(ctx, s.chats, chatID, userID); err != nil {
		return nil, err
	}

	return s.messages.ListByChatID(ctx, chatID)
}
