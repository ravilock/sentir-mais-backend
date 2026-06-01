package requests

import "github.com/ravilock/sentir-mais-backend/internal/validations"

type CreateChatRequest struct {
	InitialMessage string `json:"initialMessage" validate:"required,notblank"`
}

type SendMessageRequest struct {
	Message string `json:"message" validate:"required,notblank"`
}

func (r *CreateChatRequest) Validate() error {
	return validations.Validate.Struct(r)
}

func (r *SendMessageRequest) Validate() error {
	return validations.Validate.Struct(r)
}
