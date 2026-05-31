package services

import (
	"context"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type userCreator interface {
	Create(ctx context.Context, user domain.User) error
}

type userByEmailFinder interface {
	FindByEmail(ctx context.Context, email string) (domain.User, error)
}

type userByIDFinder interface {
	FindByID(ctx context.Context, id string) (domain.User, error)
}

type sessionSaver interface {
	Save(ctx context.Context, session domain.Session) error
}

type sessionByTokenFinder interface {
	FindByToken(ctx context.Context, token string) (domain.Session, error)
}

type clock interface {
	Now() time.Time
}
