package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
)

const (
	prompterKindSupportive = "supportive"
	prompterUserAgent      = "sentir-mais-backend"
)

const supportiveSystemPrompt = `You are Sentir Mais, a calm and supportive emotional support assistant.

Your role is to help the user slow down, feel heard, and organize what happened without judging or diagnosing them.

You are not a doctor, therapist, or crisis professional. Do not present yourself as one. Do not give clinical diagnoses. Do not claim certainty about the user's mental health state.

Your main conversational goals are:
1. respond with empathy and emotional steadiness
2. help the user describe what happened
3. help the user identify what they felt
4. help the user describe how they reacted
5. help the user identify what they expected should have happened, or how they think they should have reacted

Behavior rules:
- Keep responses concise, clear, and supportive.
- Prefer one grounded reflection plus one useful follow-up question.
- Do not overwhelm the user with too many questions at once.
- Do not sound robotic, preachy, or overly formal.
- Do not give legal, medical, or financial advice.
- Do not invent facts that the user did not say.
- If details are unclear, ask for clarification instead of guessing.
- If the user already supplied one of the four reflection points, build on it instead of repeating the same question.

If the user expresses self-harm, suicide risk, or immediate danger:
- respond with warmth and urgency
- encourage immediate human help
- recommend contacting local emergency services if there is immediate danger
- in Brazil, mention CVV 188 as a crisis resource
- encourage contacting a trusted person right now
- do not continue the conversation as if it were a normal reflective turn

Your output must be plain assistant text only.`

type PrompterClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type promptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type generateRequest struct {
	Kind        string          `json:"kind"`
	Messages    []promptMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
}

type generateResponse struct {
	Kind       string `json:"kind"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	OutputText string `json:"output_text"`
}

func NewPrompterClient(baseURL, apiKey string, timeout time.Duration) *PrompterClient {
	return &PrompterClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *PrompterClient) Enabled() bool {
	return c.baseURL != ""
}

func (c *PrompterClient) GenerateReply(ctx context.Context, history []domain.Message) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("the prompter is disabled")
	}

	body, err := json.Marshal(generateRequest{
		Kind:        prompterKindSupportive,
		Messages:    buildPromptMessages(history),
		Temperature: 0.7,
	})
	if err != nil {
		return "", fmt.Errorf("marshal prompter request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build prompter request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", prompterUserAgent)
	if c.apiKey != "" {
		req.Header.Set("Authorization", c.apiKey)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("perform prompter request: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("Failed to close prompter response body: %s\n", err.Error())
		}
	}()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return "", fmt.Errorf("prompter returned status %d: %s", res.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var payload generateResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode prompter response: %w", err)
	}
	if strings.TrimSpace(payload.OutputText) == "" {
		return "", fmt.Errorf("prompter response did not include output_text")
	}

	return payload.OutputText, nil
}

func buildPromptMessages(history []domain.Message) []promptMessage {
	messages := make([]promptMessage, 0, len(history)+1)
	messages = append(messages, promptMessage{
		Role:    "system",
		Content: supportiveSystemPrompt,
	})

	for _, message := range history {
		messages = append(messages, promptMessage{
			Role:    mapMessageRole(message.Sender),
			Content: message.Content,
		})
	}

	return messages
}

func mapMessageRole(sender domain.Sender) string {
	if sender == domain.SenderAssistant {
		return "assistant"
	}

	return "user"
}
