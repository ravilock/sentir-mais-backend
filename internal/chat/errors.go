package chat

import "errors"

var (
	ErrChatNotFound = errors.New("chat not found")
	ErrEmptyMessage = errors.New("message is empty")
)
