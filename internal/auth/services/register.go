package services

import (
	"context"
	"errors"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/id"
)

type RegisterService struct {
	userFinder  userByEmailFinder
	userCreator userCreator
	sessions    sessionSaver
	sessionTTL  time.Duration
	clock       clock
}

func NewRegisterService(userFinder userByEmailFinder, userCreator userCreator, sessions sessionSaver, sessionTTL time.Duration) *RegisterService {
	return &RegisterService{
		userFinder:  userFinder,
		userCreator: userCreator,
		sessions:    sessions,
		sessionTTL:  sessionTTL,
		clock:       realClock{},
	}
}

func (s *RegisterService) Register(ctx context.Context, email, password string) (auth.Result, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return auth.Result{}, err
	}

	if err := validatePassword(password); err != nil {
		return auth.Result{}, err
	}

	if _, err := s.userFinder.FindByEmail(ctx, normalizedEmail); err == nil {
		return auth.Result{}, auth.ErrEmailAlreadyExists
	} else if !errors.Is(err, auth.ErrNotFound) {
		return auth.Result{}, err
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return auth.Result{}, err
	}

	userID, err := id.New("usr")
	if err != nil {
		return auth.Result{}, err
	}

	now := s.clock.Now()
	user := domain.User{
		ID:           userID,
		Email:        normalizedEmail,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userCreator.Create(ctx, user); err != nil {
		if errors.Is(err, auth.ErrEmailAlreadyExists) {
			return auth.Result{}, auth.ErrEmailAlreadyExists
		}

		return auth.Result{}, err
	}

	return createSession(ctx, s.sessions, s.sessionTTL, user, s.clock)
}
