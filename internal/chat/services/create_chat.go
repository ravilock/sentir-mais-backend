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

type CreateChatService struct {
	chats     chatCreator
	messages  messageCreator
	responder llmResponder
	analysis  analysisJobEnqueuer
	clock     clock
}

func NewCreateChatService(chats chatCreator, messages messageCreator, responder llmResponder, analysis analysisJobEnqueuer) *CreateChatService {
	return &CreateChatService{
		chats:     chats,
		messages:  messages,
		responder: responder,
		analysis:  analysis,
		clock:     realClock{},
	}
}

func (s *CreateChatService) CreateChat(ctx context.Context, userID, initialMessage string) (domain.Chat, domain.Message, error) {
	initialMessage = strings.TrimSpace(initialMessage)
	if initialMessage == "" {
		return domain.Chat{}, domain.Message{}, chat.ErrEmptyMessage
	}

	now := s.clock.Now()
	chatID, err := id.New("cht")
	if err != nil {
		return domain.Chat{}, domain.Message{}, err
	}

	userMessageID, err := id.New("msg")
	if err != nil {
		return domain.Chat{}, domain.Message{}, err
	}

	assistantMessageID, err := id.New("msg")
	if err != nil {
		return domain.Chat{}, domain.Message{}, err
	}

	chatRecord := domain.Chat{ID: chatID, UserID: userID, CreatedAt: now, UpdatedAt: now}
	userMessage := domain.Message{ID: userMessageID, ChatID: chatID, UserID: userID, Sender: domain.SenderUser, Content: initialMessage, CreatedAt: now}

	replyContent, err := s.responder.GenerateReply(ctx, []domain.Message{userMessage})
	if err != nil {
		return domain.Chat{}, domain.Message{}, err
	}

	assistantMessage := domain.Message{ID: assistantMessageID, ChatID: chatID, UserID: userID, Sender: domain.SenderAssistant, Content: replyContent, CreatedAt: now}

	if err := s.chats.Create(ctx, chatRecord); err != nil {
		return domain.Chat{}, domain.Message{}, err
	}
	if err := s.messages.Create(ctx, userMessage); err != nil {
		return domain.Chat{}, domain.Message{}, err
	}
	if err := s.messages.Create(ctx, assistantMessage); err != nil {
		return domain.Chat{}, domain.Message{}, err
	}
	if err := s.enqueueAnalysis(ctx, userMessage, now); err != nil {
		return domain.Chat{}, domain.Message{}, err
	}

	return chatRecord, assistantMessage, nil
}

func (s *CreateChatService) enqueueAnalysis(ctx context.Context, message domain.Message, enqueuedAt time.Time) error {
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
