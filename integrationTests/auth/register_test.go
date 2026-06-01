package auth

import (
	"net/http"
	"strings"
	"testing"
	"time"

	integrationtests "github.com/ravilock/sentir-mais-backend/integrationTests"
	apirequests "github.com/ravilock/sentir-mais-backend/internal/api/requests"
	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	authdomain "github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	t.Run("should create a user and session", func(t *testing.T) {
		integrationtests.ClearDatabase(t)

		request := apirequests.RegisterRequest{
			Email:    "Register.User@Test.com",
			Password: "super-safe-password",
		}

		response := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/auth/register", request, "")
		require.Equal(t, http.StatusCreated, response.StatusCode)

		payload := integrationtests.DecodeResponse[apiresponses.AuthResponse](t, response)
		require.NotEmpty(t, payload.AccessToken)
		require.Equal(t, strings.ToLower(request.Email), payload.User.Email)
		require.NotEmpty(t, payload.User.ID)
		require.WithinDuration(t, time.Now().UTC(), payload.User.CreatedAt, 10*time.Second)
		require.WithinDuration(t, payload.User.CreatedAt, payload.User.UpdatedAt, time.Second)

		userDocument := integrationtests.MustFindUserDocument(t, request.Email)
		require.Equal(t, payload.User.ID, userDocument["_id"])
		require.Equal(t, strings.ToLower(request.Email), userDocument["email"])
		require.NotEmpty(t, userDocument["password_hash"])
		require.NotEqual(t, request.Password, userDocument["password_hash"])

		sessionDocument := integrationtests.MustFindSessionByToken(t, payload.AccessToken)
		require.Equal(t, payload.AccessToken, sessionDocument["_id"])
		require.Equal(t, payload.User.ID, sessionDocument["user_id"])
	})

	t.Run("should reject duplicated email", func(t *testing.T) {
		integrationtests.ClearDatabase(t)

		request := apirequests.RegisterRequest{
			Email:    "duplicate@test.com",
			Password: "super-safe-password",
		}

		firstResponse := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/auth/register", request, "")
		require.Equal(t, http.StatusCreated, firstResponse.StatusCode)
		_ = integrationtests.DecodeResponse[apiresponses.AuthResponse](t, firstResponse)

		secondResponse := integrationtests.MustJSONRequest(t, http.MethodPost, "/api/v1/auth/register", request, "")
		require.Equal(t, http.StatusConflict, secondResponse.StatusCode)

		errorPayload := integrationtests.DecodeResponse[apiresponses.ErrorResponse](t, secondResponse)
		require.Equal(t, authdomain.ErrEmailAlreadyExists.Error(), errorPayload.Message)
	})
}
