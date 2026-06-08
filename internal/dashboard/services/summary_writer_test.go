package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestSummaryWriterUpdateForAnalysis(t *testing.T) {
	t.Run("should upsert daily and weekly summaries from matching analyses", func(t *testing.T) {
		now := time.Date(2026, time.June, 3, 15, 0, 0, 0, time.UTC)
		generatedAt := time.Date(2026, time.June, 8, 3, 51, 37, 625000000, time.UTC)
		lister := &stubAnalysisRangeLister{
			results: map[string][]domain.MessageAnalysis{
				rangeKey(startOfDay(now), startOfDay(now).AddDate(0, 0, 1)): {
					{
						UserID:         "usr_123",
						PrimaryFeeling: domain.FeelingScore{Label: "anxious", Confidence: 0.9},
						ExtractedEvent: &domain.ExtractedEvent{EventSummary: "Argument with manager"},
						CreatedAt:      now.Add(-2 * time.Hour),
					},
					{
						UserID:         "usr_123",
						PrimaryFeeling: domain.FeelingScore{Label: "anxious", Confidence: 0.7},
						ExtractedEvent: &domain.ExtractedEvent{EventSummary: "Argument with manager"},
						CreatedAt:      now,
					},
				},
				rangeKey(startOfWeek(now), startOfWeek(now).AddDate(0, 0, 7)): {
					{
						UserID:         "usr_123",
						PrimaryFeeling: domain.FeelingScore{Label: "sad", Confidence: 0.6},
						ExtractedEvent: &domain.ExtractedEvent{EventSummary: "Bad meeting"},
						CreatedAt:      startOfWeek(now).Add(8 * time.Hour),
					},
					{
						UserID:         "usr_123",
						PrimaryFeeling: domain.FeelingScore{Label: "anxious", Confidence: 0.9},
						ExtractedEvent: &domain.ExtractedEvent{EventSummary: "Argument with manager"},
						CreatedAt:      now,
					},
				},
			},
		}
		daily := &stubDailySummaryUpserter{}
		weekly := &stubWeeklySummaryUpserter{}
		writer := NewSummaryWriter(lister, daily, weekly)
		writer.now = func() time.Time { return generatedAt }

		err := writer.UpdateForAnalysis(context.Background(), domain.MessageAnalysis{
			UserID:    "usr_123",
			CreatedAt: now,
		})

		require.NoError(t, err)
		require.Len(t, daily.summaries, 1)
		require.Equal(t, startOfDay(now), daily.summaries[0].DayStart)
		require.Equal(t, []domain.FeelingScore{{Label: "anxious", Confidence: 0.8}}, daily.summaries[0].DominantFeelings)
		require.Equal(t, []string{"Argument with manager"}, daily.summaries[0].MainEvents)
		require.Len(t, daily.summaries[0].TimelinePoints, 2)
		require.Equal(t, generatedAt, daily.summaries[0].GeneratedAt)

		require.Len(t, weekly.summaries, 1)
		require.Equal(t, startOfWeek(now), weekly.summaries[0].WeekStart)
		require.Equal(t, []domain.FeelingScore{
			{Label: "anxious", Confidence: 0.9},
			{Label: "sad", Confidence: 0.6},
		}, weekly.summaries[0].DominantFeelings)
		require.Equal(t, []string{"Bad meeting", "Argument with manager"}, weekly.summaries[0].MainEvents)
		require.Len(t, weekly.summaries[0].TimelinePoints, 2)
		require.Equal(t, generatedAt, weekly.summaries[0].GeneratedAt)
	})

	t.Run("should return day range errors before writing summaries", func(t *testing.T) {
		now := time.Date(2026, time.June, 3, 15, 0, 0, 0, time.UTC)
		lister := &stubAnalysisRangeLister{err: errors.New("read failed")}
		daily := &stubDailySummaryUpserter{}
		weekly := &stubWeeklySummaryUpserter{}
		writer := NewSummaryWriter(lister, daily, weekly)

		err := writer.UpdateForAnalysis(context.Background(), domain.MessageAnalysis{
			UserID:    "usr_123",
			CreatedAt: now,
		})

		require.ErrorContains(t, err, "read failed")
		require.Empty(t, daily.summaries)
		require.Empty(t, weekly.summaries)
	})
}

func TestSummarizeTimelinePointsSkipsMissingClassifierLabels(t *testing.T) {
	points := summarizeTimelinePoints([]domain.MessageAnalysis{
		{
			PrimaryFeeling: domain.FeelingScore{Label: ""},
			CreatedAt:      time.Date(2026, time.June, 3, 10, 0, 0, 0, time.UTC),
		},
		{
			PrimaryFeeling: domain.FeelingScore{Label: "sad"},
			ExtractedEvent: &domain.ExtractedEvent{EventSummary: "Bad meeting"},
			CreatedAt:      time.Date(2026, time.June, 3, 11, 0, 0, 0, time.UTC),
		},
	})

	require.Equal(t, []domain.TimelinePoint{{
		Date:            "2026-06-03",
		PrimaryFeeling:  "sad",
		SupportingEvent: "Bad meeting",
	}}, points)
}

type stubAnalysisRangeLister struct {
	results map[string][]domain.MessageAnalysis
	err     error
}

func (s *stubAnalysisRangeLister) ListByUserAndCreatedAtRange(_ context.Context, _ string, start, end time.Time) ([]domain.MessageAnalysis, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.results[rangeKey(start, end)], nil
}

type stubDailySummaryUpserter struct {
	summaries []domain.DailySummary
}

func (s *stubDailySummaryUpserter) Upsert(_ context.Context, summary domain.DailySummary) error {
	s.summaries = append(s.summaries, summary)
	return nil
}

type stubWeeklySummaryUpserter struct {
	summaries []domain.WeeklySummary
}

func (s *stubWeeklySummaryUpserter) Upsert(_ context.Context, summary domain.WeeklySummary) error {
	s.summaries = append(s.summaries, summary)
	return nil
}

func rangeKey(start, end time.Time) string {
	return start.UTC().Format(time.RFC3339) + "|" + end.UTC().Format(time.RFC3339)
}
