package handlers

import (
	"log/slog"
	"net/http"

	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	"github.com/ravilock/sentir-mais-backend/internal/http/middleware"
)

type DashboardHandler struct {
	logger  *slog.Logger
	service weeklySummaryGetter
}

func NewDashboardHandler(logger *slog.Logger, service weeklySummaryGetter) *DashboardHandler {
	return &DashboardHandler{logger: logger, service: service}
}

func (h *DashboardHandler) GetWeek(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		logRequestError(h.logger, r, http.StatusUnauthorized, "missing authenticated user in dashboard week request", nil)
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	summary := h.service.GetWeek(r.Context(), user.ID)
	response := apiresponses.WeeklySummaryResponse{
		WeekStart:        summary.WeekStart,
		DominantFeelings: make([]apiresponses.FeelingScoreResponse, 0, len(summary.DominantFeelings)),
		MainEvents:       summary.MainEvents,
		TimelinePoints:   make([]apiresponses.TimelinePointResponse, 0, len(summary.TimelinePoints)),
		GeneratedAt:      summary.GeneratedAt,
	}

	for _, feeling := range summary.DominantFeelings {
		response.DominantFeelings = append(response.DominantFeelings, apiresponses.FeelingScoreResponse{
			Label:      feeling.Label,
			Confidence: feeling.Confidence,
		})
	}

	for _, point := range summary.TimelinePoints {
		response.TimelinePoints = append(response.TimelinePoints, apiresponses.TimelinePointResponse{
			Date:            point.Date,
			PrimaryFeeling:  point.PrimaryFeeling,
			SupportingEvent: point.SupportingEvent,
		})
	}

	respondJSON(w, http.StatusOK, response)
}
