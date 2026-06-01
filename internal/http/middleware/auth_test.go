package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRequireAuth(t *testing.T) {
	t.Run("should inject user into context", func(t *testing.T) {
		authenticator := newMockAuthenticator(t)
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		authenticator.EXPECT().
			Authenticate(req.Context(), "tok_123").
			Return(domain.User{ID: "usr_123", Email: "user@test.com"}, nil).
			Once()

		handler := RequireAuth(authenticator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			require.True(t, ok)
			require.Equal(t, "usr_123", user.ID)
			w.WriteHeader(http.StatusNoContent)
		}))

		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("should reject missing bearer token", func(t *testing.T) {
		handler := RequireAuth(newMockAuthenticator(t))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		}))

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.JSONEq(t, `{"message":"missing bearer token"}`, rec.Body.String())
	})

	t.Run("should reject failed authentication", func(t *testing.T) {
		authenticator := newMockAuthenticator(t)
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		authenticator.EXPECT().
			Authenticate(req.Context(), "tok_123").
			Return(domain.User{}, errors.New("invalid token")).
			Once()

		handler := RequireAuth(authenticator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("next handler should not be called")
		}))

		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.JSONEq(t, `{"message":"unauthorized"}`, rec.Body.String())
	})
}
