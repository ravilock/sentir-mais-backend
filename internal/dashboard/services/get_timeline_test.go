package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGetTimelineServiceGetTimeline(t *testing.T) {
	t.Run("should use default last 30 days when range is omitted", func(t *testing.T) {
		now := time.Date(2026, time.June, 8, 13, 45, 0, 0, time.UTC)
		repository := &stubDailySummaryRangeLister{
			summaries: []domain.DailySummary{{
				UserID:           "usr_123",
				DayStart:         time.Date(2026, time.June, 7, 0, 0, 0, 0, time.UTC),
				DominantFeelings: []domain.FeelingScore{{Label: "sad", Confidence: 0.8}},
				MainEvents:       []string{"Bad meeting"},
				TimelinePoints:   []domain.TimelinePoint{{Date: "2026-06-07", PrimaryFeeling: "sad", SupportingEvent: "Bad meeting"}},
				GeneratedAt:      now,
			}},
		}
		service := NewGetTimelineService(repository)
		service.now = func() time.Time { return now }

		timeline, err := service.GetTimeline(context.Background(), "usr_123", nil, nil)

		require.NoError(t, err)
		require.Equal(t, time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC), repository.requestedFrom)
		require.Equal(t, time.Date(2026, time.June, 8, 0, 0, 0, 0, time.UTC), repository.requestedTo)
		require.Equal(t, domain.DashboardTimeline{
			From: repository.requestedFrom,
			To:   repository.requestedTo,
			Days: repository.summaries,
		}, timeline)
	})

	t.Run("should use explicit day range", func(t *testing.T) {
		repository := &stubDailySummaryRangeLister{}
		service := NewGetTimelineService(repository)

		from := time.Date(2026, time.June, 1, 18, 0, 0, 0, time.UTC)
		to := time.Date(2026, time.June, 7, 21, 0, 0, 0, time.UTC)

		_, err := service.GetTimeline(context.Background(), "usr_123", &from, &to)

		require.NoError(t, err)
		require.Equal(t, time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC), repository.requestedFrom)
		require.Equal(t, time.Date(2026, time.June, 7, 0, 0, 0, 0, time.UTC), repository.requestedTo)
	})

	t.Run("should return repository errors", func(t *testing.T) {
		repository := &stubDailySummaryRangeLister{err: errors.New("database unavailable")}
		service := NewGetTimelineService(repository)

		timeline, err := service.GetTimeline(context.Background(), "usr_123", nil, nil)

		require.ErrorContains(t, err, "database unavailable")
		require.Equal(t, domain.DashboardTimeline{}, timeline)
	})
}

type stubDailySummaryRangeLister struct {
	summaries     []domain.DailySummary
	err           error
	requestedUser string
	requestedFrom time.Time
	requestedTo   time.Time
}

func (s *stubDailySummaryRangeLister) ListByUserAndDayRange(_ context.Context, userID string, from, to time.Time) ([]domain.DailySummary, error) {
	s.requestedUser = userID
	s.requestedFrom = from
	s.requestedTo = to
	if s.err != nil {
		return nil, s.err
	}

	return s.summaries, nil
}
