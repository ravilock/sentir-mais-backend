package auth

import (
	"net/http"
	"testing"
	"time"

	integrationtests "github.com/ravilock/sentir-mais-backend/integrationTests"
	apirequests "github.com/ravilock/sentir-mais-backend/internal/api/requests"
	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	"github.com/stretchr/testify/require"
)

func TestMe(t *testing.T) {
	t.Run("should return authenticated user", func(t *testing.T) {
		integrationtests.ClearDatabase(t)

		registerRequest := apirequests.RegisterRequest{
			Email:    "me@test.com",
			Password: "super-safe-password",
		}

		registerResponse := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/auth/register", registerRequest, "")
		require.Equal(t, http.StatusCreated, registerResponse.StatusCode)
		registerPayload := integrationtests.DecodeResponse[apiresponses.AuthResponse](t, registerResponse)

		meResponse := integrationtests.MustJSONRequest(t, http.MethodGet, "/api/v1/auth/me", nil, registerPayload.AccessToken)
		require.Equal(t, http.StatusOK, meResponse.StatusCode)

		mePayload := integrationtests.DecodeResponse[apiresponses.UserResponse](t, meResponse)
		require.Equal(t, registerPayload.User.ID, mePayload.ID)
		require.Equal(t, registerPayload.User.Email, mePayload.Email)
		require.WithinDuration(t, registerPayload.User.CreatedAt, mePayload.CreatedAt, time.Millisecond)
		require.WithinDuration(t, registerPayload.User.UpdatedAt, mePayload.UpdatedAt, time.Millisecond)
	})

	t.Run("should require bearer token", func(t *testing.T) {
		integrationtests.ClearDatabase(t)

		meResponse := integrationtests.MustJSONRequest(t, http.MethodGet, "/api/v1/auth/me", nil, "")
		require.Equal(t, http.StatusUnauthorized, meResponse.StatusCode)

		errorPayload := integrationtests.DecodeResponse[apiresponses.ErrorResponse](t, meResponse)
		require.Equal(t, "missing bearer token", errorPayload.Message)
	})
}
