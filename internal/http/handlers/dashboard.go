package handlers

import (
	"net/http"

	"github.com/ravilock/sentir-mais-backend/internal/http/dto"
	"github.com/ravilock/sentir-mais-backend/internal/http/middleware"
)

type DashboardHandler struct {
	service weeklySummaryGetter
}

func NewDashboardHandler(service weeklySummaryGetter) *DashboardHandler {
	return &DashboardHandler{service: service}
}

func (h *DashboardHandler) GetWeek(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	summary := h.service.GetWeek(r.Context(), user.ID)
	response := dto.WeeklySummaryResponse{
		WeekStart:        summary.WeekStart,
		DominantFeelings: make([]dto.FeelingScoreResponse, 0, len(summary.DominantFeelings)),
		MainEvents:       summary.MainEvents,
		TimelinePoints:   make([]dto.TimelinePointResponse, 0, len(summary.TimelinePoints)),
		GeneratedAt:      summary.GeneratedAt,
	}

	for _, feeling := range summary.DominantFeelings {
		response.DominantFeelings = append(response.DominantFeelings, dto.FeelingScoreResponse{
			Label:      feeling.Label,
			Confidence: feeling.Confidence,
		})
	}

	for _, point := range summary.TimelinePoints {
		response.TimelinePoints = append(response.TimelinePoints, dto.TimelinePointResponse{
			Date:            point.Date,
			PrimaryFeeling:  point.PrimaryFeeling,
			SupportingEvent: point.SupportingEvent,
		})
	}

	respondJSON(w, http.StatusOK, response)
}
