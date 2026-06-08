package services

import (
	"context"
	"errors"
	"strings"
	"time"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
	"github.com/ravilock/sentir-mais-backend/internal/classifier"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/id"
)

var ErrMessageNotFound = errors.New("analysis message not found")

type MessageHistoryLister interface {
	ListByChatID(ctx context.Context, chatID string) ([]domain.Message, error)
}

type Extractor interface {
	ExtractEvent(ctx context.Context, history []domain.Message) (domain.ExtractedEvent, error)
}

type FeelingClassifier interface {
	Classify(ctx context.Context, text string) (domain.ClassificationResult, error)
}

type MessageAnalysisCreator interface {
	Create(ctx context.Context, analysis domain.MessageAnalysis) error
}

type SummaryWriter interface {
	UpdateForAnalysis(ctx context.Context, analysis domain.MessageAnalysis) error
}

type Clock interface {
	Now() time.Time
}

type Processor struct {
	history    MessageHistoryLister
	extractor  Extractor
	classifier FeelingClassifier
	analyses   MessageAnalysisCreator
	summaries  SummaryWriter
	clock      Clock
}

func NewProcessor(history MessageHistoryLister, extractor Extractor, classifier FeelingClassifier, analyses MessageAnalysisCreator, summaries SummaryWriter, clock Clock) *Processor {
	if clock == nil {
		clock = realClock{}
	}

	return &Processor{
		history:    history,
		extractor:  extractor,
		classifier: classifier,
		analyses:   analyses,
		summaries:  summaries,
		clock:      clock,
	}
}

func (p *Processor) Process(ctx context.Context, job analysisqueue.AnalysisJob) error {
	if p.history == nil {
		return errors.New("message history lister is required")
	}

	history, err := p.history.ListByChatID(ctx, job.ChatID)
	if err != nil {
		return err
	}

	message, ok := findTargetMessage(history, job)
	if !ok {
		return ErrMessageNotFound
	}

	return p.persistMessageAnalysis(ctx, history, message)
}

func findTargetMessage(history []domain.Message, job analysisqueue.AnalysisJob) (domain.Message, bool) {
	for _, message := range history {
		if message.ID != job.MessageID {
			continue
		}
		if message.ChatID != job.ChatID || message.UserID != job.UserID {
			return domain.Message{}, false
		}

		return message, true
	}

	return domain.Message{}, false
}

func (p *Processor) persistMessageAnalysis(ctx context.Context, history []domain.Message, message domain.Message) error {
	if p.analyses == nil || (p.classifier == nil && p.extractor == nil) {
		return nil
	}

	analysisID, err := id.New("anl")
	if err != nil {
		return err
	}

	analysis := domain.MessageAnalysis{
		ID:         analysisID,
		MessageID:  message.ID,
		ChatID:     message.ChatID,
		UserID:     message.UserID,
		SourceText: message.Content,
		CreatedAt:  analysisCreatedAt(message, p.clock),
	}

	if p.extractor != nil {
		extractedEvent, err := p.extractor.ExtractEvent(ctx, history)
		if err != nil {
			return err
		}

		analysis.ExtractedEvent = &extractedEvent
		analysis.EnoughContext = boolPointer(extractedEvent.EnoughContext)
		analysis.ContextGaps = extractedEvent.ContextGaps
	}

	if p.classifier != nil && shouldProceedToClassifier(analysis, p.extractor != nil) {
		classifierInputText := message.Content
		if analysis.ExtractedEvent != nil {
			classifierInputText = buildClassifierInputText(*analysis.ExtractedEvent)
		}

		result, err := p.classifier.Classify(ctx, classifierInputText)
		if err != nil {
			return err
		}

		analysis.ClassifierInputText = classifierInputText
		analysis.PrimaryFeeling = result.PrimaryFeeling
		analysis.SecondaryFeelings = result.SecondaryFeelings
		analysis.AllScores = result.AllScores
		if result.PrimaryFeeling.Label != "" {
			analysis.ClassifierProvider = classifier.ProviderName
			analysis.ClassifierModel = result.ModelName
		}
	}

	if analysis.ExtractedEvent == nil && analysis.PrimaryFeeling.Label == "" {
		return nil
	}

	if err := p.analyses.Create(ctx, analysis); err != nil {
		return err
	}
	if p.summaries == nil {
		return nil
	}

	return p.summaries.UpdateForAnalysis(ctx, analysis)
}

func boolPointer(value bool) *bool {
	return &value
}

func analysisCreatedAt(message domain.Message, clock Clock) time.Time {
	if !message.CreatedAt.IsZero() {
		return message.CreatedAt.UTC()
	}

	return clock.Now()
}

func shouldProceedToClassifier(analysis domain.MessageAnalysis, hasExtractor bool) bool {
	if !hasExtractor {
		return true
	}

	return determineNextAnalysisStep(analysis) == analysisNextStepProceedToNextStage
}

func buildClassifierInputText(event domain.ExtractedEvent) string {
	lines := make([]string, 0, 4)

	if value := strings.TrimSpace(event.WhatHappened); value != "" {
		lines = append(lines, "What happened: "+value)
	}
	if len(event.FeltEmotionsDescribedByUser) > 0 {
		emotions := compactClassifierStrings(event.FeltEmotionsDescribedByUser)
		if len(emotions) > 0 {
			lines = append(lines, "User felt: "+strings.Join(emotions, ", "))
		}
	}
	if value := strings.TrimSpace(event.UserReaction); value != "" {
		lines = append(lines, "User reaction: "+value)
	}
	if value := strings.TrimSpace(event.ExpectedOutcomeOrSelfExpectation); value != "" {
		lines = append(lines, "Expected outcome or self-expectation: "+value)
	}

	return strings.Join(lines, "\n")
}

func compactClassifierStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		compacted = append(compacted, trimmed)
	}

	return compacted
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now().UTC()
}
