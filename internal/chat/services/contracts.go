package services

import (
	"context"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type chatCreator interface {
	Create(ctx context.Context, chat domain.Chat) error
}

type chatFinder interface {
	FindByID(ctx context.Context, id string) (domain.Chat, error)
}

type chatUpdater interface {
	Update(ctx context.Context, chat domain.Chat) error
}

type messageCreator interface {
	Create(ctx context.Context, message domain.Message) error
}

type messageLister interface {
	ListByChatID(ctx context.Context, chatID string) ([]domain.Message, error)
}

type llmResponder interface {
	GenerateReply(ctx context.Context, history []domain.Message) (string, error)
}

type feelingClassifier interface {
	Classify(ctx context.Context, text string) (domain.ClassificationResult, error)
}

type messageAnalysisCreator interface {
	Create(ctx context.Context, analysis domain.MessageAnalysis) error
}

type clock interface {
	Now() time.Time
}
