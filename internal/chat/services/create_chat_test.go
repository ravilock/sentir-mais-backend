package services

import (
	"context"
	"errors"
	"testing"
	"time"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateChatService_CreateChat(t *testing.T) {
	t.Run("should enqueue initial message analysis job", func(t *testing.T) {
		chats := newMockChatCreator(t)
		messages := newMockMessageCreator(t)
		responder := newMockLlmResponder(t)
		clock := newMockClock(t)
		enqueuer := &capturingAnalysisJobEnqueuer{}

		now := time.Date(2026, time.May, 31, 14, 0, 0, 0, time.UTC)
		service := NewCreateChatService(chats, messages, responder, enqueuer, testLogger())
		service.clock = clock

		clock.EXPECT().Now().Return(now).Once()
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

		chatRecord, response, err := service.CreateChat(context.Background(), "usr_123", " initial vent ")

		require.NoError(t, err)
		require.NotEmpty(t, chatRecord.ID)
		require.Equal(t, "assistant reply", response.Content)
		require.Equal(t, 1, enqueuer.jobCount())

		job := enqueuer.lastJob()
		require.NotEmpty(t, job.JobID)
		require.Equal(t, chatRecord.ID, job.ChatID)
		require.Equal(t, "usr_123", job.UserID)
		require.NotEmpty(t, job.MessageID)
		require.Equal(t, now, job.MessageCreatedAt)
		require.Equal(t, now, job.EnqueuedAt)
		require.Equal(t, analysisqueue.StageExtract, job.Stage)
		require.Zero(t, job.Attempt)
	})

	t.Run("should still return chat response when analysis job cannot be enqueued", func(t *testing.T) {
		chats := newMockChatCreator(t)
		messages := newMockMessageCreator(t)
		responder := newMockLlmResponder(t)
		clock := newMockClock(t)
		expectedErr := errors.New("redis unavailable")
		enqueuer := &capturingAnalysisJobEnqueuer{err: expectedErr}

		now := time.Date(2026, time.May, 31, 14, 30, 0, 0, time.UTC)
		service := NewCreateChatService(chats, messages, responder, enqueuer, testLogger())
		service.clock = clock

		clock.EXPECT().Now().Return(now).Once()
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

		chatRecord, response, err := service.CreateChat(context.Background(), "usr_123", " initial vent ")

		require.NoError(t, err)
		require.NotEmpty(t, chatRecord.ID)
		require.Equal(t, "assistant reply", response.Content)
		require.Equal(t, 0, enqueuer.jobCount())
	})
}
