package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
Do not include explanatory prose.

Return exactly one top-level JSON object with these fields and no others:
{
  "enough_context": true,
  "context_gaps": [],
  "event_summary": "string or null",
  "what_happened": "string or null",
  "felt_emotions_described_by_user": ["string"],
  "user_reaction": "string or null",
  "expected_outcome_or_self_expectation": "string or null",
  "people_involved": ["string"],
  "setting": "string or null",
  "time_reference": "string or null",
  "risk_flags": {
    "self_harm": false,
    "suicidal_ideation": false,
    "immediate_danger": false
  },
  "confidence_notes": "string or null"
}

Rules:
- Do not wrap the result in an "event" field or any other wrapper object.
- Use these exact top-level field names.
- Use these exact risk_flags field names.
- "context_gaps" must contain only these enum values:
  - "what_happened"
  - "felt_emotions_described_by_user"
  - "user_reaction"
  - "expected_outcome_or_self_expectation"
- If enough_context is true, context_gaps must be [].
- If enough_context is false, context_gaps must list the missing reflection fields using only the allowed enum values.
- "felt_emotions_described_by_user" must always be an array of strings, never null.
- "people_involved" must always be an array of strings, never null.
- Use null only for nullable string fields when the conversation does not establish them.
- Do not output any fields that are not listed above.`

type PrompterClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
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

func NewPrompterClient(baseURL, apiKey string, timeout time.Duration, logger *slog.Logger) *PrompterClient {
	return &PrompterClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger.With("component", "PrompterClient"),
	}
}

func (c *PrompterClient) Enabled() bool {
	return c.baseURL != ""
}

func (c *PrompterClient) GenerateReply(ctx context.Context, history []domain.Message) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("the prompter is disabled")
	}

	requestSummary := c.buildPrompterRequestSummary(prompterKindSupportive, history, 0.3, 0, nil)
	body, err := json.Marshal(generateRequest{
		Kind:        prompterKindSupportive,
		Messages:    buildPromptMessages(history),
		Temperature: 0.3,
	})
	if err != nil {
		return "", fmt.Errorf("marshal prompter request: %w", err)
	}

	payload, err := c.doGenerate(ctx, body, requestSummary)
	if err != nil {
		return "", err
	}

	c.logger.InfoContext(ctx, "prompter reply received",
		"kind", payload.Kind,
		"provider", payload.Provider,
		"model", payload.Model,
		"output_length", len(payload.OutputText),
	)
	return payload.OutputText, nil
}

func (c *PrompterClient) ExtractEvent(ctx context.Context, history []domain.Message) (domain.ExtractedEvent, error) {
	if !c.Enabled() {
		return domain.ExtractedEvent{}, fmt.Errorf("the prompter is disabled")
	}

	requestSummary := c.buildPrompterRequestSummary(prompterKindExtraction, history, 0.1, 1200, map[string]string{
		"type": "json_object",
	})
	body, err := json.Marshal(generateRequest{
		Kind:        prompterKindExtraction,
		Messages:    buildExtractionPromptMessages(history),
		Temperature: 0.1,
		MaxTokens:   1200,
		ResponseFormat: &generateResponseFormat{
			Type: "json_object",
		},
	})
	if err != nil {
		return domain.ExtractedEvent{}, fmt.Errorf("marshal extraction request: %w", err)
	}

	payload, err := c.doGenerate(ctx, body, requestSummary)
	if err != nil {
		return domain.ExtractedEvent{}, err
	}

	c.logger.DebugContext(ctx, "prompter extraction response received",
		"provider", payload.Provider,
		"model", payload.Model,
		"output_preview", previewText(payload.OutputText, 300),
	)
	extracted, err := decodeExtractedEventPayload(payload.OutputText)
	if err != nil {
		c.logger.ErrorContext(ctx, "prompter extraction payload decode failed",
			"provider", payload.Provider,
			"model", payload.Model,
			"output_preview", previewText(payload.OutputText, 300),
			"error", err,
		)
		return domain.ExtractedEvent{}, fmt.Errorf("decode extracted event payload: %w", err)
	}

	return extracted.toDomain(), nil
}

func (c *PrompterClient) doGenerate(ctx context.Context, body []byte, requestSummary map[string]any) (generateResponse, error) {
	startedAt := time.Now()
	c.logger.DebugContext(ctx, "dispatching prompter request", "summary", requestSummary)

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
		c.logger.ErrorContext(ctx, "prompter request failed",
			"summary", requestSummary,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return generateResponse{}, fmt.Errorf("perform prompter request: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			c.logger.WarnContext(ctx, "failed to close prompter response body", "error", err)
		}
	}()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		c.logger.ErrorContext(ctx, "prompter returned non-success status",
			"summary", requestSummary,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"status_code", res.StatusCode,
			"response_preview", strings.TrimSpace(string(snippet)),
		)
		return generateResponse{}, fmt.Errorf("prompter returned status %d: %s", res.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var payload generateResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		c.logger.ErrorContext(ctx, "prompter returned invalid response",
			"summary", requestSummary,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return generateResponse{}, fmt.Errorf("decode prompter response: %w", err)
	}
	if strings.TrimSpace(payload.OutputText) == "" {
		return generateResponse{}, fmt.Errorf("prompter response did not include output_text")
	}

	c.logger.InfoContext(ctx, "prompter response received",
		"summary", requestSummary,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"provider", payload.Provider,
		"model", payload.Model,
		"kind", payload.Kind,
		"output_length", len(payload.OutputText),
	)

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

func previewText(value string, limit int) string {
	normalized := strings.Join(strings.Fields(value), " ")
	if len(normalized) <= limit {
		return normalized
	}

	return normalized[:limit] + "..."
}

func (c *PrompterClient) buildPrompterRequestSummary(kind string, history []domain.Message, temperature float64, maxTokens int, responseFormat map[string]string) map[string]any {
	messageRoles := make([]string, 0, len(history))
	messageLengths := make([]int, 0, len(history))
	for _, message := range history {
		messageRoles = append(messageRoles, mapMessageRole(message.Sender))
		messageLengths = append(messageLengths, len(message.Content))
	}

	summary := map[string]any{
		"kind":            kind,
		"base_url":        c.baseURL,
		"history_count":   len(history),
		"message_roles":   messageRoles,
		"message_lengths": messageLengths,
		"temperature":     temperature,
	}
	if maxTokens > 0 {
		summary["max_tokens"] = maxTokens
	}
	if responseFormat != nil {
		summary["response_format"] = responseFormat
	}

	return summary
}

type extractedEventPayload struct {
	EnoughContext                    bool               `json:"enough_context"`
	ContextGaps                      []string           `json:"context_gaps"`
	EventSummary                     string             `json:"event_summary"`
	WhatHappened                     string             `json:"what_happened"`
	FeltEmotionsDescribedByUser      []string           `json:"felt_emotions_described_by_user"`
	UserReaction                     string             `json:"user_reaction"`
	ExpectedOutcomeOrSelfExpectation string             `json:"expected_outcome_or_self_expectation"`
	PeopleInvolved                   []string           `json:"people_involved"`
	Setting                          string             `json:"setting"`
	TimeReference                    string             `json:"time_reference"`
	RiskFlags                        extractedRiskFlags `json:"risk_flags"`
	ConfidenceNotes                  string             `json:"confidence_notes"`
}

type extractedRiskFlags struct {
	SelfHarm         bool `json:"self_harm"`
	SuicidalIdeation bool `json:"suicidal_ideation"`
	ImmediateDanger  bool `json:"immediate_danger"`
}

func decodeExtractedEventPayload(raw string) (extractedEventPayload, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()

	var payload extractedEventPayload
	if err := decoder.Decode(&payload); err != nil {
		return extractedEventPayload{}, err
	}

	if decoder.More() {
		return extractedEventPayload{}, fmt.Errorf("unexpected trailing JSON content")
	}

	if err := payload.validate(); err != nil {
		return extractedEventPayload{}, err
	}

	return payload.normalize(), nil
}

func (payload extractedEventPayload) validate() error {
	for _, gap := range payload.ContextGaps {
		switch domain.ContextGap(strings.TrimSpace(gap)) {
		case domain.ContextGapWhatHappened,
			domain.ContextGapFeltEmotionsDescribedByUser,
			domain.ContextGapUserReaction,
			domain.ContextGapExpectedOutcomeOrExpectation:
		default:
			return fmt.Errorf("invalid context_gaps value %q", gap)
		}
	}

	return nil
}

func (payload extractedEventPayload) normalize() extractedEventPayload {
	payload.EventSummary = strings.TrimSpace(payload.EventSummary)
	payload.WhatHappened = strings.TrimSpace(payload.WhatHappened)
	payload.UserReaction = strings.TrimSpace(payload.UserReaction)
	payload.ExpectedOutcomeOrSelfExpectation = strings.TrimSpace(payload.ExpectedOutcomeOrSelfExpectation)
	payload.Setting = strings.TrimSpace(payload.Setting)
	payload.TimeReference = strings.TrimSpace(payload.TimeReference)
	payload.ConfidenceNotes = strings.TrimSpace(payload.ConfidenceNotes)
	payload.ContextGaps = compactStrings(payload.ContextGaps)
	payload.FeltEmotionsDescribedByUser = compactStrings(payload.FeltEmotionsDescribedByUser)
	payload.PeopleInvolved = compactStrings(payload.PeopleInvolved)
	return payload
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

func compactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		compacted = append(compacted, trimmed)
	}

	return compacted
}
