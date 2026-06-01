package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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

func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Enabled() bool {
	return c.baseURL != ""
}

func (c *Client) Classify(ctx context.Context, text string) (domain.ClassificationResult, error) {
	if !c.Enabled() {
		return domain.ClassificationResult{}, fmt.Errorf("the classifier is disabled")
	}

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
		return domain.ClassificationResult{}, fmt.Errorf("perform classify request: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("Failed to close %s response body: %s\n", ProviderName, err.Error())
		}
	}()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return domain.ClassificationResult{}, fmt.Errorf("classifier returned status %d", res.StatusCode)
	}

	var payload classifyResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return domain.ClassificationResult{}, fmt.Errorf("decode classify response: %w", err)
	}

	return domain.ClassificationResult{
		PrimaryFeeling:    toFeelingScore(payload.PrimaryFeeling),
		SecondaryFeelings: toFeelingScores(payload.SecondaryFeelings),
		AllScores:         toFeelingScores(payload.AllScores),
		ModelName:         payload.ModelName,
	}, nil
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
