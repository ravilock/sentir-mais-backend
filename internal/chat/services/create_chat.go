package services

import (
	"context"
	"strings"

	"github.com/ravilock/sentir-mais-backend/internal/chat"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/id"
)

type CreateChatService struct {
	chats      chatCreator
	messages   messageCreator
	responder  llmResponder
	extractor  llmExtractor
	classifier feelingClassifier
	analyses   messageAnalysisCreator
	summaries  summaryWriter
	clock      clock
}

func NewCreateChatService(chats chatCreator, messages messageCreator, responder llmResponder) *CreateChatService {
	return &CreateChatService{
		chats:     chats,
		messages:  messages,
		responder: responder,
		clock:     realClock{},
	}
}

func (s *CreateChatService) WithAnalysis(classifier feelingClassifier, analyses messageAnalysisCreator) *CreateChatService {
	s.classifier = classifier
	s.analyses = analyses
	return s
}

func (s *CreateChatService) WithExtraction(extractor llmExtractor) *CreateChatService {
	s.extractor = extractor
	return s
}

func (s *CreateChatService) WithSummaries(summaries summaryWriter) *CreateChatService {
	s.summaries = summaries
	return s
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
	if err := persistMessageAnalysis(ctx, s.classifier, s.extractor, s.analyses, s.summaries, s.clock, []domain.Message{userMessage, assistantMessage}, userMessage); err != nil {
		return domain.Chat{}, domain.Message{}, err
	}

	return chatRecord, assistantMessage, nil
}
