package handlers

import (
	"errors"
	"net/http"

	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/http/dto"
	"github.com/ravilock/sentir-mais-backend/internal/http/middleware"
)

type AuthHandler struct {
	registerer registerer
	loginer    loginer
}

func NewAuthHandler(registerer registerer, loginer loginer) *AuthHandler {
	return &AuthHandler{
		registerer: registerer,
		loginer:    loginer,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var request dto.RegisterRequest
	if err := decodeJSON(r, &request); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.registerer.Register(r.Context(), request.Email, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidEmail), errors.Is(err, auth.ErrWeakPassword):
			respondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, auth.ErrEmailAlreadyExists):
			respondError(w, http.StatusConflict, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "failed to register user")
		}
		return
	}

	respondJSON(w, http.StatusCreated, dto.AuthResponse{
		AccessToken: result.AccessToken,
		User:        toUserResponse(result.User),
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var request dto.LoginRequest
	if err := decodeJSON(r, &request); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.loginer.Login(r.Context(), request.Email, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			respondError(w, http.StatusUnauthorized, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "failed to login")
		}
		return
	}

	respondJSON(w, http.StatusOK, dto.AuthResponse{
		AccessToken: result.AccessToken,
		User:        toUserResponse(result.User),
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	respondJSON(w, http.StatusOK, toUserResponse(user))
}

func toUserResponse(user domain.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
