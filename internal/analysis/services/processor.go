package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
	"github.com/ravilock/sentir-mais-backend/internal/classifier"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/id"
)

const (
	extractionMaxAttempts  = 3
	classifierMaxAttempts  = 10
	summaryMaxAttempts     = 3
	stageRetryBackoff      = 5 * time.Second
	deadLetterReasonFailed = "stage_retry_exhausted"
)

var (
	ErrMessageNotFound = errors.New("analysis message not found")
	ErrDeadLettered    = errors.New("analysis job dead-lettered")
)

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

type MessageAnalysisFinder interface {
	ExistsByMessageID(ctx context.Context, messageID string) (bool, error)
}

type SummaryWriter interface {
	UpdateForAnalysis(ctx context.Context, analysis domain.MessageAnalysis) error
}

type DeadLetterCreator interface {
	Create(ctx context.Context, deadLetter domain.AnalysisDeadLetter) error
}

type Clock interface {
	Now() time.Time
}

type Processor struct {
	history     MessageHistoryLister
	extractor   Extractor
	classifier  FeelingClassifier
	analyses    MessageAnalysisCreator
	finder      MessageAnalysisFinder
	summaries   SummaryWriter
	deadLetters DeadLetterCreator
	clock       Clock
	sleep       func(context.Context, time.Duration) error
}

func NewProcessor(history MessageHistoryLister, extractor Extractor, classifier FeelingClassifier, analyses MessageAnalysisCreator, summaries SummaryWriter, clock Clock) *Processor {
	return NewProcessorWithDeadLetters(history, extractor, classifier, analyses, summaries, nil, clock)
}

func NewProcessorWithDeadLetters(history MessageHistoryLister, extractor Extractor, classifier FeelingClassifier, analyses MessageAnalysisCreator, summaries SummaryWriter, deadLetters DeadLetterCreator, clock Clock) *Processor {
	if clock == nil {
		clock = realClock{}
	}

	processor := &Processor{
		history:     history,
		extractor:   extractor,
		classifier:  classifier,
		analyses:    analyses,
		summaries:   summaries,
		deadLetters: deadLetters,
		clock:       clock,
		sleep:       sleepContext,
	}
	if finder, ok := analyses.(MessageAnalysisFinder); ok {
		processor.finder = finder
	}

	return processor
}

func (p *Processor) Process(ctx context.Context, job analysisqueue.AnalysisJob) error {
	if p.history == nil {
		return errors.New("message history lister is required")
	}
	if p.finder != nil {
		exists, err := p.finder.ExistsByMessageID(ctx, job.MessageID)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
	}

	history, err := p.history.ListByChatID(ctx, job.ChatID)
	if err != nil {
		return err
	}

	message, ok := findTargetMessage(history, job)
	if !ok {
		return p.deadLetter(ctx, job, string(analysisqueue.StageExtract), "message_not_found", ErrMessageNotFound)
	}

	return p.persistMessageAnalysis(ctx, job, history, message)
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

func (p *Processor) persistMessageAnalysis(ctx context.Context, job analysisqueue.AnalysisJob, history []domain.Message, message domain.Message) error {
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
		extractedEvent, err := p.extractEventWithRetries(ctx, history)
		if err != nil {
			if p.classifier == nil {
				return p.deadLetter(ctx, job, string(analysisqueue.StageExtract), deadLetterReasonFailed, err)
			}
		} else {
			analysis.ExtractedEvent = &extractedEvent
			analysis.EnoughContext = boolPointer(extractedEvent.EnoughContext)
			analysis.ContextGaps = extractedEvent.ContextGaps
		}
	}

	if p.classifier != nil && shouldProceedToClassifier(analysis, analysis.ExtractedEvent != nil) {
		classifierInputText := message.Content
		if analysis.ExtractedEvent != nil {
			classifierInputText = buildClassifierInputText(*analysis.ExtractedEvent)
		}

		result, err := p.classifyWithRetries(ctx, classifierInputText)
		if err != nil {
			return p.deadLetter(ctx, job, string(analysisqueue.StageClassify), deadLetterReasonFailed, err)
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

	if err := p.updateSummariesWithRetries(ctx, analysis); err != nil {
		return p.deadLetter(ctx, job, string(analysisqueue.StageSummaries), deadLetterReasonFailed, err)
	}

	return nil
}

func (p *Processor) extractEventWithRetries(ctx context.Context, history []domain.Message) (domain.ExtractedEvent, error) {
	var lastErr error
	for attempt := 1; attempt <= extractionMaxAttempts; attempt++ {
		event, err := p.extractor.ExtractEvent(ctx, history)
		if err == nil {
			return event, nil
		}

		lastErr = err
		if err := p.sleepBetweenAttempts(ctx, attempt, extractionMaxAttempts); err != nil {
			return domain.ExtractedEvent{}, err
		}
	}

	return domain.ExtractedEvent{}, lastErr
}

func (p *Processor) classifyWithRetries(ctx context.Context, text string) (domain.ClassificationResult, error) {
	var lastErr error
	for attempt := 1; attempt <= classifierMaxAttempts; attempt++ {
		result, err := p.classifier.Classify(ctx, text)
		if err == nil {
			return result, nil
		}

		lastErr = err
		if err := p.sleepBetweenAttempts(ctx, attempt, classifierMaxAttempts); err != nil {
			return domain.ClassificationResult{}, err
		}
	}

	return domain.ClassificationResult{}, lastErr
}

func (p *Processor) updateSummariesWithRetries(ctx context.Context, analysis domain.MessageAnalysis) error {
	var lastErr error
	for attempt := 1; attempt <= summaryMaxAttempts; attempt++ {
		if err := p.summaries.UpdateForAnalysis(ctx, analysis); err != nil {
			lastErr = err
			if err := p.sleepBetweenAttempts(ctx, attempt, summaryMaxAttempts); err != nil {
				return err
			}
			continue
		}

		return nil
	}

	return lastErr
}

func (p *Processor) sleepBetweenAttempts(ctx context.Context, attempt, maxAttempts int) error {
	if attempt >= maxAttempts {
		return nil
	}

	return p.sleep(ctx, stageRetryBackoff)
}

func (p *Processor) deadLetter(ctx context.Context, job analysisqueue.AnalysisJob, stage, reason string, cause error) error {
	if p.deadLetters == nil {
		return fmt.Errorf("%w: %s: %w", ErrDeadLettered, reason, cause)
	}

	deadLetterID, err := id.New("adl")
	if err != nil {
		return err
	}
	if err := p.deadLetters.Create(ctx, domain.AnalysisDeadLetter{
		ID:        deadLetterID,
		JobID:     job.JobID,
		ChatID:    job.ChatID,
		UserID:    job.UserID,
		MessageID: job.MessageID,
		Stage:     stage,
		Reason:    reason,
		Error:     cause.Error(),
		Attempt:   job.Attempt,
		CreatedAt: p.clock.Now(),
	}); err != nil {
		return err
	}

	return ErrDeadLettered
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

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
