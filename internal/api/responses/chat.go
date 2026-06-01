package responses

type CreateChatResponse struct {
	ChatID   string          `json:"chatId"`
	Response MessageResponse `json:"response"`
}

type MessageResponse struct {
	ID      string `json:"id"`
	Content string `json:"content"`
	Sender  int    `json:"sender"`
}

type ListMessagesResponse struct {
	ChatID   string            `json:"chatId"`
	Messages []MessageResponse `json:"messages"`
}
