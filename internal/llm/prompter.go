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
	prompterKindExtraction = "extraction"
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

const extractionSystemPrompt = `You are an information extraction component for Sentir Mais.

Your task is to read a conversation and extract a structured representation of the user's situation.

You must extract only information grounded in the conversation. Do not invent missing facts. If a field is unclear, uncertain, or not established, return null for that field.

You are not a classifier. Do not predict final emotion labels. Only capture emotional words or descriptions when they were explicitly stated by the user or are directly quoted/paraphrased from the conversation as evidence.

The extraction should focus on:
- what happened
- what the user said they felt
- how the user reacted
- what the user expected should have happened, or how they think they should have reacted

You must also assess whether these four reflection points are sufficiently established in the conversation.

Return valid JSON only, matching the required schema exactly.
Do not include markdown.
Do not include explanatory prose.`

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
	Kind           string                  `json:"kind"`
	Messages       []promptMessage         `json:"messages"`
	Temperature    float64                 `json:"temperature"`
	MaxTokens      int                     `json:"max_tokens,omitempty"`
	ResponseFormat *generateResponseFormat `json:"response_format,omitempty"`
}

type generateResponse struct {
	Kind       string `json:"kind"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	OutputText string `json:"output_text"`
}

type generateResponseFormat struct {
	Type string `json:"type"`
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

func (c *PrompterClient) ExtractEvent(ctx context.Context, history []domain.Message) (domain.ExtractedEvent, error) {
	if !c.Enabled() {
		return domain.ExtractedEvent{}, fmt.Errorf("the prompter is disabled")
	}

	body, err := json.Marshal(generateRequest{
		Kind:        prompterKindExtraction,
		Messages:    buildExtractionPromptMessages(history),
		Temperature: 0.2,
		MaxTokens:   1200,
		ResponseFormat: &generateResponseFormat{
			Type: "json_object",
		},
	})
	if err != nil {
		return domain.ExtractedEvent{}, fmt.Errorf("marshal extraction request: %w", err)
	}

	payload, err := c.doGenerate(ctx, body)
	if err != nil {
		return domain.ExtractedEvent{}, err
	}

	var extracted extractedEventPayload
	if err := json.Unmarshal([]byte(payload.OutputText), &extracted); err != nil {
		return domain.ExtractedEvent{}, fmt.Errorf("decode extracted event payload: %w", err)
	}

	return extracted.toDomain(), nil
}

func (c *PrompterClient) doGenerate(ctx context.Context, body []byte) (generateResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/generate", bytes.NewReader(body))
	if err != nil {
		return generateResponse{}, fmt.Errorf("build prompter request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", prompterUserAgent)
	if c.apiKey != "" {
		req.Header.Set("Authorization", c.apiKey)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return generateResponse{}, fmt.Errorf("perform prompter request: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("Failed to close prompter response body: %s\n", err.Error())
		}
	}()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return generateResponse{}, fmt.Errorf("prompter returned status %d: %s", res.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var payload generateResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return generateResponse{}, fmt.Errorf("decode prompter response: %w", err)
	}
	if strings.TrimSpace(payload.OutputText) == "" {
		return generateResponse{}, fmt.Errorf("prompter response did not include output_text")
	}

	return payload, nil
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

func buildExtractionPromptMessages(history []domain.Message) []promptMessage {
	return []promptMessage{
		{
			Role:    "system",
			Content: extractionSystemPrompt,
		},
		{
			Role: "user",
			Content: fmt.Sprintf(
				"Extract the structured event/context from the conversation below.\n\nConversation:\n%s",
				buildConversationTranscript(history),
			),
		},
	}
}

func buildConversationTranscript(history []domain.Message) string {
	lines := make([]string, 0, len(history))
	for _, message := range history {
		lines = append(lines, fmt.Sprintf("%s: %s", strings.ToUpper(mapMessageRole(message.Sender)), message.Content))
	}

	return strings.Join(lines, "\n")
}

func mapMessageRole(sender domain.Sender) string {
	if sender == domain.SenderAssistant {
		return "assistant"
	}

	return "user"
}

type extractedEventPayload struct {
	EnoughContext                    bool     `json:"enough_context"`
	ContextGaps                      []string `json:"context_gaps"`
	EventSummary                     string   `json:"event_summary"`
	WhatHappened                     string   `json:"what_happened"`
	FeltEmotionsDescribedByUser      []string `json:"felt_emotions_described_by_user"`
	UserReaction                     string   `json:"user_reaction"`
	ExpectedOutcomeOrSelfExpectation string   `json:"expected_outcome_or_self_expectation"`
	PeopleInvolved                   []string `json:"people_involved"`
	Setting                          string   `json:"setting"`
	TimeReference                    string   `json:"time_reference"`
	RiskFlags                        struct {
		SelfHarm         bool `json:"self_harm"`
		SuicidalIdeation bool `json:"suicidal_ideation"`
		ImmediateDanger  bool `json:"immediate_danger"`
	} `json:"risk_flags"`
	ConfidenceNotes string `json:"confidence_notes"`
}

func (payload extractedEventPayload) toDomain() domain.ExtractedEvent {
	return domain.ExtractedEvent{
		EnoughContext:                    payload.EnoughContext,
		ContextGaps:                      toContextGaps(payload.ContextGaps),
		EventSummary:                     payload.EventSummary,
		WhatHappened:                     payload.WhatHappened,
		FeltEmotionsDescribedByUser:      payload.FeltEmotionsDescribedByUser,
		UserReaction:                     payload.UserReaction,
		ExpectedOutcomeOrSelfExpectation: payload.ExpectedOutcomeOrSelfExpectation,
		PeopleInvolved:                   payload.PeopleInvolved,
		Setting:                          payload.Setting,
		TimeReference:                    payload.TimeReference,
		RiskFlags: domain.RiskFlags{
			SelfHarm:         payload.RiskFlags.SelfHarm,
			SuicidalIdeation: payload.RiskFlags.SuicidalIdeation,
			ImmediateDanger:  payload.RiskFlags.ImmediateDanger,
		},
		ConfidenceNotes: payload.ConfidenceNotes,
	}
}

func toContextGaps(values []string) []domain.ContextGap {
	gaps := make([]domain.ContextGap, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}

		gaps = append(gaps, domain.ContextGap(value))
	}

	return gaps
}
