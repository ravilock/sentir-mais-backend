package services

import (
	"context"
	"strings"

	"github.com/ravilock/sentir-mais-backend/internal/classifier"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/id"
)

func persistMessageAnalysis(
	ctx context.Context,
	classifierClient feelingClassifier,
	extractor llmExtractor,
	analyses messageAnalysisCreator,
	summaries summaryWriter,
	serviceClock clock,
	history []domain.Message,
	message domain.Message,
) error {
	if analyses == nil || (classifierClient == nil && extractor == nil) {
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
		CreatedAt:  serviceClock.Now(),
	}

	if extractor != nil {
		extractedEvent, err := extractor.ExtractEvent(ctx, history)
		if err != nil {
			return err
		}

		analysis.ExtractedEvent = &extractedEvent
		analysis.EnoughContext = boolPointer(extractedEvent.EnoughContext)
		analysis.ContextGaps = extractedEvent.ContextGaps
	}

	if classifierClient != nil && shouldProceedToClassifier(analysis, extractor != nil) {
		classifierInputText := message.Content
		if analysis.ExtractedEvent != nil {
			classifierInputText = buildClassifierInputText(*analysis.ExtractedEvent)
		}

		result, err := classifierClient.Classify(ctx, classifierInputText)
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

	if err := analyses.Create(ctx, analysis); err != nil {
		return err
	}

	if summaries == nil {
		return nil
	}

	return summaries.UpdateForAnalysis(ctx, analysis)
}

func boolPointer(value bool) *bool {
	return &value
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
