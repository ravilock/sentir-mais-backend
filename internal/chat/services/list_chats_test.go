package services

import (
	"context"
	"testing"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestListChatsServiceListChats(t *testing.T) {
	now := time.Date(2026, time.June, 7, 21, 15, 0, 0, time.UTC)
	service := NewListChatsService(
		stubChatLister{
			chats: []domain.Chat{
				{
					ID:        "cht_123",
					UserID:    "usr_123",
					CreatedAt: now.Add(-24 * time.Hour),
					UpdatedAt: now,
				},
			},
		},
		stubLatestMessageFinder{
			messages: map[string]domain.Message{
				"cht_123": {
					ID:        "msg_456",
					ChatID:    "cht_123",
					UserID:    "usr_123",
					Content:   " Hoje eu chorei depois da conversa com meu chefe. ",
					CreatedAt: now,
				},
			},
		},
	)

	chats, err := service.ListChats(context.Background(), "usr_123")

	require.NoError(t, err)
	require.Equal(t, []domain.ChatSummary{{
		ID:                 "cht_123",
		CreatedAt:          now.Add(-24 * time.Hour),
		UpdatedAt:          now,
		LastMessagePreview: "Hoje eu chorei depois da conversa com meu chefe.",
		LastMessageAt:      now,
	}}, chats)
}

type stubChatLister struct {
	chats []domain.Chat
}

func (s stubChatLister) ListByUserID(_ context.Context, _ string) ([]domain.Chat, error) {
	return s.chats, nil
}

type stubLatestMessageFinder struct {
	messages map[string]domain.Message
}

func (s stubLatestMessageFinder) FindLatestByChatID(_ context.Context, chatID string) (domain.Message, error) {
	return s.messages[chatID], nil
}
