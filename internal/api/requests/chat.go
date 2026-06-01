package requests

type CreateChatRequest struct {
	InitialMessage string `json:"initialMessage"`
}

type SendMessageRequest struct {
	Message string `json:"message"`
}
