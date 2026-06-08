package services

import (
	"context"
	"strings"
	"time"

	analysisqueue "github.com/ravilock/sentir-mais-backend/internal/analysis/queue"
	"github.com/ravilock/sentir-mais-backend/internal/chat"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/id"
)

type SendMessageService struct {
	chats     chatFinder
	messages  messageCreator
	history   messageLister
	updater   chatUpdater
	responder llmResponder
	analysis  analysisJobEnqueuer
	clock     clock
}

func NewSendMessageService(chats chatFinder, messages messageCreator, history messageLister, updater chatUpdater, responder llmResponder, analysis analysisJobEnqueuer) *SendMessageService {
	return &SendMessageService{
		chats:     chats,
		messages:  messages,
		history:   history,
		updater:   updater,
		responder: responder,
		analysis:  analysis,
		clock:     realClock{},
	}
}

func (s *SendMessageService) SendMessage(ctx context.Context, chatID, userID, content string) (domain.Message, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return domain.Message{}, chat.ErrEmptyMessage
	}

	chatRecord, err := authorizeChat(ctx, s.chats, chatID, userID)
	if err != nil {
		return domain.Message{}, err
	}

	now := s.clock.Now()
	userMessageID, err := id.New("msg")
	if err != nil {
		return domain.Message{}, err
	}

	userMessage := domain.Message{ID: userMessageID, ChatID: chatID, UserID: userID, Sender: domain.SenderUser, Content: content, CreatedAt: now}
	if err := s.messages.Create(ctx, userMessage); err != nil {
		return domain.Message{}, err
	}

	history, err := s.history.ListByChatID(ctx, chatID)
	if err != nil {
		return domain.Message{}, err
	}

	replyContent, err := s.responder.GenerateReply(ctx, history)
	if err != nil {
		return domain.Message{}, err
	}

	assistantMessageID, err := id.New("msg")
	if err != nil {
		return domain.Message{}, err
	}

	assistantMessage := domain.Message{ID: assistantMessageID, ChatID: chatID, UserID: userID, Sender: domain.SenderAssistant, Content: replyContent, CreatedAt: now}
	if err := s.messages.Create(ctx, assistantMessage); err != nil {
		return domain.Message{}, err
	}

	chatRecord.UpdatedAt = now
	if err := s.updater.Update(ctx, chatRecord); err != nil {
		return domain.Message{}, err
	}
	if err := s.enqueueAnalysis(ctx, userMessage, now); err != nil {
		return domain.Message{}, err
	}

	return assistantMessage, nil
}

func (s *SendMessageService) enqueueAnalysis(ctx context.Context, message domain.Message, enqueuedAt time.Time) error {
	if s.analysis == nil {
		return nil
	}

	jobID, err := id.New("anj")
	if err != nil {
		return err
	}

	return s.analysis.Enqueue(ctx, analysisqueue.AnalysisJob{
		JobID:            jobID,
		ChatID:           message.ChatID,
		UserID:           message.UserID,
		MessageID:        message.ID,
		MessageCreatedAt: message.CreatedAt,
		EnqueuedAt:       enqueuedAt,
		Stage:            analysisqueue.StageExtract,
	})
}

func authorizeChat(ctx context.Context, finder chatFinder, chatID, userID string) (domain.Chat, error) {
	chatRecord, err := finder.FindByID(ctx, chatID)
	if err != nil {
		return domain.Chat{}, chat.ErrChatNotFound
	}
	if chatRecord.UserID != userID {
		return domain.Chat{}, chat.ErrChatNotFound
	}

	return chatRecord, nil
}
