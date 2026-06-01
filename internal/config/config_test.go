package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadReadsClassifierConfiguration(t *testing.T) {
	t.Setenv("PROMPTER_BASE_URL", "http://prompter:8020")
	t.Setenv("PROMPTER_API_KEY", "prompter-secret")
	t.Setenv("PROMPTER_TIMEOUT_SECONDS", "12")
	t.Setenv("CLASSIFIER_BASE_URL", "http://classifier:8010")
	t.Setenv("CLASSIFIER_API_KEY", "classifier-secret")
	t.Setenv("CLASSIFIER_TIMEOUT_SECONDS", "15")

	cfg := Load()

	require.Equal(t, "http://prompter:8020", cfg.PrompterBaseURL)
	require.Equal(t, "prompter-secret", cfg.PrompterAPIKey)
	require.Equal(t, 12*time.Second, cfg.PrompterTimeout)
	require.Equal(t, "http://classifier:8010", cfg.ClassifierBaseURL)
	require.Equal(t, "classifier-secret", cfg.ClassifierAPIKey)
	require.Equal(t, 15*time.Second, cfg.ClassifierTimeout)
}
