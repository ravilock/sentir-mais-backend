package classifier

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientClassifySendsAPIKey(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/classify", r.URL.Path)
		require.Equal(t, "test-api-key", r.Header.Get("Authorization"))
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, UserAgent, r.Header.Get("User-Agent"))

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), `"text":"hello world"`)
		require.Contains(t, string(body), `"top_k":3`)
		require.Contains(t, string(body), `"multi_label":true`)

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{
			"primary_feeling":{"label":"happy","confidence":0.9},
			"secondary_feelings":[{"label":"relaxed","confidence":0.4}],
			"all_scores":[
				{"label":"happy","confidence":0.9},
				{"label":"relaxed","confidence":0.4}
			],
			"model_name":"test-model"
		}`)),
		}, nil
	})

	client := NewClient("http://classifier.test", "test-api-key", time.Second)
	client.httpClient.Transport = transport

	result, err := client.Classify(context.Background(), "hello world")

	require.NoError(t, err)
	require.Equal(t, "happy", result.PrimaryFeeling.Label)
	require.Equal(t, 0.9, result.PrimaryFeeling.Confidence)
	require.Equal(t, "test-model", result.ModelName)
	require.Len(t, result.SecondaryFeelings, 1)
	require.Len(t, result.AllScores, 2)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
