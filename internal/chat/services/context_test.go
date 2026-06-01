package services

import (
	"testing"

	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestDetermineNextAnalysisStep(t *testing.T) {
	t.Run("should continue supportive conversation when context decision is unknown", func(t *testing.T) {
		step := determineNextAnalysisStep(domain.MessageAnalysis{})

		require.Equal(t, analysisNextStepContinueSupportive, step)
	})

	t.Run("should continue supportive conversation when enough context is false", func(t *testing.T) {
		enoughContext := false

		step := determineNextAnalysisStep(domain.MessageAnalysis{
			EnoughContext: &enoughContext,
			ContextGaps: []domain.ContextGap{
				domain.ContextGapWhatHappened,
				domain.ContextGapExpectedOutcomeOrExpectation,
			},
		})

		require.Equal(t, analysisNextStepContinueSupportive, step)
	})

	t.Run("should proceed when enough context is true", func(t *testing.T) {
		enoughContext := true

		step := determineNextAnalysisStep(domain.MessageAnalysis{
			EnoughContext: &enoughContext,
		})

		require.Equal(t, analysisNextStepProceedToNextStage, step)
	})
}
