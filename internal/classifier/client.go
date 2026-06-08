package classifier

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
	ProviderName = "sentir-mais-classifier"
	UserAgent    = "sentir-mais-backend"
)

const defaultTopK = 3

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

type classifyRequest struct {
	Text       string `json:"text"`
	TopK       int    `json:"top_k"`
	MultiLabel bool   `json:"multi_label"`
}

type feelingScoreResponse struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

type classifyResponse struct {
	PrimaryFeeling    feelingScoreResponse   `json:"primary_feeling"`
	SecondaryFeelings []feelingScoreResponse `json:"secondary_feelings"`
	AllScores         []feelingScoreResponse `json:"all_scores"`
	ModelName         string                 `json:"model_name"`
}

func NewClient(baseURL, apiKey string, timeout time.Duration, logger *slog.Logger) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger.With("component", ProviderName),
	}
}

func (c *Client) Enabled() bool {
	return c.baseURL != ""
}

func (c *Client) Classify(ctx context.Context, text string) (domain.ClassificationResult, error) {
	if !c.Enabled() {
		return domain.ClassificationResult{}, fmt.Errorf("the classifier is disabled")
	}

	requestSummary := map[string]any{
		"base_url":     c.baseURL,
		"text_length":  len(text),
		"top_k":        defaultTopK,
		"multi_label":  true,
		"text_preview": previewText(text, 200),
	}
	startedAt := time.Now()
	c.logger.DebugContext(ctx, "dispatching classifier request", "summary", requestSummary)

	requestBody, err := json.Marshal(classifyRequest{
		Text:       text,
		TopK:       defaultTopK,
		MultiLabel: true,
	})
	if err != nil {
		return domain.ClassificationResult{}, fmt.Errorf("marshal classify request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/classify", bytes.NewReader(requestBody))
	if err != nil {
		return domain.ClassificationResult{}, fmt.Errorf("build classify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	if c.apiKey != "" {
		req.Header.Set("Authorization", c.apiKey)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.ErrorContext(ctx, "classifier request failed",
			"summary", requestSummary,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return domain.ClassificationResult{}, fmt.Errorf("perform classify request: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			c.logger.WarnContext(ctx, "failed to close classifier response body", "error", err)
		}
	}()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		c.logger.ErrorContext(ctx, "classifier returned non-success status",
			"summary", requestSummary,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"status_code", res.StatusCode,
			"response_preview", strings.TrimSpace(string(snippet)),
		)
		return domain.ClassificationResult{}, fmt.Errorf("classifier returned status %d: %s", res.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var payload classifyResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		c.logger.ErrorContext(ctx, "classifier returned invalid response",
			"summary", requestSummary,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		return domain.ClassificationResult{}, fmt.Errorf("decode classify response: %w", err)
	}

	c.logger.InfoContext(ctx, "classifier response received",
		"summary", requestSummary,
		"duration_ms", time.Since(startedAt).Milliseconds(),
		"model", payload.ModelName,
		"primary_feeling", payload.PrimaryFeeling.Label,
		"score_count", len(payload.AllScores),
	)

	return domain.ClassificationResult{
		PrimaryFeeling:    toFeelingScore(payload.PrimaryFeeling),
		SecondaryFeelings: toFeelingScores(payload.SecondaryFeelings),
		AllScores:         toFeelingScores(payload.AllScores),
		ModelName:         payload.ModelName,
	}, nil
}

func previewText(value string, limit int) string {
	normalized := strings.Join(strings.Fields(value), " ")
	if len(normalized) <= limit {
		return normalized
	}

	return normalized[:limit] + "..."
}

func toFeelingScore(score feelingScoreResponse) domain.FeelingScore {
	return domain.FeelingScore{
		Label:      score.Label,
		Confidence: score.Confidence,
	}
}

func toFeelingScores(scores []feelingScoreResponse) []domain.FeelingScore {
	converted := make([]domain.FeelingScore, 0, len(scores))
	for _, score := range scores {
		converted = append(converted, toFeelingScore(score))
	}

	return converted
}
