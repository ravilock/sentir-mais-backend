package services

import (
	"context"
	"errors"
	"testing"
	"time"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
	"github.com/ravilock/sentir-mais-backend/internal/chat"
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
		enqueuer := &capturingAnalysisJobEnqueuer{}

		now := time.Date(2026, time.May, 31, 15, 0, 0, 0, time.UTC)
		service := NewSendMessageService(chats, messages, history, updater, responder, enqueuer)
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
		require.Equal(t, 1, enqueuer.jobCount())

		job := enqueuer.lastJob()
		require.NotEmpty(t, job.JobID)
		require.Equal(t, "cht_123", job.ChatID)
		require.Equal(t, "usr_123", job.UserID)
		require.NotEmpty(t, job.MessageID)
		require.Equal(t, now, job.MessageCreatedAt)
		require.Equal(t, now, job.EnqueuedAt)
		require.Equal(t, analysisqueue.StageExtract, job.Stage)
		require.Zero(t, job.Attempt)
	})

	t.Run("should reject empty content", func(t *testing.T) {
		service := NewSendMessageService(newMockChatFinder(t), newMockMessageCreator(t), newMockMessageLister(t), newMockChatUpdater(t), newMockLlmResponder(t), nil)

		response, err := service.SendMessage(context.Background(), "cht_123", "usr_123", "   ")

		require.ErrorIs(t, err, chat.ErrEmptyMessage)
		require.Equal(t, domain.Message{}, response)
	})

	t.Run("should return not found when chat belongs to another user", func(t *testing.T) {
		chats := newMockChatFinder(t)
		service := NewSendMessageService(chats, newMockMessageCreator(t), newMockMessageLister(t), newMockChatUpdater(t), newMockLlmResponder(t), nil)

		chats.EXPECT().
			FindByID(mock.Anything, "cht_123").
			Return(domain.Chat{ID: "cht_123", UserID: "usr_other"}, nil).
			Once()

		response, err := service.SendMessage(context.Background(), "cht_123", "usr_123", "hello")

		require.ErrorIs(t, err, chat.ErrChatNotFound)
		require.Equal(t, domain.Message{}, response)
	})

	t.Run("should still return assistant response when analysis job cannot be enqueued", func(t *testing.T) {
		chats := newMockChatFinder(t)
		messages := newMockMessageCreator(t)
		history := newMockMessageLister(t)
		updater := newMockChatUpdater(t)
		responder := newMockLlmResponder(t)
		clock := newMockClock(t)
		expectedErr := errors.New("redis unavailable")
		enqueuer := &capturingAnalysisJobEnqueuer{err: expectedErr}

		now := time.Date(2026, time.May, 31, 16, 0, 0, 0, time.UTC)
		service := NewSendMessageService(chats, messages, history, updater, responder, enqueuer)
		service.clock = clock

		chatRecord := domain.Chat{
			ID:        "cht_123",
			UserID:    "usr_123",
			CreatedAt: now.Add(-time.Hour),
			UpdatedAt: now.Add(-time.Minute),
		}

		chats.EXPECT().FindByID(mock.Anything, "cht_123").Return(chatRecord, nil).Once()
		clock.EXPECT().Now().Return(now).Once()
		messages.EXPECT().Create(mock.Anything, mock.AnythingOfType("domain.Message")).Return(nil).Twice()
		history.EXPECT().ListByChatID(mock.Anything, "cht_123").Return([]domain.Message{{ID: "msg_prev", ChatID: "cht_123", UserID: "usr_123", Content: "prev"}}, nil).Once()
		responder.EXPECT().GenerateReply(mock.Anything, mock.AnythingOfType("[]domain.Message")).Return("assistant reply", nil).Once()
		updater.EXPECT().Update(mock.Anything, mock.AnythingOfType("domain.Chat")).Return(nil).Once()

		response, err := service.SendMessage(context.Background(), "cht_123", "usr_123", " new message ")

		require.NoError(t, err)
		require.Equal(t, "assistant reply", response.Content)
		require.Equal(t, domain.SenderAssistant, response.Sender)
		require.Equal(t, 0, enqueuer.jobCount())
	})
}
