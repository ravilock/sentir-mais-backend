package services

import (
	"context"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type GetWeekService struct{}

func NewGetWeekService() *GetWeekService {
	return &GetWeekService{}
}

func (s *GetWeekService) GetWeek(_ context.Context, userID string) domain.WeeklySummary {
	now := time.Now().UTC()
	return domain.WeeklySummary{
		UserID:           userID,
		WeekStart:        startOfWeek(now),
		DominantFeelings: []domain.FeelingScore{},
		MainEvents:       []string{},
		TimelinePoints:   []domain.TimelinePoint{},
		GeneratedAt:      now,
	}
}

func startOfWeek(value time.Time) time.Time {
	weekday := int(value.Weekday())
	if weekday == 0 {
		weekday = 7
	}

	dayStart := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
	return dayStart.AddDate(0, 0, -(weekday - 1))
}
