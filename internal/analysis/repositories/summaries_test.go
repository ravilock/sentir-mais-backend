package repositories

import (
	"testing"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestDailySummaryDocumentToDomain(t *testing.T) {
	dayStart := time.Date(2026, time.June, 3, 0, 0, 0, 0, time.UTC)
	generatedAt := time.Date(2026, time.June, 3, 18, 0, 0, 0, time.UTC)

	document := dailySummaryDocument{
		UserID:           "usr_123",
		DayStart:         dayStart,
		DominantFeelings: []feelingScoreDocument{{Label: "anxious", Confidence: 0.82}},
		MainEvents:       []string{"Argument with manager"},
		TimelinePoints: []timelinePointDocument{{
			Date:            "2026-06-03",
			PrimaryFeeling:  "anxious",
			SupportingEvent: "Argument with manager",
		}},
		GeneratedAt: generatedAt,
	}

	require.Equal(t, domain.DailySummary{
		UserID:           "usr_123",
		DayStart:         dayStart,
		DominantFeelings: []domain.FeelingScore{{Label: "anxious", Confidence: 0.82}},
		MainEvents:       []string{"Argument with manager"},
		TimelinePoints: []domain.TimelinePoint{{
			Date:            "2026-06-03",
			PrimaryFeeling:  "anxious",
			SupportingEvent: "Argument with manager",
		}},
		GeneratedAt: generatedAt,
	}, document.toDomain())
}

func TestWeeklySummaryDocumentToDomain(t *testing.T) {
	weekStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	generatedAt := time.Date(2026, time.June, 7, 18, 0, 0, 0, time.UTC)

	document := weeklySummaryDocument{
		UserID:           "usr_123",
		WeekStart:        weekStart,
		DominantFeelings: []feelingScoreDocument{{Label: "sad", Confidence: 0.73}},
		MainEvents:       []string{"Bad meeting"},
		TimelinePoints: []timelinePointDocument{{
			Date:            "2026-06-03",
			PrimaryFeeling:  "sad",
			SupportingEvent: "Bad meeting",
		}},
		GeneratedAt: generatedAt,
	}

	require.Equal(t, domain.WeeklySummary{
		UserID:           "usr_123",
		WeekStart:        weekStart,
		DominantFeelings: []domain.FeelingScore{{Label: "sad", Confidence: 0.73}},
		MainEvents:       []string{"Bad meeting"},
		TimelinePoints: []domain.TimelinePoint{{
			Date:            "2026-06-03",
			PrimaryFeeling:  "sad",
			SupportingEvent: "Bad meeting",
		}},
		GeneratedAt: generatedAt,
	}, document.toDomain())
}
