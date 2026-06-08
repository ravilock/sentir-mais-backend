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

const classifierInputText = "What happened: The user argued with their manager at work.\nUser felt: anxious\nUser reaction: The user became defensive.\nExpected outcome or self-expectation: The user expected more respect."

func TestPersistMessageAnalysis(t *testing.T) {
	t.Run("should persist extracted event and sufficiency state", func(t *testing.T) {
		extractor := newMockLlmExtractor(t)
		analyses := newMockMessageAnalysisCreator(t)
		clock := newMockClock(t)

		now := time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
		history := []domain.Message{
			{ID: "msg_user", Sender: domain.SenderUser, Content: "I argued with my manager and felt anxious"},
			{ID: "msg_assistant", Sender: domain.SenderAssistant, Content: "What happened next?"},
		}
		userMessage := domain.Message{
			ID:      "msg_user",
			ChatID:  "cht_123",
			UserID:  "usr_123",
			Content: "I argued with my manager and felt anxious",
		}

		clock.EXPECT().Now().Return(now).Once()
		extractor.EXPECT().
			ExtractEvent(mock.Anything, history).
			Return(domain.ExtractedEvent{
				EnoughContext:               false,
				ContextGaps:                 []domain.ContextGap{domain.ContextGapUserReaction},
				EventSummary:                "The user argued with their manager.",
				WhatHappened:                "The user argued with their manager at work.",
				FeltEmotionsDescribedByUser: []string{"anxious"},
			}, nil).
			Once()
		analyses.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.MessageAnalysis")).
			RunAndReturn(func(_ context.Context, analysis domain.MessageAnalysis) error {
				require.Equal(t, "usr_123", analysis.UserID)
				require.Equal(t, "cht_123", analysis.ChatID)
				require.NotNil(t, analysis.ExtractedEvent)
				require.NotNil(t, analysis.EnoughContext)
				require.False(t, *analysis.EnoughContext)
				require.Equal(t, []domain.ContextGap{domain.ContextGapUserReaction}, analysis.ContextGaps)
				require.Equal(t, "The user argued with their manager.", analysis.ExtractedEvent.EventSummary)
				require.Equal(t, now, analysis.CreatedAt)
				require.Empty(t, analysis.ClassifierProvider)
				return nil
			}).
			Once()

		err := persistMessageAnalysis(context.Background(), nil, extractor, analyses, nil, clock, history, userMessage)

		require.NoError(t, err)
	})

	t.Run("should not classify when extraction says context is insufficient", func(t *testing.T) {
		feelingClassifier := newMockFeelingClassifier(t)
		extractor := newMockLlmExtractor(t)
		analyses := newMockMessageAnalysisCreator(t)
		clock := newMockClock(t)

		now := time.Date(2026, time.June, 1, 10, 30, 0, 0, time.UTC)
		history := []domain.Message{
			{ID: "msg_user", Sender: domain.SenderUser, Content: "I argued with my manager and felt anxious"},
		}
		userMessage := domain.Message{
			ID:      "msg_user",
			ChatID:  "cht_123",
			UserID:  "usr_123",
			Content: "I argued with my manager and felt anxious",
		}

		clock.EXPECT().Now().Return(now).Once()
		extractor.EXPECT().
			ExtractEvent(mock.Anything, history).
			Return(domain.ExtractedEvent{
				EnoughContext: false,
				ContextGaps:   []domain.ContextGap{domain.ContextGapUserReaction},
				EventSummary:  "The user argued with their manager.",
			}, nil).
			Once()
		analyses.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.MessageAnalysis")).
			RunAndReturn(func(_ context.Context, analysis domain.MessageAnalysis) error {
				require.NotNil(t, analysis.ExtractedEvent)
				require.NotNil(t, analysis.EnoughContext)
				require.False(t, *analysis.EnoughContext)
				require.Empty(t, analysis.PrimaryFeeling.Label)
				require.Empty(t, analysis.ClassifierProvider)
				return nil
			}).
			Once()

		err := persistMessageAnalysis(context.Background(), feelingClassifier, extractor, analyses, nil, clock, history, userMessage)

		require.NoError(t, err)
	})

	t.Run("should persist classifier and extraction together", func(t *testing.T) {
		feelingClassifier := newMockFeelingClassifier(t)
		extractor := newMockLlmExtractor(t)
		analyses := newMockMessageAnalysisCreator(t)
		clock := newMockClock(t)

		now := time.Date(2026, time.June, 1, 11, 0, 0, 0, time.UTC)
		history := []domain.Message{
			{ID: "msg_user", Sender: domain.SenderUser, Content: "I argued with my manager and felt anxious"},
		}
		userMessage := domain.Message{
			ID:      "msg_user",
			ChatID:  "cht_123",
			UserID:  "usr_123",
			Content: "I argued with my manager and felt anxious",
		}

		clock.EXPECT().Now().Return(now).Once()
		extractor.EXPECT().
			ExtractEvent(mock.Anything, history).
			Return(domain.ExtractedEvent{
				EnoughContext:                    true,
				EventSummary:                     "The user argued with their manager.",
				WhatHappened:                     "The user argued with their manager at work.",
				FeltEmotionsDescribedByUser:      []string{"anxious"},
				UserReaction:                     "The user became defensive.",
				ExpectedOutcomeOrSelfExpectation: "The user expected more respect.",
			}, nil).
			Once()
		feelingClassifier.EXPECT().
			Classify(mock.Anything, classifierInputText).
			Return(domain.ClassificationResult{
				PrimaryFeeling: domain.FeelingScore{Label: "anxious", Confidence: 0.91},
				ModelName:      "MoritzLaurer/mDeBERTa-v3-base-mnli-xnli",
			}, nil).
			Once()
		analyses.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.MessageAnalysis")).
			RunAndReturn(func(_ context.Context, analysis domain.MessageAnalysis) error {
				require.NotNil(t, analysis.ExtractedEvent)
				require.NotNil(t, analysis.EnoughContext)
				require.True(t, *analysis.EnoughContext)
				require.Equal(t, classifierInputText, analysis.ClassifierInputText)
				require.Equal(t, "anxious", analysis.PrimaryFeeling.Label)
				require.Equal(t, classifier.ProviderName, analysis.ClassifierProvider)
				return nil
			}).
			Once()

		err := persistMessageAnalysis(context.Background(), feelingClassifier, extractor, analyses, nil, clock, history, userMessage)

		require.NoError(t, err)
	})

	t.Run("should still classify when extractor is not configured", func(t *testing.T) {
		feelingClassifier := newMockFeelingClassifier(t)
		analyses := newMockMessageAnalysisCreator(t)
		clock := newMockClock(t)

		now := time.Date(2026, time.June, 1, 11, 30, 0, 0, time.UTC)
		userMessage := domain.Message{
			ID:      "msg_user",
			ChatID:  "cht_123",
			UserID:  "usr_123",
			Content: "I argued with my manager and felt anxious",
		}

		clock.EXPECT().Now().Return(now).Once()
		feelingClassifier.EXPECT().
			Classify(mock.Anything, userMessage.Content).
			Return(domain.ClassificationResult{
				PrimaryFeeling: domain.FeelingScore{Label: "anxious", Confidence: 0.91},
				ModelName:      "MoritzLaurer/mDeBERTa-v3-base-mnli-xnli",
			}, nil).
			Once()
		analyses.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.MessageAnalysis")).
			RunAndReturn(func(_ context.Context, analysis domain.MessageAnalysis) error {
				require.Nil(t, analysis.ExtractedEvent)
				require.Equal(t, userMessage.Content, analysis.ClassifierInputText)
				require.Equal(t, "anxious", analysis.PrimaryFeeling.Label)
				require.Equal(t, classifier.ProviderName, analysis.ClassifierProvider)
				return nil
			}).
			Once()

		err := persistMessageAnalysis(context.Background(), feelingClassifier, nil, analyses, nil, clock, nil, userMessage)

		require.NoError(t, err)
	})

	t.Run("should update summaries after persisting analysis", func(t *testing.T) {
		feelingClassifier := newMockFeelingClassifier(t)
		analyses := newMockMessageAnalysisCreator(t)
		clock := newMockClock(t)
		summaries := &capturingSummaryWriter{}

		now := time.Date(2026, time.June, 1, 12, 0, 0, 0, time.UTC)
		userMessage := domain.Message{
			ID:      "msg_user",
			ChatID:  "cht_123",
			UserID:  "usr_123",
			Content: "I argued with my manager and felt anxious",
		}

		clock.EXPECT().Now().Return(now).Once()
		feelingClassifier.EXPECT().
			Classify(mock.Anything, userMessage.Content).
			Return(domain.ClassificationResult{
				PrimaryFeeling: domain.FeelingScore{Label: "anxious", Confidence: 0.91},
				ModelName:      "MoritzLaurer/mDeBERTa-v3-base-mnli-xnli",
			}, nil).
			Once()
		analyses.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.MessageAnalysis")).
			Return(nil).
			Once()

		err := persistMessageAnalysis(context.Background(), feelingClassifier, nil, analyses, summaries, clock, nil, userMessage)

		require.NoError(t, err)
		require.Len(t, summaries.analyses, 1)
		require.Equal(t, "usr_123", summaries.analyses[0].UserID)
		require.Equal(t, "anxious", summaries.analyses[0].PrimaryFeeling.Label)
	})

	t.Run("should return summary update errors after analysis persistence", func(t *testing.T) {
		feelingClassifier := newMockFeelingClassifier(t)
		analyses := newMockMessageAnalysisCreator(t)
		clock := newMockClock(t)
		summaries := &capturingSummaryWriter{err: context.DeadlineExceeded}

		now := time.Date(2026, time.June, 1, 12, 30, 0, 0, time.UTC)
		userMessage := domain.Message{
			ID:      "msg_user",
			ChatID:  "cht_123",
			UserID:  "usr_123",
			Content: "I argued with my manager and felt anxious",
		}

		clock.EXPECT().Now().Return(now).Once()
		feelingClassifier.EXPECT().
			Classify(mock.Anything, userMessage.Content).
			Return(domain.ClassificationResult{
				PrimaryFeeling: domain.FeelingScore{Label: "anxious", Confidence: 0.91},
				ModelName:      "MoritzLaurer/mDeBERTa-v3-base-mnli-xnli",
			}, nil).
			Once()
		analyses.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.MessageAnalysis")).
			Return(nil).
			Once()

		err := persistMessageAnalysis(context.Background(), feelingClassifier, nil, analyses, summaries, clock, nil, userMessage)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Len(t, summaries.analyses, 1)
	})
}

func TestBuildClassifierInputText(t *testing.T) {
	t.Run("should build normalized text from the four core extracted fields", func(t *testing.T) {
		input := buildClassifierInputText(domain.ExtractedEvent{
			WhatHappened:                     " The user argued with their manager at work. ",
			FeltEmotionsDescribedByUser:      []string{" anxious ", ""},
			UserReaction:                     " The user became defensive. ",
			ExpectedOutcomeOrSelfExpectation: " The user expected more respect. ",
			EventSummary:                     "ignored",
			PeopleInvolved:                   []string{"manager"},
			Setting:                          "work",
		})

		require.Equal(t, classifierInputText, input)
	})
}

type capturingSummaryWriter struct {
	analyses []domain.MessageAnalysis
	err      error
}

func (w *capturingSummaryWriter) UpdateForAnalysis(_ context.Context, analysis domain.MessageAnalysis) error {
	w.analyses = append(w.analyses, analysis)
	return w.err
}
