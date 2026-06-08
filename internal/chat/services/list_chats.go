package services

import (
	"context"
	"strings"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type ListChatsService struct {
	chats    chatLister
	messages latestMessageFinder
}

func NewListChatsService(chats chatLister, messages latestMessageFinder) *ListChatsService {
	return &ListChatsService{
		chats:    chats,
		messages: messages,
	}
}

func (s *ListChatsService) ListChats(ctx context.Context, userID string) ([]domain.ChatSummary, error) {
	chats, err := s.chats.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	summaries := make([]domain.ChatSummary, 0, len(chats))
	for _, chatRecord := range chats {
		latestMessage, err := s.messages.FindLatestByChatID(ctx, chatRecord.ID)
		if err != nil {
			return nil, err
		}

		lastMessageAt := latestMessage.CreatedAt
		if lastMessageAt.IsZero() {
			lastMessageAt = chatRecord.UpdatedAt
		}

		summaries = append(summaries, domain.ChatSummary{
			ID:                 chatRecord.ID,
			CreatedAt:          chatRecord.CreatedAt,
			UpdatedAt:          chatRecord.UpdatedAt,
			LastMessagePreview: strings.TrimSpace(latestMessage.Content),
			LastMessageAt:      lastMessageAt,
		})
	}

	return summaries, nil
}
