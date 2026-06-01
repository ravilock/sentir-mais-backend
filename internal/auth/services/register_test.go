package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/auth"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRegisterService_Register(t *testing.T) {
	t.Run("should create user and session", func(t *testing.T) {
		userFinder := newMockUserByEmailFinder(t)
		userCreator := newMockUserCreator(t)
		sessionSaver := newMockSessionSaver(t)
		clock := newMockClock(t)

		now := time.Date(2026, time.May, 31, 12, 0, 0, 0, time.UTC)
		service := NewRegisterService(userFinder, userCreator, sessionSaver, 2*time.Hour)
		service.clock = clock

		userFinder.EXPECT().
			FindByEmail(mock.Anything, "user@test.com").
			Return(domain.User{}, auth.ErrNotFound).
			Once()
		clock.EXPECT().Now().Return(now).Twice()
		userCreator.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.User")).
			RunAndReturn(func(_ context.Context, user domain.User) error {
				require.NotEmpty(t, user.ID)
				require.Equal(t, "user@test.com", user.Email)
				require.NotEqual(t, "very-secure-password", user.PasswordHash)
				require.Equal(t, now, user.CreatedAt)
				require.Equal(t, now, user.UpdatedAt)
				return nil
			}).
			Once()
		sessionSaver.EXPECT().
			Save(mock.Anything, mock.AnythingOfType("domain.Session")).
			RunAndReturn(func(_ context.Context, session domain.Session) error {
				require.NotEmpty(t, session.Token)
				require.Equal(t, now, session.CreatedAt)
				require.Equal(t, now.Add(2*time.Hour), session.ExpiresAt)
				return nil
			}).
			Once()

		result, err := service.Register(context.Background(), " User@Test.com ", "very-secure-password")

		require.NoError(t, err)
		require.Equal(t, "user@test.com", result.User.Email)
		require.NotEmpty(t, result.User.ID)
		require.NotEmpty(t, result.AccessToken)
	})

	t.Run("should return conflict when email already exists", func(t *testing.T) {
		userFinder := newMockUserByEmailFinder(t)
		service := NewRegisterService(userFinder, newMockUserCreator(t), newMockSessionSaver(t), time.Hour)

		userFinder.EXPECT().
			FindByEmail(mock.Anything, "user@test.com").
			Return(domain.User{ID: "usr_123"}, nil).
			Once()

		result, err := service.Register(context.Background(), "user@test.com", "very-secure-password")

		require.ErrorIs(t, err, auth.ErrEmailAlreadyExists)
		require.Equal(t, auth.Result{}, result)
	})

	t.Run("should return validation error for weak password", func(t *testing.T) {
		service := NewRegisterService(newMockUserByEmailFinder(t), newMockUserCreator(t), newMockSessionSaver(t), time.Hour)

		result, err := service.Register(context.Background(), "user@test.com", "short")

		require.ErrorIs(t, err, auth.ErrWeakPassword)
		require.Equal(t, auth.Result{}, result)
	})

	t.Run("should return repository error when user lookup fails unexpectedly", func(t *testing.T) {
		userFinder := newMockUserByEmailFinder(t)
		service := NewRegisterService(userFinder, newMockUserCreator(t), newMockSessionSaver(t), time.Hour)
		expectedErr := errors.New("database down")

		userFinder.EXPECT().
			FindByEmail(mock.Anything, "user@test.com").
			Return(domain.User{}, expectedErr).
			Once()

		result, err := service.Register(context.Background(), "user@test.com", "very-secure-password")

		require.ErrorIs(t, err, expectedErr)
		require.Equal(t, auth.Result{}, result)
	})
}
