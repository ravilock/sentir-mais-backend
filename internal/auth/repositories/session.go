package repositories

import (
	"context"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/storage/memory"
)

type SessionRepository struct {
	store *memory.Store
}

func NewSessionRepository(store *memory.Store) *SessionRepository {
	return &SessionRepository{store: store}
}

func (r *SessionRepository) Save(_ context.Context, session domain.Session) error {
	return r.store.SaveSession(session)
}

func (r *SessionRepository) FindByToken(_ context.Context, token string) (domain.Session, error) {
	return r.store.FindSessionByToken(token)
}
