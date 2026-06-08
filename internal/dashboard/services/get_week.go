package services

import (
	"context"
	"errors"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"go.mongodb.org/mongo-driver/mongo"
)

type weeklySummaryFinder interface {
	FindByUserAndWeek(ctx context.Context, userID string, weekStart time.Time) (domain.WeeklySummary, error)
}

type GetWeekService struct {
	summaries weeklySummaryFinder
	now       func() time.Time
}

func NewGetWeekService(summaries weeklySummaryFinder) *GetWeekService {
	return &GetWeekService{
		summaries: summaries,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func (s *GetWeekService) GetWeek(ctx context.Context, userID string) (domain.WeeklySummary, error) {
	now := s.now()
	weekStart := startOfWeek(now)
	summary, err := s.summaries.FindByUserAndWeek(ctx, userID, weekStart)
	if err == nil {
		return summary, nil
	}

	if !errors.Is(err, mongo.ErrNoDocuments) {
		return domain.WeeklySummary{}, err
	}

	return domain.WeeklySummary{
		UserID:           userID,
		WeekStart:        weekStart,
		DominantFeelings: []domain.FeelingScore{},
		MainEvents:       []string{},
		TimelinePoints:   []domain.TimelinePoint{},
		GeneratedAt:      now,
	}, nil
}

func startOfWeek(value time.Time) time.Time {
	weekday := int(value.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	dayStart := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
	return dayStart.AddDate(0, 0, -(weekday - 1))
}
