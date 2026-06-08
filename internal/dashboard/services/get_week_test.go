package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestGetWeekServiceGetWeek(t *testing.T) {
	now := time.Date(2026, time.June, 8, 13, 45, 0, 0, time.UTC)
	weekStart := time.Date(2026, time.June, 8, 0, 0, 0, 0, time.UTC)

	t.Run("should return persisted summary for current week", func(t *testing.T) {
		repository := &stubWeeklySummaryFinder{
			summary: domain.WeeklySummary{
				UserID:           "usr_123",
				WeekStart:        weekStart,
				DominantFeelings: []domain.FeelingScore{{Label: "sad", Confidence: 0.8}},
				MainEvents:       []string{"Bad meeting"},
				TimelinePoints:   []domain.TimelinePoint{{Date: "2026-06-08", PrimaryFeeling: "sad", SupportingEvent: "Bad meeting"}},
				GeneratedAt:      now,
			},
		}
		service := NewGetWeekService(repository)
		service.now = func() time.Time { return now }

		summary, err := service.GetWeek(context.Background(), "usr_123")

		require.NoError(t, err)
		require.Equal(t, weekStart, repository.requestedWeekStart)
		require.Equal(t, repository.summary, summary)
	})

	t.Run("should return empty summary when current week has no persisted data", func(t *testing.T) {
		repository := &stubWeeklySummaryFinder{err: mongo.ErrNoDocuments}
		service := NewGetWeekService(repository)
		service.now = func() time.Time { return now }

		summary, err := service.GetWeek(context.Background(), "usr_123")

		require.NoError(t, err)
		require.Equal(t, weekStart, repository.requestedWeekStart)
		require.Equal(t, domain.WeeklySummary{
			UserID:           "usr_123",
			WeekStart:        weekStart,
			DominantFeelings: []domain.FeelingScore{},
			MainEvents:       []string{},
			TimelinePoints:   []domain.TimelinePoint{},
			GeneratedAt:      now,
		}, summary)
	})

	t.Run("should return repository errors", func(t *testing.T) {
		repository := &stubWeeklySummaryFinder{err: errors.New("db unavailable")}
		service := NewGetWeekService(repository)
		service.now = func() time.Time { return now }

		summary, err := service.GetWeek(context.Background(), "usr_123")

		require.ErrorContains(t, err, "db unavailable")
		require.Equal(t, domain.WeeklySummary{}, summary)
	})
}

type stubWeeklySummaryFinder struct {
	summary            domain.WeeklySummary
	err                error
	requestedUserID    string
	requestedWeekStart time.Time
}

func (s *stubWeeklySummaryFinder) FindByUserAndWeek(_ context.Context, userID string, weekStart time.Time) (domain.WeeklySummary, error) {
	s.requestedUserID = userID
	s.requestedWeekStart = weekStart
	if s.err != nil {
		return domain.WeeklySummary{}, s.err
	}

	return s.summary, nil
}
