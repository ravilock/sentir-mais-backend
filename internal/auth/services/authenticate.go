package services

import (
	"context"
	"errors"
	"strings"

	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type AuthenticateService struct {
	sessions sessionByTokenFinder
	users    userByIDFinder
	clock    clock
}

func NewAuthenticateService(sessions sessionByTokenFinder, users userByIDFinder) *AuthenticateService {
	return &AuthenticateService{
		sessions: sessions,
		users:    users,
		clock:    realClock{},
	}
}

func (s *AuthenticateService) Authenticate(ctx context.Context, token string) (domain.User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return domain.User{}, auth.ErrUnauthorized
	}

	session, err := s.sessions.FindByToken(ctx, token)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return domain.User{}, auth.ErrUnauthorized
		}

		return domain.User{}, err
	}

	if s.clock.Now().After(session.ExpiresAt) {
		return domain.User{}, auth.ErrUnauthorized
	}

	user, err := s.users.FindByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return domain.User{}, auth.ErrUnauthorized
		}

		return domain.User{}, err
	}

	return user, nil
}
