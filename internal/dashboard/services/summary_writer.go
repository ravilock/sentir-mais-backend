package services

import (
	"context"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type analysisRangeLister interface {
	ListByUserAndCreatedAtRange(ctx context.Context, userID string, start, end time.Time) ([]domain.MessageAnalysis, error)
}

type dailySummaryUpserter interface {
	Upsert(ctx context.Context, summary domain.DailySummary) error
}

type weeklySummaryUpserter interface {
	Upsert(ctx context.Context, summary domain.WeeklySummary) error
}

type SummaryWriter struct {
	analyses analysisRangeLister
	daily    dailySummaryUpserter
	weekly   weeklySummaryUpserter
}

func NewSummaryWriter(analyses analysisRangeLister, daily dailySummaryUpserter, weekly weeklySummaryUpserter) *SummaryWriter {
	return &SummaryWriter{
		analyses: analyses,
		daily:    daily,
		weekly:   weekly,
	}
}

func (w *SummaryWriter) UpdateForAnalysis(ctx context.Context, analysis domain.MessageAnalysis) error {
	if w == nil || w.analyses == nil || w.daily == nil || w.weekly == nil {
		return nil
	}

	dayStart := startOfDay(analysis.CreatedAt)
	dayEnd := dayStart.AddDate(0, 0, 1)
	dayAnalyses, err := w.analyses.ListByUserAndCreatedAtRange(ctx, analysis.UserID, dayStart, dayEnd)
	if err != nil {
		return err
	}

	if err := w.daily.Upsert(ctx, buildDailySummary(analysis.UserID, dayStart, analysis.CreatedAt, dayAnalyses)); err != nil {
		return err
	}

	weekStart := startOfWeek(analysis.CreatedAt)
	weekEnd := weekStart.AddDate(0, 0, 7)
	weekAnalyses, err := w.analyses.ListByUserAndCreatedAtRange(ctx, analysis.UserID, weekStart, weekEnd)
	if err != nil {
		return err
	}

	return w.weekly.Upsert(ctx, buildWeeklySummary(analysis.UserID, weekStart, analysis.CreatedAt, weekAnalyses))
}

func buildDailySummary(userID string, dayStart, generatedAt time.Time, analyses []domain.MessageAnalysis) domain.DailySummary {
	return domain.DailySummary{
		UserID:           userID,
		DayStart:         dayStart.UTC(),
		DominantFeelings: summarizeFeelings(analyses),
		MainEvents:       summarizeMainEvents(analyses),
		TimelinePoints:   summarizeTimelinePoints(analyses),
		GeneratedAt:      generatedAt.UTC(),
	}
}

func buildWeeklySummary(userID string, weekStart, generatedAt time.Time, analyses []domain.MessageAnalysis) domain.WeeklySummary {
	return domain.WeeklySummary{
		UserID:           userID,
		WeekStart:        weekStart.UTC(),
		DominantFeelings: summarizeFeelings(analyses),
		MainEvents:       summarizeMainEvents(analyses),
		TimelinePoints:   summarizeTimelinePoints(analyses),
		GeneratedAt:      generatedAt.UTC(),
	}
}

type feelingAggregate struct {
	count           int
	totalConfidence float64
}

func summarizeFeelings(analyses []domain.MessageAnalysis) []domain.FeelingScore {
	aggregates := map[string]feelingAggregate{}
	for _, analysis := range analyses {
		label := strings.TrimSpace(analysis.PrimaryFeeling.Label)
		if label == "" {
			continue
		}

		current := aggregates[label]
		current.count++
		current.totalConfidence += analysis.PrimaryFeeling.Confidence
		aggregates[label] = current
	}

	type rankedFeeling struct {
		score domain.FeelingScore
		count int
	}

	ranked := make([]rankedFeeling, 0, len(aggregates))
	for label, aggregate := range aggregates {
		ranked = append(ranked, rankedFeeling{
			score: domain.FeelingScore{
				Label:      label,
				Confidence: aggregate.totalConfidence / float64(aggregate.count),
			},
			count: aggregate.count,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		if ranked[i].score.Confidence != ranked[j].score.Confidence {
			return ranked[i].score.Confidence > ranked[j].score.Confidence
		}
		return ranked[i].score.Label < ranked[j].score.Label
	})

	scores := make([]domain.FeelingScore, 0, len(ranked))
	for _, item := range ranked {
		scores = append(scores, item.score)
	}

	return scores
}

func summarizeMainEvents(analyses []domain.MessageAnalysis) []string {
	seen := map[string]struct{}{}
	events := make([]string, 0, len(analyses))
	for _, analysis := range analyses {
		if analysis.ExtractedEvent == nil {
			continue
		}

		event := strings.TrimSpace(analysis.ExtractedEvent.EventSummary)
		if event == "" {
			continue
		}
		if _, exists := seen[event]; exists {
			continue
		}

		seen[event] = struct{}{}
		events = append(events, event)
	}

	return events
}

func summarizeTimelinePoints(analyses []domain.MessageAnalysis) []domain.TimelinePoint {
	sorted := append([]domain.MessageAnalysis(nil), analyses...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	points := make([]domain.TimelinePoint, 0, len(sorted))
	for _, analysis := range sorted {
		label := strings.TrimSpace(analysis.PrimaryFeeling.Label)
		if label == "" {
			continue
		}

		supportingEvent := ""
		if analysis.ExtractedEvent != nil {
			supportingEvent = strings.TrimSpace(analysis.ExtractedEvent.EventSummary)
		}

		points = append(points, domain.TimelinePoint{
			Date:            analysis.CreatedAt.UTC().Format(time.DateOnly),
			PrimaryFeeling:  label,
			SupportingEvent: supportingEvent,
		})
	}

	return points
}

func startOfDay(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func compactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		compacted = append(compacted, trimmed)
	}

	return slices.Clip(compacted)
}
