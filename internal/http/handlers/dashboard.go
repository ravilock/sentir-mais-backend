package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	"github.com/ravilock/sentir-mais-backend/internal/http/middleware"
)

type DashboardHandler struct {
	logger          *slog.Logger
	weekService     weeklySummaryGetter
	timelineService timelineSummaryGetter
}

func NewDashboardHandler(logger *slog.Logger, weekService weeklySummaryGetter, timelineService timelineSummaryGetter) *DashboardHandler {
	return &DashboardHandler{
		logger:          logger,
		weekService:     weekService,
		timelineService: timelineService,
	}
}

func (h *DashboardHandler) GetWeek(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		logRequestError(h.logger, r, http.StatusUnauthorized, "missing authenticated user in dashboard week request", nil)
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	summary, err := h.weekService.GetWeek(r.Context(), user.ID)
	if err != nil {
		logRequestError(h.logger, r, http.StatusInternalServerError, "get dashboard week request failed", err)
		respondError(w, http.StatusInternalServerError, "failed to load weekly dashboard")
		return
	}

	respondJSON(w, http.StatusOK, toWeeklySummaryResponse(summary))
}

func (h *DashboardHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		logRequestError(h.logger, r, http.StatusUnauthorized, "missing authenticated user in dashboard timeline request", nil)
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	from, to, err := parseTimelineRangeQuery(r)
	if err != nil {
		logRequestError(h.logger, r, http.StatusUnprocessableEntity, "invalid dashboard timeline query", err)
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	timeline, err := h.timelineService.GetTimeline(r.Context(), user.ID, from, to)
	if err != nil {
		logRequestError(h.logger, r, http.StatusInternalServerError, "get dashboard timeline request failed", err)
		respondError(w, http.StatusInternalServerError, "failed to load dashboard timeline")
		return
	}

	response := apiresponses.DashboardTimelineResponse{
		From: timeline.From.UTC().Format(time.DateOnly),
		To:   timeline.To.UTC().Format(time.DateOnly),
		Days: make([]apiresponses.DailySummaryResponse, 0, len(timeline.Days)),
	}
	for _, day := range timeline.Days {
		response.Days = append(response.Days, toDailySummaryResponse(day))
	}

	respondJSON(w, http.StatusOK, response)
}

func parseTimelineRangeQuery(r *http.Request) (*time.Time, *time.Time, error) {
	fromValue := strings.TrimSpace(r.URL.Query().Get("from"))
	toValue := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromValue == "" && toValue == "" {
		return nil, nil, nil
	}
	if fromValue == "" || toValue == "" {
		return nil, nil, errors.New("query params 'from' and 'to' must be provided together")
	}

	from, err := time.ParseInLocation(time.DateOnly, fromValue, time.UTC)
	if err != nil {
		return nil, nil, errors.New("query params 'from' and 'to' must use YYYY-MM-DD")
	}
	to, err := time.ParseInLocation(time.DateOnly, toValue, time.UTC)
	if err != nil {
		return nil, nil, errors.New("query params 'from' and 'to' must use YYYY-MM-DD")
	}
	if to.Before(from) {
		return nil, nil, errors.New("query param 'to' must be on or after 'from'")
	}
	if int(to.Sub(from).Hours()/24) > 29 {
		return nil, nil, errors.New("dashboard timeline range must not exceed 30 days")
	}

	return &from, &to, nil
}

func toWeeklySummaryResponse(summary domain.WeeklySummary) apiresponses.WeeklySummaryResponse {
	return apiresponses.WeeklySummaryResponse{
		WeekStart:        summary.WeekStart,
		DominantFeelings: toFeelingScoreResponses(summary.DominantFeelings),
		MainEvents:       summary.MainEvents,
		TimelinePoints:   toTimelinePointResponses(summary.TimelinePoints),
		GeneratedAt:      summary.GeneratedAt,
	}
}

func toDailySummaryResponse(summary domain.DailySummary) apiresponses.DailySummaryResponse {
	return apiresponses.DailySummaryResponse{
		DayStart:         summary.DayStart,
		DominantFeelings: toFeelingScoreResponses(summary.DominantFeelings),
		MainEvents:       summary.MainEvents,
		TimelinePoints:   toTimelinePointResponses(summary.TimelinePoints),
		GeneratedAt:      summary.GeneratedAt,
	}
}

func toFeelingScoreResponses(feelings []domain.FeelingScore) []apiresponses.FeelingScoreResponse {
	response := make([]apiresponses.FeelingScoreResponse, 0, len(feelings))
	for _, feeling := range feelings {
		response = append(response, apiresponses.FeelingScoreResponse{
			Label:      feeling.Label,
			Confidence: feeling.Confidence,
		})
	}

	return response
}

func toTimelinePointResponses(points []domain.TimelinePoint) []apiresponses.TimelinePointResponse {
	response := make([]apiresponses.TimelinePointResponse, 0, len(points))
	for _, point := range points {
		response = append(response, apiresponses.TimelinePointResponse{
			Date:            point.Date,
			PrimaryFeeling:  point.PrimaryFeeling,
			SupportingEvent: point.SupportingEvent,
		})
	}

	return response
}
