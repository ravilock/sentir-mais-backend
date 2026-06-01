package services

import (
	"context"
	"testing"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/chat"
	"github.com/ravilock/sentir-mais-backend/internal/classifier"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSendMessageService_SendMessage(t *testing.T) {
	t.Run("should persist user and assistant messages and update chat", func(t *testing.T) {
		chats := newMockChatFinder(t)
		messages := newMockMessageCreator(t)
		history := newMockMessageLister(t)
		updater := newMockChatUpdater(t)
		responder := newMockLlmResponder(t)
		clock := newMockClock(t)

		now := time.Date(2026, time.May, 31, 15, 0, 0, 0, time.UTC)
		service := NewSendMessageService(chats, messages, history, updater, responder)
		service.clock = clock

		chatRecord := domain.Chat{
			ID:        "cht_123",
			UserID:    "usr_123",
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Minute),
		}
		historyMessages := []domain.Message{
			{ID: "msg_prev", ChatID: "cht_123", UserID: "usr_123", Sender: domain.SenderUser, Content: "previous"},
		}

		chats.EXPECT().FindByID(mock.Anything, "cht_123").Return(chatRecord, nil).Once()
		clock.EXPECT().Now().Return(now).Once()
		messages.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.Message")).
			RunAndReturn(func(_ context.Context, message domain.Message) error {
				require.NotEmpty(t, message.ID)
				require.Equal(t, "cht_123", message.ChatID)
				require.Equal(t, "usr_123", message.UserID)
				require.Equal(t, domain.SenderUser, message.Sender)
				require.Equal(t, "new message", message.Content)
				require.Equal(t, now, message.CreatedAt)
				return nil
			}).
			Once()
		history.EXPECT().ListByChatID(mock.Anything, "cht_123").Return(historyMessages, nil).Once()
		responder.EXPECT().
			GenerateReply(mock.Anything, historyMessages).
			Return("assistant reply", nil).
			Once()
		messages.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.Message")).
			RunAndReturn(func(_ context.Context, message domain.Message) error {
				require.NotEmpty(t, message.ID)
				require.Equal(t, "cht_123", message.ChatID)
				require.Equal(t, "usr_123", message.UserID)
				require.Equal(t, domain.SenderAssistant, message.Sender)
				require.Equal(t, "assistant reply", message.Content)
				require.Equal(t, now, message.CreatedAt)
				return nil
			}).
			Once()
		updater.EXPECT().
			Update(mock.Anything, mock.AnythingOfType("domain.Chat")).
			RunAndReturn(func(_ context.Context, updated domain.Chat) error {
				require.Equal(t, "cht_123", updated.ID)
				require.Equal(t, "usr_123", updated.UserID)
				require.Equal(t, chatRecord.CreatedAt, updated.CreatedAt)
				require.Equal(t, now, updated.UpdatedAt)
				return nil
			}).
			Once()

		response, err := service.SendMessage(context.Background(), "cht_123", "usr_123", " new message ")

		require.NoError(t, err)
		require.NotEmpty(t, response.ID)
		require.Equal(t, "assistant reply", response.Content)
		require.Equal(t, domain.SenderAssistant, response.Sender)
	})

	t.Run("should reject empty content", func(t *testing.T) {
		service := NewSendMessageService(newMockChatFinder(t), newMockMessageCreator(t), newMockMessageLister(t), newMockChatUpdater(t), newMockLlmResponder(t))

		response, err := service.SendMessage(context.Background(), "cht_123", "usr_123", "   ")

		require.ErrorIs(t, err, chat.ErrEmptyMessage)
		require.Equal(t, domain.Message{}, response)
	})

	t.Run("should return not found when chat belongs to another user", func(t *testing.T) {
		chats := newMockChatFinder(t)
		service := NewSendMessageService(chats, newMockMessageCreator(t), newMockMessageLister(t), newMockChatUpdater(t), newMockLlmResponder(t))

		chats.EXPECT().
			FindByID(mock.Anything, "cht_123").
			Return(domain.Chat{ID: "cht_123", UserID: "usr_other"}, nil).
			Once()

		response, err := service.SendMessage(context.Background(), "cht_123", "usr_123", "hello")

		require.ErrorIs(t, err, chat.ErrChatNotFound)
		require.Equal(t, domain.Message{}, response)
	})

	t.Run("should classify and persist user message analysis", func(t *testing.T) {
		chats := newMockChatFinder(t)
		messages := newMockMessageCreator(t)
		history := newMockMessageLister(t)
		updater := newMockChatUpdater(t)
		responder := newMockLlmResponder(t)
		feelingClassifier := newMockFeelingClassifier(t)
		analyses := newMockMessageAnalysisCreator(t)
		clock := newMockClock(t)

		now := time.Date(2026, time.May, 31, 16, 0, 0, 0, time.UTC)
		service := NewSendMessageService(chats, messages, history, updater, responder).WithAnalysis(feelingClassifier, analyses)
		service.clock = clock

		chatRecord := domain.Chat{
			ID:        "cht_123",
			UserID:    "usr_123",
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Minute),
		}

		chats.EXPECT().FindByID(mock.Anything, "cht_123").Return(chatRecord, nil).Once()
		clock.EXPECT().Now().Return(now).Twice()
		messages.EXPECT().Create(mock.Anything, mock.AnythingOfType("domain.Message")).Return(nil).Twice()
		history.EXPECT().ListByChatID(mock.Anything, "cht_123").Return([]domain.Message{{ID: "msg_prev", ChatID: "cht_123", UserID: "usr_123", Content: "prev"}}, nil).Once()
		responder.EXPECT().GenerateReply(mock.Anything, mock.AnythingOfType("[]domain.Message")).Return("assistant reply", nil).Once()
		updater.EXPECT().Update(mock.Anything, mock.AnythingOfType("domain.Chat")).Return(nil).Once()
		feelingClassifier.EXPECT().
			Classify(mock.Anything, "new message").
			Return(domain.ClassificationResult{
				PrimaryFeeling: domain.FeelingScore{Label: "stressed", Confidence: 0.82},
				SecondaryFeelings: []domain.FeelingScore{
					{Label: "tense", Confidence: 0.7},
				},
				AllScores: []domain.FeelingScore{
					{Label: "stressed", Confidence: 0.82},
					{Label: "tense", Confidence: 0.7},
				},
				ModelName: "MoritzLaurer/mDeBERTa-v3-base-mnli-xnli",
			}, nil).
			Once()
		analyses.EXPECT().
			Create(mock.Anything, mock.AnythingOfType("domain.MessageAnalysis")).
			RunAndReturn(func(_ context.Context, analysis domain.MessageAnalysis) error {
				require.Equal(t, "usr_123", analysis.UserID)
				require.Equal(t, "cht_123", analysis.ChatID)
				require.Equal(t, "new message", analysis.SourceText)
				require.Equal(t, "stressed", analysis.PrimaryFeeling.Label)
				require.Equal(t, classifier.ProviderName, analysis.ClassifierProvider)
				require.Equal(t, now, analysis.CreatedAt)
				return nil
			}).
			Once()

		response, err := service.SendMessage(context.Background(), "cht_123", "usr_123", " new message ")

		require.NoError(t, err)
		require.Equal(t, "assistant reply", response.Content)
	})
}
