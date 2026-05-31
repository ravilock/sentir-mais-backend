package repositories

import (
	"context"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/storage/memory"
)

type ChatRepository struct {
	store *memory.Store
}

func NewChatRepository(store *memory.Store) *ChatRepository {
	return &ChatRepository{store: store}
}

func (r *ChatRepository) Create(_ context.Context, chat domain.Chat) error {
	return r.store.CreateChat(chat)
}

func (r *ChatRepository) FindByID(_ context.Context, id string) (domain.Chat, error) {
	return r.store.FindChatByID(id)
}

func (r *ChatRepository) Update(_ context.Context, chat domain.Chat) error {
	return r.store.UpdateChat(chat)
}
