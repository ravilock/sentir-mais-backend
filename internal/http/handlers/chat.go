package handlers

import (
	"errors"
	"net/http"

	apirequests "github.com/ravilock/sentir-mais-backend/internal/api/requests"
	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	"github.com/ravilock/sentir-mais-backend/internal/chat"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/http/middleware"
)

type ChatHandler struct {
	creator chatCreator
	sender  messageSender
	lister  messagesLister
}

func NewChatHandler(creator chatCreator, sender messageSender, lister messagesLister) *ChatHandler {
	return &ChatHandler{
		creator: creator,
		sender:  sender,
		lister:  lister,
	}
}

func (h *ChatHandler) CreateChat(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var request apirequests.CreateChatRequest
	if err := decodeJSON(r, &request); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	chatRecord, response, err := h.creator.CreateChat(r.Context(), user.ID, request.InitialMessage)
	if err != nil {
		switch {
		case errors.Is(err, chat.ErrEmptyMessage):
			respondError(w, http.StatusBadRequest, err.Error())
		default:
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
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var request apirequests.SendMessageRequest
	if err := decodeJSON(r, &request); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := h.sender.SendMessage(r.Context(), r.PathValue("chatId"), user.ID, request.Message)
	if err != nil {
		switch {
		case errors.Is(err, chat.ErrEmptyMessage):
			respondError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, chat.ErrChatNotFound):
			respondError(w, http.StatusNotFound, err.Error())
		default:
			respondError(w, http.StatusInternalServerError, "failed to send message")
		}
		return
	}

	respondJSON(w, http.StatusOK, toMessageResponse(response))
}

func (h *ChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	chatID := r.PathValue("chatId")
	messages, err := h.lister.ListMessages(r.Context(), chatID, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, chat.ErrChatNotFound):
			respondError(w, http.StatusNotFound, err.Error())
		default:
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
