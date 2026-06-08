package llm

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestPrompterClientGenerateReplySendsExpectedRequest(t *testing.T) {
	t.Parallel()

	var capturedBody string
	client := newTestPrompterClient()
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/generate", r.URL.Path)
		require.Equal(t, "test-key", r.Header.Get("Authorization"))
		require.Equal(t, prompterUserAgent, r.Header.Get("User-Agent"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		capturedBody = string(body)

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"kind":"supportive",
				"provider":"openrouter",
				"model":"google/gemini-2.5-flash-lite",
				"output_text":"assistant reply"
			}`)),
		}, nil
	})

	reply, err := client.GenerateReply(context.Background(), []domain.Message{
		{Sender: domain.SenderUser, Content: "I feel overwhelmed"},
		{Sender: domain.SenderAssistant, Content: "Tell me more"},
	})

	require.NoError(t, err)
	require.Equal(t, "assistant reply", reply)
	require.Contains(t, capturedBody, `"kind":"supportive"`)
	require.Contains(t, capturedBody, `"role":"system"`)
	require.Contains(t, capturedBody, `"role":"user"`)
	require.Contains(t, capturedBody, `"role":"assistant"`)
	require.Contains(t, capturedBody, `"content":"I feel overwhelmed"`)
}

func TestPrompterClientGenerateReplyReturnsErrorOnNon2xx(t *testing.T) {
	t.Parallel()

	client := newTestPrompterClient()
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader(`{"detail":"provider failed"}`)),
		}, nil
	})

	reply, err := client.GenerateReply(context.Background(), []domain.Message{
		{Sender: domain.SenderUser, Content: "hello"},
	})

	require.ErrorContains(t, err, "prompter returned status 502")
	require.Empty(t, reply)
}

func TestPrompterClientExtractEventParsesStructuredOutput(t *testing.T) {
	t.Parallel()

	var capturedBody string
	client := newTestPrompterClient()
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		capturedBody = string(body)

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"kind":"extraction",
				"provider":"openrouter",
				"model":"google/gemini-2.5-flash-lite",
				"output_text":"{\"enough_context\":true,\"context_gaps\":[],\"event_summary\":\"The user argued with their manager.\",\"what_happened\":\"The user argued with their manager at work.\",\"felt_emotions_described_by_user\":[\"anxious\"],\"user_reaction\":\"The user became defensive.\",\"expected_outcome_or_self_expectation\":\"The user expected more respect.\",\"people_involved\":[\"manager\"],\"setting\":\"work\",\"time_reference\":\"today\",\"risk_flags\":{\"self_harm\":false,\"suicidal_ideation\":false,\"immediate_danger\":false},\"confidence_notes\":\"Directly stated.\"}"
			}`)),
		}, nil
	})

	event, err := client.ExtractEvent(context.Background(), []domain.Message{
		{Sender: domain.SenderUser, Content: "I argued with my manager and felt anxious"},
	})

	require.NoError(t, err)
	require.Contains(t, capturedBody, `"kind":"extraction"`)
	require.Contains(t, capturedBody, `"response_format":{"type":"json_object"}`)
	require.Equal(t, "The user argued with their manager.", event.EventSummary)
	require.True(t, event.EnoughContext)
	require.Equal(t, []string{"anxious"}, event.FeltEmotionsDescribedByUser)
}

func TestPrompterClientExtractEventReturnsErrorOnInvalidJSONPayload(t *testing.T) {
	t.Parallel()

	client := newTestPrompterClient()
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"kind":"extraction",
				"provider":"openrouter",
				"model":"google/gemini-2.5-flash-lite",
				"output_text":"not-json"
			}`)),
		}, nil
	})

	event, err := client.ExtractEvent(context.Background(), []domain.Message{
		{Sender: domain.SenderUser, Content: "hello"},
	})

	require.ErrorContains(t, err, "decode extracted event payload")
	require.Equal(t, domain.ExtractedEvent{}, event)
}

func TestPrompterClientExtractEventRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	client := newTestPrompterClient()
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"kind":"extraction",
				"provider":"openrouter",
				"model":"google/gemini-2.5-flash-lite",
				"output_text":"{\"enough_context\":true,\"context_gaps\":[],\"event_summary\":\"Summary\",\"what_happened\":\"What happened\",\"felt_emotions_described_by_user\":[],\"user_reaction\":\"reaction\",\"expected_outcome_or_self_expectation\":\"expectation\",\"people_involved\":[],\"setting\":\"work\",\"time_reference\":\"today\",\"risk_flags\":{\"self_harm\":false,\"suicidal_ideation\":false,\"immediate_danger\":false},\"confidence_notes\":\"noted\",\"unexpected\":\"value\"}"
			}`)),
		}, nil
	})

	event, err := client.ExtractEvent(context.Background(), []domain.Message{{Sender: domain.SenderUser, Content: "hello"}})

	require.ErrorContains(t, err, "decode extracted event payload")
	require.ErrorContains(t, err, "unknown field")
	require.Equal(t, domain.ExtractedEvent{}, event)
}

func TestPrompterClientExtractEventRejectsInvalidContextGap(t *testing.T) {
	t.Parallel()

	client := newTestPrompterClient()
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"kind":"extraction",
				"provider":"openrouter",
				"model":"google/gemini-2.5-flash-lite",
				"output_text":"{\"enough_context\":false,\"context_gaps\":[\"unknown_gap\"],\"event_summary\":\"Summary\",\"what_happened\":\"What happened\",\"felt_emotions_described_by_user\":[],\"user_reaction\":\"reaction\",\"expected_outcome_or_self_expectation\":\"expectation\",\"people_involved\":[],\"setting\":\"work\",\"time_reference\":\"today\",\"risk_flags\":{\"self_harm\":false,\"suicidal_ideation\":false,\"immediate_danger\":false},\"confidence_notes\":\"noted\"}"
			}`)),
		}, nil
	})

	event, err := client.ExtractEvent(context.Background(), []domain.Message{{Sender: domain.SenderUser, Content: "hello"}})

	require.ErrorContains(t, err, "invalid context_gaps value")
	require.Equal(t, domain.ExtractedEvent{}, event)
}

func TestPrompterClientExtractEventRejectsInvalidFieldTypes(t *testing.T) {
	t.Parallel()

	client := newTestPrompterClient()
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"kind":"extraction",
				"provider":"openrouter",
				"model":"google/gemini-2.5-flash-lite",
				"output_text":"{\"enough_context\":true,\"context_gaps\":[],\"event_summary\":\"Summary\",\"what_happened\":\"What happened\",\"felt_emotions_described_by_user\":\"anxious\",\"user_reaction\":\"reaction\",\"expected_outcome_or_self_expectation\":\"expectation\",\"people_involved\":[],\"setting\":\"work\",\"time_reference\":\"today\",\"risk_flags\":{\"self_harm\":false,\"suicidal_ideation\":false,\"immediate_danger\":false},\"confidence_notes\":\"noted\"}"
			}`)),
		}, nil
	})

	event, err := client.ExtractEvent(context.Background(), []domain.Message{{Sender: domain.SenderUser, Content: "hello"}})

	require.ErrorContains(t, err, "decode extracted event payload")
	require.Equal(t, domain.ExtractedEvent{}, event)
}

func TestPrompterClientExtractEventAcceptsMissingRiskFlagsAsZeroValues(t *testing.T) {
	t.Parallel()

	client := newTestPrompterClient()
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"kind":"extraction",
				"provider":"openrouter",
				"model":"google/gemini-2.5-flash-lite",
				"output_text":"{\"enough_context\":true,\"context_gaps\":[],\"event_summary\":\"Summary\",\"what_happened\":\"What happened\",\"felt_emotions_described_by_user\":[],\"user_reaction\":\"reaction\",\"expected_outcome_or_self_expectation\":\"expectation\",\"people_involved\":[],\"setting\":\"work\",\"time_reference\":\"today\",\"confidence_notes\":\"noted\"}"
			}`)),
		}, nil
	})

	event, err := client.ExtractEvent(context.Background(), []domain.Message{{Sender: domain.SenderUser, Content: "hello"}})

	require.NoError(t, err)
	require.False(t, event.RiskFlags.SelfHarm)
	require.False(t, event.RiskFlags.SuicidalIdeation)
	require.False(t, event.RiskFlags.ImmediateDanger)
}

func TestPrompterClientExtractEventNormalizesNullableAndBlankFields(t *testing.T) {
	t.Parallel()

	client := newTestPrompterClient()
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
				"kind":"extraction",
				"provider":"openrouter",
				"model":"google/gemini-2.5-flash-lite",
				"output_text":"{\"enough_context\":false,\"context_gaps\":[\" what_happened \"],\"event_summary\":null,\"what_happened\":\"  \",\"felt_emotions_described_by_user\":[\" anxious \",\"\"],\"user_reaction\":null,\"expected_outcome_or_self_expectation\":\" expected calm \",\"people_involved\":[\" manager \",\"\"],\"setting\":null,\"time_reference\":\" today \",\"risk_flags\":{\"self_harm\":false,\"suicidal_ideation\":false,\"immediate_danger\":false},\"confidence_notes\":null}"
			}`)),
		}, nil
	})

	event, err := client.ExtractEvent(context.Background(), []domain.Message{{Sender: domain.SenderUser, Content: "hello"}})

	require.NoError(t, err)
	require.False(t, event.EnoughContext)
	require.Equal(t, []domain.ContextGap{domain.ContextGapWhatHappened}, event.ContextGaps)
	require.Equal(t, "", event.EventSummary)
	require.Equal(t, "", event.WhatHappened)
	require.Equal(t, []string{"anxious"}, event.FeltEmotionsDescribedByUser)
	require.Equal(t, "", event.UserReaction)
	require.Equal(t, "expected calm", event.ExpectedOutcomeOrSelfExpectation)
	require.Equal(t, []string{"manager"}, event.PeopleInvolved)
	require.Equal(t, "", event.Setting)
	require.Equal(t, "today", event.TimeReference)
	require.Equal(t, "", event.ConfidenceNotes)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func newTestPrompterClient() *PrompterClient {
	return NewPrompterClient(
		"http://prompter.test",
		"test-key",
		time.Second,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}
