package services

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/id"
)

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now().UTC()
}

func createSession(ctx context.Context, saver sessionSaver, ttl time.Duration, user domain.User, serviceClock clock) (auth.Result, error) {
	token, err := id.New("tok")
	if err != nil {
		return auth.Result{}, err
	}

	now := serviceClock.Now()
	session := domain.Session{
		Token:     token,
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	if err := saver.Save(ctx, session); err != nil {
		return auth.Result{}, err
	}

	return auth.Result{
		AccessToken: token,
		User:        user,
	}, nil
}

func normalizeEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", auth.ErrInvalidEmail
	}

	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", auth.ErrInvalidEmail
	}

	return normalized, nil
}

func validatePassword(password string) error {
	if len(strings.TrimSpace(password)) < 8 {
		return auth.ErrWeakPassword
	}

	return nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(hash), nil
}

func comparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
