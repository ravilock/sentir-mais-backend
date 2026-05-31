package middleware

import (
	"context"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type authenticator interface {
	Authenticate(ctx context.Context, token string) (domain.User, error)
}
