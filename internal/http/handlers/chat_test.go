package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	"github.com/ravilock/sentir-mais-backend/internal/chat"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	httpmiddleware "github.com/ravilock/sentir-mais-backend/internal/http/middleware"
	"github.com/ravilock/sentir-mais-backend/internal/validations"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type chatHandlerAuthStub struct {
	user domain.User
}

func (s chatHandlerAuthStub) Authenticate(_ context.Context, _ string) (domain.User, error) {
	return s.user, nil
}

func TestChatHandler_CreateChat(t *testing.T) {
	require.NoError(t, validations.InitValidator())
	creator := newMockChatCreator(t)
	handler := NewChatHandler(newTestHTTPLogger(), creator, newMockMessageSender(t), newMockMessagesLister(t))

	req := httptest.NewRequest(http.MethodPost, "/chats", bytes.NewReader([]byte(`{"initialMessage":"I need help"}`)))
	req.Header.Set("Authorization", "Bearer tok_123")
	rec := httptest.NewRecorder()

	creator.EXPECT().
		CreateChat(mock.Anything, "usr_123", "I need help").
		Return(
			domain.Chat{ID: "cht_123"},
			domain.Message{ID: "msg_123", Content: "assistant reply", Sender: domain.SenderAssistant, CreatedAt: time.Now().UTC()},
			nil,
		).
		Once()

	protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.CreateChat))
	protected.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.JSONEq(t, `{"chatId":"cht_123","response":{"id":"msg_123","content":"assistant reply","sender":1}}`, rec.Body.String())
}

func TestChatHandler_CreateChatValidation(t *testing.T) {
	require.NoError(t, validations.InitValidator())
	handler := NewChatHandler(newTestHTTPLogger(), newMockChatCreator(t), newMockMessageSender(t), newMockMessagesLister(t))
	req := httptest.NewRequest(http.MethodPost, "/chats", bytes.NewReader([]byte(`{"initialMessage":"   "}`)))
	req.Header.Set("Authorization", "Bearer tok_123")
	rec := httptest.NewRecorder()

	protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.CreateChat))
	protected.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.JSONEq(t, `{"message":"field 'InitialMessage' is required"}`, rec.Body.String())
}

func TestChatHandler_SendMessage(t *testing.T) {
	require.NoError(t, validations.InitValidator())
	sender := newMockMessageSender(t)
	handler := NewChatHandler(newTestHTTPLogger(), newMockChatCreator(t), sender, newMockMessagesLister(t))

	req := httptest.NewRequest(http.MethodPost, "/chats/cht_123/messages", bytes.NewReader([]byte(`{"message":"follow up"}`)))
	req.SetPathValue("chatId", "cht_123")
	req.Header.Set("Authorization", "Bearer tok_123")
	rec := httptest.NewRecorder()

	sender.EXPECT().
		SendMessage(mock.Anything, "cht_123", "usr_123", "follow up").
		Return(domain.Message{ID: "msg_456", Content: "reply", Sender: domain.SenderAssistant}, nil).
		Once()

	protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.SendMessage))
	protected.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"id":"msg_456","content":"reply","sender":1}`, rec.Body.String())
}

func TestChatHandler_SendMessageValidation(t *testing.T) {
	require.NoError(t, validations.InitValidator())
	handler := NewChatHandler(newTestHTTPLogger(), newMockChatCreator(t), newMockMessageSender(t), newMockMessagesLister(t))
	req := httptest.NewRequest(http.MethodPost, "/chats/cht_123/messages", bytes.NewReader([]byte(`{"message":""}`)))
	req.SetPathValue("chatId", "cht_123")
	req.Header.Set("Authorization", "Bearer tok_123")
	rec := httptest.NewRecorder()

	protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.SendMessage))
	protected.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.JSONEq(t, `{"message":"field 'Message' is required"}`, rec.Body.String())
}

func TestChatHandler_ListMessages(t *testing.T) {
	lister := newMockMessagesLister(t)
	handler := NewChatHandler(newTestHTTPLogger(), newMockChatCreator(t), newMockMessageSender(t), lister)

	t.Run("should return messages", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/chats/cht_123/messages", nil)
		req.SetPathValue("chatId", "cht_123")
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		lister.EXPECT().
			ListMessages(mock.Anything, "cht_123", "usr_123").
			Return([]domain.Message{
				{ID: "msg_1", Content: "hello", Sender: domain.SenderUser},
				{ID: "msg_2", Content: "reply", Sender: domain.SenderAssistant},
			}, nil).
			Once()

		protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.ListMessages))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var payload apiresponses.ListMessagesResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		require.Equal(t, "cht_123", payload.ChatID)
	})

	t.Run("should map chat not found to 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/chats/cht_missing/messages", nil)
		req.SetPathValue("chatId", "cht_missing")
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		lister.EXPECT().
			ListMessages(mock.Anything, "cht_missing", "usr_123").
			Return(nil, chat.ErrChatNotFound).
			Once()

		protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.ListMessages))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
		require.JSONEq(t, `{"message":"chat not found"}`, rec.Body.String())
	})
}
