package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadReadsClassifierConfiguration(t *testing.T) {
	t.Setenv("CLASSIFIER_BASE_URL", "http://classifier:8010")
	t.Setenv("CLASSIFIER_API_KEY", "classifier-secret")
	t.Setenv("CLASSIFIER_TIMEOUT_SECONDS", "15")

	cfg := Load()

	require.Equal(t, "http://classifier:8010", cfg.ClassifierBaseURL)
	require.Equal(t, "classifier-secret", cfg.ClassifierAPIKey)
	require.Equal(t, 15*time.Second, cfg.ClassifierTimeout)
}
