package llm

import (
	"context"
	"strings"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

type SupportClient interface {
	GenerateReply(ctx context.Context, history []domain.Message) (string, error)
}

type StubSupportClient struct{}

func NewStubSupportClient() StubSupportClient {
	return StubSupportClient{}
}

func (StubSupportClient) GenerateReply(_ context.Context, history []domain.Message) (string, error) {
	lastMessage := ""
	for index := len(history) - 1; index >= 0; index-- {
		if history[index].Sender == domain.SenderUser {
			lastMessage = history[index].Content
			break
		}
	}

	if containsRiskLanguage(lastMessage) {
		return "Sinto muito que isso esteja tão pesado agora. Se houver risco imediato ou vontade de se machucar, procure ajuda humana agora mesmo: ligue 188 (CVV, Brasil) ou acione o serviço de emergência da sua região. Se puder, avise uma pessoa de confiança e não fique sozinho enquanto isso. Se quiser, posso continuar com você e organizar o que aconteceu passo a passo.", nil
	}

	if len(history) <= 1 {
		return "Obrigado por confiar isso aqui. Vamos com calma: o que aconteceu, o que você sentiu naquele momento, como você reagiu e o que você esperava que tivesse acontecido?", nil
	}

	return "Entendi. Quero te ajudar a organizar isso sem pressa: o que aconteceu, o que você sentiu, como você reagiu e o que esperava que tivesse acontecido ou que você tivesse feito?", nil
}

func containsRiskLanguage(message string) bool {
	normalized := strings.ToLower(message)
	keywords := []string{
		"suicid",
		"me matar",
		"tirar minha vida",
		"acabar com tudo",
		"self-harm",
		"me machucar",
	}

	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}

	return false
}
