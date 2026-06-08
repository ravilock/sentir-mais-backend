package handlers

import (
	"context"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type registerer interface {
	Register(ctx context.Context, email, password string) (auth.Result, error)
}

type loginer interface {
	Login(ctx context.Context, email, password string) (auth.Result, error)
}

type chatCreator interface {
	CreateChat(ctx context.Context, userID, initialMessage string) (domain.Chat, domain.Message, error)
}

type messageSender interface {
	SendMessage(ctx context.Context, chatID, userID, content string) (domain.Message, error)
}

type chatsLister interface {
	ListChats(ctx context.Context, userID string) ([]domain.ChatSummary, error)
}

type messagesLister interface {
	ListMessages(ctx context.Context, chatID, userID string) ([]domain.Message, error)
}

type weeklySummaryGetter interface {
	GetWeek(ctx context.Context, userID string) (domain.WeeklySummary, error)
}

type timelineSummaryGetter interface {
	GetTimeline(ctx context.Context, userID string, from, to *time.Time) (domain.DashboardTimeline, error)
}
