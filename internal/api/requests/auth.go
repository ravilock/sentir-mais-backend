package requests

import "github.com/ravilock/sentir-mais-backend/internal/validations"

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,notblank,max=256,email"`
	Password string `json:"password" validate:"required,notblank,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,notblank,max=256,email"`
	Password string `json:"password" validate:"required,notblank,min=8,max=72"`
}

func (r *RegisterRequest) Validate() error {
	return validations.Validate.Struct(r)
}

func (r *LoginRequest) Validate() error {
	return validations.Validate.Struct(r)
}
