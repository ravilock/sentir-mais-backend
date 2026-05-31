package services

import (
	"context"
	"errors"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/auth"
)

type LoginService struct {
	userFinder userByEmailFinder
	sessions   sessionSaver
	sessionTTL time.Duration
	clock      clock
}

func NewLoginService(userFinder userByEmailFinder, sessions sessionSaver, sessionTTL time.Duration) *LoginService {
	return &LoginService{
		userFinder: userFinder,
		sessions:   sessions,
		sessionTTL: sessionTTL,
		clock:      realClock{},
	}
}

func (s *LoginService) Login(ctx context.Context, email, password string) (auth.Result, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return auth.Result{}, auth.ErrInvalidCredentials
	}

	user, err := s.userFinder.FindByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) {
			return auth.Result{}, auth.ErrInvalidCredentials
		}

		return auth.Result{}, err
	}

	if err := comparePassword(user.PasswordHash, password); err != nil {
		return auth.Result{}, auth.ErrInvalidCredentials
	}

	return createSession(ctx, s.sessions, s.sessionTTL, user, s.clock)
}
