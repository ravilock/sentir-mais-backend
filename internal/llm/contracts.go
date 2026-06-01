package llm

import (
	"context"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type SupportClient interface {
	GenerateReply(ctx context.Context, history []domain.Message) (string, error)
}

type Extractor interface {
	ExtractEvent(ctx context.Context, history []domain.Message) (domain.ExtractedEvent, error)
}
