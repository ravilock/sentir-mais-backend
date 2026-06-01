package services

import (
	"context"

	"github.com/ravilock/sentir-mais-backend/internal/classifier"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/id"
)

func persistMessageAnalysis(
	ctx context.Context,
	classifierClient feelingClassifier,
	extractor llmExtractor,
	analyses messageAnalysisCreator,
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

	if classifierClient != nil {
		result, err := classifierClient.Classify(ctx, message.Content)
		if err != nil {
			return err
		}

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

	return analyses.Create(ctx, analysis)
}

func boolPointer(value bool) *bool {
	return &value
}
