package repositories

import (
	"context"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/storage/memory"
)

type MessageRepository struct {
	store *memory.Store
}

func NewMessageRepository(store *memory.Store) *MessageRepository {
	return &MessageRepository{store: store}
}

func (r *MessageRepository) Create(_ context.Context, message domain.Message) error {
	return r.store.CreateMessage(message)
}

func (r *MessageRepository) ListByChatID(_ context.Context, chatID string) ([]domain.Message, error) {
	return r.store.ListMessagesByChatID(chatID)
}
