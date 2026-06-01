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
	analyses messageAnalysisCreator,
	serviceClock clock,
	message domain.Message,
) error {
	if classifierClient == nil || analyses == nil {
		return nil
	}

	result, err := classifierClient.Classify(ctx, message.Content)
	if err != nil {
		return err
	}
	if result.PrimaryFeeling.Label == "" {
		return nil
	}

	analysisID, err := id.New("anl")
	if err != nil {
		return err
	}

	return analyses.Create(ctx, domain.MessageAnalysis{
		ID:                 analysisID,
		MessageID:          message.ID,
		ChatID:             message.ChatID,
		UserID:             message.UserID,
		SourceText:         message.Content,
		PrimaryFeeling:     result.PrimaryFeeling,
		SecondaryFeelings:  result.SecondaryFeelings,
		AllScores:          result.AllScores,
		ClassifierProvider: classifier.ProviderName,
		ClassifierModel:    result.ModelName,
		CreatedAt:          serviceClock.Now(),
	})
}
