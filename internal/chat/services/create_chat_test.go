package services

import (
	"context"
	"testing"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/classifier"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateChatService_CreateChat(t *testing.T) {
	t.Run("should persist initial analysis when classifier is configured", func(t *testing.T) {
		chats := newMockChatCreator(t)
		messages := newMockMessageCreator(t)
		responder := newMockLlmResponder(t)
		feelingClassifier := newMockFeelingClassifier(t)
		analyses := newMockMessageAnalysisCreator(t)
		clock := newMockClock(t)

		now := time.Date(2026, time.May, 31, 14, 0, 0, 0, time.UTC)
		service := NewCreateChatService(chats, messages, responder).WithAnalysis(feelingClassifier, analyses)
		service.clock = clock

		clock.EXPECT().Now().Return(now).Twice()
		responder.EXPECT().
			GenerateReply(mock.Anything, mock.AnythingOfType("[]domain.Message")).
			Return("assistant reply", nil).
			Once()
		chats.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.Chat")).
			Return(nil).
			Once()
		messages.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.Message")).
			Return(nil).
			Twice()
		feelingClassifier.EXPECT().
			Classify(mock.Anything, "initial vent").
			Return(domain.ClassificationResult{
				PrimaryFeeling: domain.FeelingScore{Label: "anxious", Confidence: 0.91},
				SecondaryFeelings: []domain.FeelingScore{
					{Label: "tense", Confidence: 0.88},
				},
				AllScores: []domain.FeelingScore{
					{Label: "anxious", Confidence: 0.91},
					{Label: "tense", Confidence: 0.88},
				},
				ModelName: "MoritzLaurer/mDeBERTa-v3-base-mnli-xnli",
			}, nil).
			Once()
		analyses.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.MessageAnalysis")).
			RunAndReturn(func(_ context.Context, analysis domain.MessageAnalysis) error {
				require.NotEmpty(t, analysis.ID)
				require.NotEmpty(t, analysis.MessageID)
				require.NotEmpty(t, analysis.ChatID)
				require.Equal(t, "usr_123", analysis.UserID)
				require.Equal(t, "initial vent", analysis.SourceText)
				require.Equal(t, "anxious", analysis.PrimaryFeeling.Label)
				require.Equal(t, classifier.ProviderName, analysis.ClassifierProvider)
				require.Equal(t, "MoritzLaurer/mDeBERTa-v3-base-mnli-xnli", analysis.ClassifierModel)
				require.Equal(t, now, analysis.CreatedAt)
				return nil
			}).
			Once()

		chatRecord, response, err := service.CreateChat(context.Background(), "usr_123", " initial vent ")

		require.NoError(t, err)
		require.NotEmpty(t, chatRecord.ID)
		require.Equal(t, "assistant reply", response.Content)
	})
}
