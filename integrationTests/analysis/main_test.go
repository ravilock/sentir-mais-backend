package analysis

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	integrationtests "github.com/ravilock/sentir-mais-backend/integrationTests"
)

var (
	redisServer      *miniredis.Miniredis
	prompterServer   *httptest.Server
	classifierServer *httptest.Server
)

func TestMain(m *testing.M) {
	integrationtests.RequireMongoAvailable()

	var err error
	redisServer, err = miniredis.Run()
	if err != nil {
		log.Fatalf("failed to start redis test server: %v", err)
	}
	if err := os.Setenv("REDIS_ADDR", redisServer.Addr()); err != nil {
		log.Fatalf("failed to set redis env: %v", err)
	}
	if err := os.Setenv("ANALYSIS_QUEUE_NAME", "analysis-integration-tests"); err != nil {
		log.Fatalf("failed to set queue env: %v", err)
	}
	if err := os.Setenv("INTEGRATION_TEST_DATABASE", "sentir_mais_analysis_integration_tests"); err != nil {
		log.Fatalf("failed to set integration database env: %v", err)
	}

	prompterServer = httptest.NewServer(http.HandlerFunc(handlePrompter))
	classifierServer = httptest.NewServer(http.HandlerFunc(handleClassifier))
	if err := os.Setenv("PROMPTER_BASE_URL", prompterServer.URL); err != nil {
		log.Fatalf("failed to set prompter env: %v", err)
	}
	if err := os.Setenv("CLASSIFIER_BASE_URL", classifierServer.URL); err != nil {
		log.Fatalf("failed to set classifier env: %v", err)
	}

	if err := integrationtests.Setup(); err != nil {
		log.Fatalf("failed to set up integration tests: %v", err)
	}

	code := m.Run()

	if err := integrationtests.Teardown(); err != nil {
		log.Printf("failed to tear down integration tests: %v", err)
	}
	prompterServer.Close()
	classifierServer.Close()
	redisServer.Close()

	os.Exit(code)
}

func handlePrompter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/generate" {
		http.NotFound(w, r)
		return
	}

	var request struct {
		Kind string `json:"kind"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	switch request.Kind {
	case "supportive":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kind":        "supportive",
			"provider":    "test-prompter",
			"model":       "test-support-model",
			"output_text": "Entendi. Vamos organizar isso com calma: o que voce esperava que acontecesse?",
		})
	case "extraction":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"kind":     "extraction",
			"provider": "test-prompter",
			"model":    "test-extraction-model",
			"output_text": `{
				"enough_context": true,
				"context_gaps": [],
				"event_summary": "Discussao com o gerente no trabalho.",
				"what_happened": "O usuario discutiu com o gerente sobre uma entrega.",
				"felt_emotions_described_by_user": ["ansioso", "tenso"],
				"user_reaction": "O usuario ficou quieto depois da discussao.",
				"expected_outcome_or_self_expectation": "O usuario esperava uma conversa mais respeitosa.",
				"people_involved": ["usuario", "gerente"],
				"setting": "trabalho",
				"time_reference": "hoje",
				"risk_flags": {
					"self_harm": false,
					"suicidal_ideation": false,
					"immediate_danger": false
				},
				"confidence_notes": "Contexto suficiente para teste de integracao."
			}`,
		})
	default:
		http.Error(w, "unknown kind", http.StatusBadRequest)
	}
}

func handleClassifier(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/classify" {
		http.NotFound(w, r)
		return
	}

	var request struct {
		Text       string `json:"text"`
		TopK       int    `json:"top_k"`
		MultiLabel bool   `json:"multi_label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Text == "" || request.TopK != 3 || !request.MultiLabel {
		http.Error(w, "invalid classify request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"primary_feeling": map[string]any{
			"label":      "anxious",
			"confidence": 0.87,
		},
		"secondary_feelings": []map[string]any{
			{"label": "tense", "confidence": 0.72},
			{"label": "stressed", "confidence": 0.64},
		},
		"all_scores": []map[string]any{
			{"label": "anxious", "confidence": 0.87},
			{"label": "tense", "confidence": 0.72},
			{"label": "stressed", "confidence": 0.64},
		},
		"model_name": "test-classifier-model",
	})
}
