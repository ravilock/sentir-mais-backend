package responses

type CreateChatResponse struct {
	ChatID   string          `json:"chatId"`
	Response MessageResponse `json:"response"`
}

type ChatSummaryResponse struct {
	ID                 string `json:"id"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
	LastMessagePreview string `json:"lastMessagePreview"`
	LastMessageAt      string `json:"lastMessageAt"`
}

type MessageResponse struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Sender  int    `json:"sender"`
}

type ListChatsResponse struct {
	Chats []ChatSummaryResponse `json:"chats"`
}

type ListMessagesResponse struct {
	ChatID   string            `json:"chatId"`
	Messages []MessageResponse `json:"messages"`
}
