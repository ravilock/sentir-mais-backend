package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	httpmiddleware "github.com/ravilock/sentir-mais-backend/internal/http/middleware"
	"github.com/ravilock/sentir-mais-backend/internal/validations"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type handlerAuthStub struct {
	user domain.User
	err  error
}

func (s handlerAuthStub) Authenticate(_ context.Context, _ string) (domain.User, error) {
	return s.user, s.err
}

func TestAuthHandler_Register(t *testing.T) {
	require.NoError(t, validations.InitValidator())
	registerer := newMockRegisterer(t)
	handler := NewAuthHandler(newTestHTTPLogger(), registerer, newMockLoginer(t))

	t.Run("should register user", func(t *testing.T) {
		requestBody := []byte(`{"email":"user@test.com","password":"very-secure-password"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(requestBody))
		rec := httptest.NewRecorder()

		expected := auth.Result{
			AccessToken: "tok_123",
			User: domain.User{
				ID:        "usr_123",
				Email:     "user@test.com",
				CreatedAt: time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
			},
		}
		registerer.EXPECT().
			Register(mock.Anything, "user@test.com", "very-secure-password").
			Return(expected, nil).
			Once()

		handler.Register(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		var payload apiresponses.AuthResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		require.Equal(t, "tok_123", payload.AccessToken)
	})

	t.Run("should return conflict for duplicated email", func(t *testing.T) {
		requestBody := []byte(`{"email":"user@test.com","password":"very-secure-password"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(requestBody))
		rec := httptest.NewRecorder()

		registerer.EXPECT().
			Register(mock.Anything, "user@test.com", "very-secure-password").
			Return(auth.Result{}, auth.ErrEmailAlreadyExists).
			Once()

		handler.Register(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
		require.JSONEq(t, `{"message":"email already exists"}`, rec.Body.String())
	})

	t.Run("should validate register payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader([]byte(`{"email":"wrong","password":"short"}`)))
		rec := httptest.NewRecorder()

		handler.Register(rec, req)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		require.JSONEq(t, `{"message":"field 'Email' must be a valid email, field 'Password' minimum length is 8"}`, rec.Body.String())
	})
}

func TestAuthHandler_Login(t *testing.T) {
	require.NoError(t, validations.InitValidator())
	loginer := newMockLoginer(t)
	handler := NewAuthHandler(newTestHTTPLogger(), newMockRegisterer(t), loginer)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte(`{"email":"user@test.com","password":"very-secure-password"}`)))
	rec := httptest.NewRecorder()

	loginer.EXPECT().
		Login(mock.Anything, "user@test.com", "very-secure-password").
		Return(auth.Result{
			AccessToken: "tok_456",
			User:        domain.User{ID: "usr_123", Email: "user@test.com"},
		}, nil).
		Once()

	handler.Login(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"accessToken":"tok_456","user":{"id":"usr_123","email":"user@test.com","createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z"}}`, rec.Body.String())
}

func TestAuthHandler_LoginValidation(t *testing.T) {
	require.NoError(t, validations.InitValidator())
	handler := NewAuthHandler(newTestHTTPLogger(), newMockRegisterer(t), newMockLoginer(t))
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte(`{"email":"   ","password":"123"}`)))
	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.JSONEq(t, `{"message":"field 'Email' is required, field 'Password' minimum length is 8"}`, rec.Body.String())
}

func TestAuthHandler_Me(t *testing.T) {
	handler := NewAuthHandler(newTestHTTPLogger(), newMockRegisterer(t), newMockLoginer(t))

	t.Run("should return authenticated user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		protected := httpmiddleware.RequireAuth(handlerAuthStub{
			user: domain.User{
				ID:        "usr_123",
				Email:     "user@test.com",
				CreatedAt: time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC),
			},
		})(http.HandlerFunc(handler.Me))

		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"id":"usr_123","email":"user@test.com","createdAt":"2026-05-31T12:00:00Z","updatedAt":"2026-05-31T12:00:00Z"}`, rec.Body.String())
	})

	t.Run("should return unauthorized when auth fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		protected := httpmiddleware.RequireAuth(handlerAuthStub{err: errors.New("nope")})(http.HandlerFunc(handler.Me))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.JSONEq(t, `{"message":"unauthorized"}`, rec.Body.String())
	})
}
