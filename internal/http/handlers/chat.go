package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	apirequests "github.com/ravilock/sentir-mais-backend/internal/api/requests"
	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	"github.com/ravilock/sentir-mais-backend/internal/chat"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/http/middleware"
)

type ChatHandler struct {
	logger  *slog.Logger
	creator chatCreator
	sender  messageSender
	chats   chatsLister
	lister  messagesLister
}

func NewChatHandler(logger *slog.Logger, creator chatCreator, sender messageSender, chats chatsLister, lister messagesLister) *ChatHandler {
	return &ChatHandler{
		logger:  logger,
		creator: creator,
		sender:  sender,
		chats:   chats,
		lister:  lister,
	}
}

func (h *ChatHandler) CreateChat(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		logRequestError(h.logger, r, http.StatusUnauthorized, "missing authenticated user in create chat request", nil)
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var request apirequests.CreateChatRequest
	if err := decodeJSON(r, &request); err != nil {
		logRequestError(h.logger, r, http.StatusBadRequest, "failed to decode create chat request", err)
		respondDecodeError(w, err)
		return
	}
	if err := request.Validate(); err != nil {
		logRequestError(h.logger, r, http.StatusUnprocessableEntity, "failed to validate create chat request", err)
		respondDecodeError(w, err)
		return
	}

	chatRecord, response, err := h.creator.CreateChat(r.Context(), user.ID, request.InitialMessage)
	if err != nil {
		switch {
		case errors.Is(err, chat.ErrEmptyMessage):
			logRequestError(h.logger, r, http.StatusBadRequest, "create chat request failed", err)
			respondError(w, http.StatusBadRequest, err.Error())
		default:
			logRequestError(h.logger, r, http.StatusInternalServerError, "create chat request failed", err)
			respondError(w, http.StatusInternalServerError, "failed to create chat")
		}
		return
	}

	respondJSON(w, http.StatusCreated, apiresponses.CreateChatResponse{
		ChatID:   chatRecord.ID,
		Response: toMessageResponse(response),
	})
}

func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		logRequestError(h.logger, r, http.StatusUnauthorized, "missing authenticated user in send message request", nil)
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var request apirequests.SendMessageRequest
	if err := decodeJSON(r, &request); err != nil {
		logRequestError(h.logger, r, http.StatusBadRequest, "failed to decode send message request", err)
		respondDecodeError(w, err)
		return
	}
	if err := request.Validate(); err != nil {
		logRequestError(h.logger, r, http.StatusUnprocessableEntity, "failed to validate send message request", err)
		respondDecodeError(w, err)
		return
	}

	response, err := h.sender.SendMessage(r.Context(), r.PathValue("chatId"), user.ID, request.Message)
	if err != nil {
		switch {
		case errors.Is(err, chat.ErrEmptyMessage):
			logRequestError(h.logger, r, http.StatusBadRequest, "send message request failed", err)
			respondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, chat.ErrChatNotFound):
			logRequestError(h.logger, r, http.StatusNotFound, "send message request failed", err)
			respondError(w, http.StatusNotFound, err.Error())
		default:
			logRequestError(h.logger, r, http.StatusInternalServerError, "send message request failed", err)
			respondError(w, http.StatusInternalServerError, "failed to send message")
		}
		return
	}

	respondJSON(w, http.StatusOK, toMessageResponse(response))
}

func (h *ChatHandler) ListChats(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		logRequestError(h.logger, r, http.StatusUnauthorized, "missing authenticated user in list chats request", nil)
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	chats, err := h.chats.ListChats(r.Context(), user.ID)
	if err != nil {
		logRequestError(h.logger, r, http.StatusInternalServerError, "list chats request failed", err)
		respondError(w, http.StatusInternalServerError, "failed to list chats")
		return
	}

	response := apiresponses.ListChatsResponse{
		Chats: make([]apiresponses.ChatSummaryResponse, 0, len(chats)),
	}
	for _, chatSummary := range chats {
		response.Chats = append(response.Chats, apiresponses.ChatSummaryResponse{
			ID:                 chatSummary.ID,
			CreatedAt:          chatSummary.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:          chatSummary.UpdatedAt.UTC().Format(time.RFC3339),
			LastMessagePreview: chatSummary.LastMessagePreview,
			LastMessageAt:      chatSummary.LastMessageAt.UTC().Format(time.RFC3339),
		})
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *ChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		logRequestError(h.logger, r, http.StatusUnauthorized, "missing authenticated user in list messages request", nil)
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	chatID := r.PathValue("chatId")
	messages, err := h.lister.ListMessages(r.Context(), chatID, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, chat.ErrChatNotFound):
			logRequestError(h.logger, r, http.StatusNotFound, "list messages request failed", err)
			respondError(w, http.StatusNotFound, err.Error())
		default:
			logRequestError(h.logger, r, http.StatusInternalServerError, "list messages request failed", err)
			respondError(w, http.StatusInternalServerError, "failed to list messages")
		}
		return
	}

	response := apiresponses.ListMessagesResponse{
		ChatID:   chatID,
		Messages: make([]apiresponses.MessageResponse, 0, len(messages)),
	}
	for _, message := range messages {
		response.Messages = append(response.Messages, toMessageResponse(message))
	}

	respondJSON(w, http.StatusOK, response)
}

func toMessageResponse(message domain.Message) apiresponses.MessageResponse {
	return apiresponses.MessageResponse{
		ID:      message.ID,
		Content: message.Content,
		Sender:  int(message.Sender),
	}
}
