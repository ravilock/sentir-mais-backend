package auth

import (
	"net/http"
	"testing"

	integrationtests "github.com/ravilock/sentir-mais-backend/integrationTests"
	apirequests "github.com/ravilock/sentir-mais-backend/internal/api/requests"
	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	authdomain "github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/stretchr/testify/require"
)

func TestLogin(t *testing.T) {
	t.Run("should login an existing user", func(t *testing.T) {
		integrationtests.ClearDatabase(t)

		registerRequest := apirequests.RegisterRequest{
			Email:    "login@test.com",
			Password: "super-safe-password",
		}

		registerResponse := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/auth/register", registerRequest, "")
		require.Equal(t, http.StatusCreated, registerResponse.StatusCode)
		registerPayload := integrationtests.DecodeResponse[apiresponses.AuthResponse](t, registerResponse)

		loginRequest := apirequests.LoginRequest{
			Email:    registerRequest.Email,
			Password: registerRequest.Password,
		}

		loginResponse := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/auth/login", loginRequest, "")
		require.Equal(t, http.StatusOK, loginResponse.StatusCode)

		loginPayload := integrationtests.DecodeResponse[apiresponses.AuthResponse](t, loginResponse)
		require.NotEmpty(t, loginPayload.AccessToken)
		require.Equal(t, registerPayload.User.ID, loginPayload.User.ID)
		require.Equal(t, registerPayload.User.Email, loginPayload.User.Email)
		require.NotEqual(t, registerPayload.AccessToken, loginPayload.AccessToken)

		sessionDocument := integrationtests.MustFindSessionByToken(t, loginPayload.AccessToken)
		require.Equal(t, loginPayload.User.ID, sessionDocument["user_id"])
	})

	t.Run("should reject wrong password", func(t *testing.T) {
		integrationtests.ClearDatabase(t)

		registerRequest := apirequests.RegisterRequest{
			Email:    "wrong-password@test.com",
			Password: "super-safe-password",
		}

		registerResponse := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/auth/register", registerRequest, "")
		require.Equal(t, http.StatusCreated, registerResponse.StatusCode)
		_ = integrationtests.DecodeResponse[apiresponses.AuthResponse](t, registerResponse)

		loginRequest := apirequests.LoginRequest{
			Email:    registerRequest.Email,
			Password: "wrong-password",
		}

		loginResponse := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/auth/login", loginRequest, "")
		require.Equal(t, http.StatusUnauthorized, loginResponse.StatusCode)

		errorPayload := integrationtests.DecodeResponse[apiresponses.ErrorResponse](t, loginResponse)
		require.Equal(t, authdomain.ErrInvalidCredentials.Error(), errorPayload.Message)
	})
}
