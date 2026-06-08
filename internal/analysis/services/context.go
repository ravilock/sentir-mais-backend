package services

import "github.com/ravilock/sentir-mais-backend/internal/domain"

type analysisNextStep string

const (
	analysisNextStepContinueSupportive analysisNextStep = "continue_supportive"
	analysisNextStepProceedToNextStage analysisNextStep = "proceed_to_next_stage"
)

func determineNextAnalysisStep(analysis domain.MessageAnalysis) analysisNextStep {
	if analysis.EnoughContext == nil {
		return analysisNextStepContinueSupportive
	}
	if !*analysis.EnoughContext {
		return analysisNextStepContinueSupportive
	}

	return analysisNextStepProceedToNextStage
}
