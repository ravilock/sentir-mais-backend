package llm

import (
	"context"
	"io"
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
	client := NewPrompterClient("http://prompter.test", "test-key", time.Second)
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

	client := NewPrompterClient("http://prompter.test", "test-key", time.Second)
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
