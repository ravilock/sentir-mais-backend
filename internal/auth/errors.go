package auth

import (
	"errors"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

var (
	ErrInvalidEmail       = errors.New("invalid email")
	ErrWeakPassword       = errors.New("password must have at least 8 characters")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrNotFound           = errors.New("not found")
)

type Result struct {
	AccessToken string
	User        domain.User
}
