package repositories

import (
	"context"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/storage/memory"
)

type UserRepository struct {
	store *memory.Store
}

func NewUserRepository(store *memory.Store) *UserRepository {
	return &UserRepository{store: store}
}

func (r *UserRepository) Create(_ context.Context, user domain.User) error {
	return r.store.CreateUser(user)
}

func (r *UserRepository) FindByEmail(_ context.Context, email string) (domain.User, error) {
	return r.store.FindUserByEmail(email)
}

func (r *UserRepository) FindByID(_ context.Context, id string) (domain.User, error) {
	return r.store.FindUserByID(id)
}
