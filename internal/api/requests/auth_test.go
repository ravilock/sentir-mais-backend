package requests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterRequestValidate(t *testing.T) {
	t.Run("valid request should not return errors", func(t *testing.T) {
		request := &RegisterRequest{Email: "test@example.com", Password: "password123"}
		require.NoError(t, request.Validate())
	})

	t.Run("email is required", func(t *testing.T) {
		request := &RegisterRequest{Password: "password123"}
		err := request.Validate()
		require.ErrorContains(t, err, "required")
		require.ErrorContains(t, err, "Email")
	})

	t.Run("email should not be blank", func(t *testing.T) {
		request := &RegisterRequest{Email: " ", Password: "password123"}
		err := request.Validate()
		require.ErrorContains(t, err, "notblank")
		require.ErrorContains(t, err, "Email")
	})

	t.Run("email should be valid", func(t *testing.T) {
		request := &RegisterRequest{Email: "wrong@", Password: "password123"}
		err := request.Validate()
		require.ErrorContains(t, err, "Email")
	})

	t.Run("password min should be enforced", func(t *testing.T) {
		request := &RegisterRequest{Email: "test@example.com", Password: "short"}
		err := request.Validate()
		require.ErrorContains(t, err, "min")
		require.ErrorContains(t, err, "Password")
	})
}

func TestLoginRequestValidate(t *testing.T) {
	t.Run("valid request should not return errors", func(t *testing.T) {
		request := &LoginRequest{Email: "test@example.com", Password: "password123"}
		require.NoError(t, request.Validate())
	})

	t.Run("password is required", func(t *testing.T) {
		request := &LoginRequest{Email: "test@example.com"}
		err := request.Validate()
		require.ErrorContains(t, err, "required")
		require.ErrorContains(t, err, "Password")
	})
}
