package services

import (
	"context"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type dailySummaryRangeLister interface {
	ListByUserAndDayRange(ctx context.Context, userID string, from, to time.Time) ([]domain.DailySummary, error)
}

type GetTimelineService struct {
	summaries dailySummaryRangeLister
	now       func() time.Time
}

func NewGetTimelineService(summaries dailySummaryRangeLister) *GetTimelineService {
	return &GetTimelineService{
		summaries: summaries,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func (s *GetTimelineService) GetTimeline(ctx context.Context, userID string, from, to *time.Time) (domain.DashboardTimeline, error) {
	rangeStart, rangeEnd := s.resolveRange(from, to)
	days, err := s.summaries.ListByUserAndDayRange(ctx, userID, rangeStart, rangeEnd)
	if err != nil {
		return domain.DashboardTimeline{}, err
	}

	return domain.DashboardTimeline{
		From: rangeStart,
		To:   rangeEnd,
		Days: days,
	}, nil
}

func (s *GetTimelineService) resolveRange(from, to *time.Time) (time.Time, time.Time) {
	if from != nil && to != nil {
		return startOfDay(from.UTC()), startOfDay(to.UTC())
	}

	rangeEnd := startOfDay(s.now())
	rangeStart := rangeEnd.AddDate(0, 0, -29)
	return rangeStart, rangeEnd
}
