package services

import (
	"strings"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/chat"
)

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now().UTC()
}

func validateMessage(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", chat.ErrEmptyMessage
	}

	return trimmed, nil
}
