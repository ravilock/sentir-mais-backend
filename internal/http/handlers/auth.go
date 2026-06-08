package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	apirequests "github.com/ravilock/sentir-mais-backend/internal/api/requests"
	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/http/middleware"
)

type AuthHandler struct {
	logger     *slog.Logger
	registerer registerer
	loginer    loginer
}

func NewAuthHandler(logger *slog.Logger, registerer registerer, loginer loginer) *AuthHandler {
	return &AuthHandler{
		logger:     logger,
		registerer: registerer,
		loginer:    loginer,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var request apirequests.RegisterRequest
	if err := decodeJSON(r, &request); err != nil {
		logRequestError(h.logger, r, http.StatusBadRequest, "failed to decode register request", err)
		respondDecodeError(w, err)
		return
	}
	if err := request.Validate(); err != nil {
		logRequestError(h.logger, r, http.StatusUnprocessableEntity, "failed to validate register request", err)
		respondDecodeError(w, err)
		return
	}

	result, err := h.registerer.Register(r.Context(), request.Email, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidEmail), errors.Is(err, auth.ErrWeakPassword):
			logRequestError(h.logger, r, http.StatusBadRequest, "register request failed", err)
			respondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, auth.ErrEmailAlreadyExists):
			logRequestError(h.logger, r, http.StatusConflict, "register request failed", err)
			respondError(w, http.StatusConflict, err.Error())
		default:
			logRequestError(h.logger, r, http.StatusInternalServerError, "register request failed", err)
			respondError(w, http.StatusInternalServerError, "failed to register user")
		}
		return
	}

	respondJSON(w, http.StatusCreated, apiresponses.AuthResponse{
		AccessToken: result.AccessToken,
		User:        toUserResponse(result.User),
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var request apirequests.LoginRequest
	if err := decodeJSON(r, &request); err != nil {
		logRequestError(h.logger, r, http.StatusBadRequest, "failed to decode login request", err)
		respondDecodeError(w, err)
		return
	}
	if err := request.Validate(); err != nil {
		logRequestError(h.logger, r, http.StatusUnprocessableEntity, "failed to validate login request", err)
		respondDecodeError(w, err)
		return
	}

	result, err := h.loginer.Login(r.Context(), request.Email, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			logRequestError(h.logger, r, http.StatusUnauthorized, "login request failed", err)
			respondError(w, http.StatusUnauthorized, err.Error())
		default:
			logRequestError(h.logger, r, http.StatusInternalServerError, "login request failed", err)
			respondError(w, http.StatusInternalServerError, "failed to login")
		}
		return
	}

	respondJSON(w, http.StatusOK, apiresponses.AuthResponse{
		AccessToken: result.AccessToken,
		User:        toUserResponse(result.User),
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		logRequestError(h.logger, r, http.StatusUnauthorized, "missing authenticated user in request context", nil)
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	respondJSON(w, http.StatusOK, toUserResponse(user))
}

func toUserResponse(user domain.User) apiresponses.UserResponse {
	return apiresponses.UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
