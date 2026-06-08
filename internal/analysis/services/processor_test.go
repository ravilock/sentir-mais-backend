package services

import (
	"context"
	"testing"
	"time"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
	"github.com/ravilock/sentir-mais-backend/internal/classifier"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

const classifierInputText = "What happened: The user argued with their manager at work.\nUser felt: anxious\nUser reaction: The user became defensive.\nExpected outcome or self-expectation: The user expected more respect."

func TestProcessorProcess(t *testing.T) {
	t.Run("should reload history and persist classifier and extraction result", func(t *testing.T) {
		messageCreatedAt := time.Date(2026, time.June, 1, 11, 0, 0, 0, time.UTC)
		history := []domain.Message{
			{ID: "msg_user", ChatID: "cht_123", UserID: "usr_123", Sender: domain.SenderUser, Content: "I argued with my manager and felt anxious", CreatedAt: messageCreatedAt},
			{ID: "msg_assistant", ChatID: "cht_123", UserID: "usr_123", Sender: domain.SenderAssistant, Content: "What happened next?", CreatedAt: messageCreatedAt.Add(time.Second)},
		}
		analyses := &capturingAnalysisCreator{}
		summaries := &capturingSummaryWriter{}
		processor := NewProcessor(
			stubHistoryLister{messages: history},
			stubExtractor{event: domain.ExtractedEvent{
				EnoughContext:                    true,
				EventSummary:                     "The user argued with their manager.",
				WhatHappened:                     "The user argued with their manager at work.",
				FeltEmotionsDescribedByUser:      []string{"anxious"},
				UserReaction:                     "The user became defensive.",
				ExpectedOutcomeOrSelfExpectation: "The user expected more respect.",
			}},
			stubClassifier{result: domain.ClassificationResult{
				PrimaryFeeling: domain.FeelingScore{Label: "anxious", Confidence: 0.91},
				ModelName:      "MoritzLaurer/mDeBERTa-v3-base-mnli-xnli",
			}},
			analyses,
			summaries,
			stubClock{now: messageCreatedAt.Add(time.Hour)},
		)

		err := processor.Process(context.Background(), analysisqueue.AnalysisJob{
			ChatID:    "cht_123",
			UserID:    "usr_123",
			MessageID: "msg_user",
		})

		require.NoError(t, err)
		require.Len(t, analyses.created, 1)
		require.Len(t, summaries.updated, 1)

		analysis := analyses.created[0]
		require.NotEmpty(t, analysis.ID)
		require.Equal(t, "msg_user", analysis.MessageID)
		require.Equal(t, "cht_123", analysis.ChatID)
		require.Equal(t, "usr_123", analysis.UserID)
		require.Equal(t, "I argued with my manager and felt anxious", analysis.SourceText)
		require.Equal(t, messageCreatedAt, analysis.CreatedAt)
		require.NotNil(t, analysis.ExtractedEvent)
		require.NotNil(t, analysis.EnoughContext)
		require.True(t, *analysis.EnoughContext)
		require.Equal(t, classifierInputText, analysis.ClassifierInputText)
		require.Equal(t, "anxious", analysis.PrimaryFeeling.Label)
		require.Equal(t, classifier.ProviderName, analysis.ClassifierProvider)
		require.Equal(t, "MoritzLaurer/mDeBERTa-v3-base-mnli-xnli", analysis.ClassifierModel)
		require.Equal(t, analysis, summaries.updated[0])
	})

	t.Run("should return not found when queued message is not in authoritative history", func(t *testing.T) {
		processor := NewProcessor(
			stubHistoryLister{messages: []domain.Message{{ID: "msg_other", ChatID: "cht_123", UserID: "usr_123"}}},
			nil,
			nil,
			&capturingAnalysisCreator{},
			nil,
			nil,
		)

		err := processor.Process(context.Background(), analysisqueue.AnalysisJob{
			ChatID:    "cht_123",
			UserID:    "usr_123",
			MessageID: "msg_missing",
		})

		require.ErrorIs(t, err, ErrMessageNotFound)
	})
}

type stubHistoryLister struct {
	messages []domain.Message
	err      error
}

func (s stubHistoryLister) ListByChatID(_ context.Context, _ string) ([]domain.Message, error) {
	return s.messages, s.err
}

type stubExtractor struct {
	event domain.ExtractedEvent
	err   error
}

func (s stubExtractor) ExtractEvent(_ context.Context, _ []domain.Message) (domain.ExtractedEvent, error) {
	return s.event, s.err
}

type stubClassifier struct {
	result domain.ClassificationResult
	err    error
}

func (s stubClassifier) Classify(_ context.Context, _ string) (domain.ClassificationResult, error) {
	return s.result, s.err
}

type capturingAnalysisCreator struct {
	created []domain.MessageAnalysis
	err     error
}

func (c *capturingAnalysisCreator) Create(_ context.Context, analysis domain.MessageAnalysis) error {
	if c.err != nil {
		return c.err
	}

	c.created = append(c.created, analysis)
	return nil
}

type capturingSummaryWriter struct {
	updated []domain.MessageAnalysis
	err     error
}

func (w *capturingSummaryWriter) UpdateForAnalysis(_ context.Context, analysis domain.MessageAnalysis) error {
	w.updated = append(w.updated, analysis)
	return w.err
}

type stubClock struct {
	now time.Time
}

func (c stubClock) Now() time.Time {
	return c.now
}
