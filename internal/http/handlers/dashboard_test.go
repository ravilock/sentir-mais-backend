package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiresponses "github.com/ravilock/sentir-mais-backend/internal/api/responses"
	"github.com/ravilock/sentir-mais-backend/internal/domain"
	httpmiddleware "github.com/ravilock/sentir-mais-backend/internal/http/middleware"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDashboardHandlerGetWeek(t *testing.T) {
	t.Run("should return weekly summary response", func(t *testing.T) {
		getter := newMockWeeklySummaryGetter(t)
		handler := NewDashboardHandler(newTestHTTPLogger(), getter, newMockTimelineSummaryGetter(t))
		now := time.Date(2026, time.June, 8, 16, 30, 0, 0, time.UTC)
		weekStart := time.Date(2026, time.June, 8, 0, 0, 0, 0, time.UTC)

		req := httptest.NewRequest(http.MethodGet, "/dashboard/week", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		getter.EXPECT().
			GetWeek(mock.Anything, "usr_123").
			Return(domain.WeeklySummary{
				UserID:           "usr_123",
				WeekStart:        weekStart,
				DominantFeelings: []domain.FeelingScore{{Label: "sad", Confidence: 0.9}},
				MainEvents:       []string{"Bad meeting"},
				TimelinePoints:   []domain.TimelinePoint{{Date: "2026-06-08", PrimaryFeeling: "sad", SupportingEvent: "Bad meeting"}},
				GeneratedAt:      now,
			}, nil).
			Once()

		protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.GetWeek))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var payload apiresponses.WeeklySummaryResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		require.Equal(t, weekStart, payload.WeekStart)
		require.Equal(t, []apiresponses.FeelingScoreResponse{{Label: "sad", Confidence: 0.9}}, payload.DominantFeelings)
		require.Equal(t, []string{"Bad meeting"}, payload.MainEvents)
		require.Equal(t, []apiresponses.TimelinePointResponse{{Date: "2026-06-08", PrimaryFeeling: "sad", SupportingEvent: "Bad meeting"}}, payload.TimelinePoints)
		require.Equal(t, now, payload.GeneratedAt)
	})

	t.Run("should reject unauthenticated requests", func(t *testing.T) {
		handler := NewDashboardHandler(newTestHTTPLogger(), newMockWeeklySummaryGetter(t), newMockTimelineSummaryGetter(t))

		req := httptest.NewRequest(http.MethodGet, "/dashboard/week", nil)
		rec := httptest.NewRecorder()

		handler.GetWeek(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.JSONEq(t, `{"message":"unauthorized"}`, rec.Body.String())
	})

	t.Run("should return internal server error when summary lookup fails", func(t *testing.T) {
		getter := newMockWeeklySummaryGetter(t)
		handler := NewDashboardHandler(newTestHTTPLogger(), getter, newMockTimelineSummaryGetter(t))

		req := httptest.NewRequest(http.MethodGet, "/dashboard/week", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		getter.EXPECT().
			GetWeek(mock.Anything, "usr_123").
			Return(domain.WeeklySummary{}, errors.New("database unavailable")).
			Once()

		protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.GetWeek))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.JSONEq(t, `{"message":"failed to load weekly dashboard"}`, rec.Body.String())
	})
}

func TestDashboardHandlerGetTimeline(t *testing.T) {
	t.Run("should return timeline response for explicit range", func(t *testing.T) {
		getter := newMockTimelineSummaryGetter(t)
		handler := NewDashboardHandler(newTestHTTPLogger(), newMockWeeklySummaryGetter(t), getter)

		req := httptest.NewRequest(http.MethodGet, "/dashboard/timeline?from=2026-06-01&to=2026-06-07", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		from := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, time.June, 7, 0, 0, 0, 0, time.UTC)
		generatedAt := time.Date(2026, time.June, 7, 19, 0, 0, 0, time.UTC)

		getter.EXPECT().
			GetTimeline(mock.Anything, "usr_123", &from, &to).
			Return(domain.DashboardTimeline{
				From: from,
				To:   to,
				Days: []domain.DailySummary{{
					UserID:           "usr_123",
					DayStart:         to,
					DominantFeelings: []domain.FeelingScore{{Label: "sad", Confidence: 0.8}},
					MainEvents:       []string{"Bad meeting"},
					TimelinePoints:   []domain.TimelinePoint{{Date: "2026-06-07", PrimaryFeeling: "sad", SupportingEvent: "Bad meeting"}},
					GeneratedAt:      generatedAt,
				}},
			}, nil).
			Once()

		protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.GetTimeline))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		var payload apiresponses.DashboardTimelineResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		require.Equal(t, "2026-06-01", payload.From)
		require.Equal(t, "2026-06-07", payload.To)
		require.Len(t, payload.Days, 1)
		require.Equal(t, to, payload.Days[0].DayStart)
		require.Equal(t, []apiresponses.FeelingScoreResponse{{Label: "sad", Confidence: 0.8}}, payload.Days[0].DominantFeelings)
	})

	t.Run("should reject unauthenticated requests", func(t *testing.T) {
		handler := NewDashboardHandler(newTestHTTPLogger(), newMockWeeklySummaryGetter(t), newMockTimelineSummaryGetter(t))

		req := httptest.NewRequest(http.MethodGet, "/dashboard/timeline", nil)
		rec := httptest.NewRecorder()

		handler.GetTimeline(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.JSONEq(t, `{"message":"unauthorized"}`, rec.Body.String())
	})

	t.Run("should reject missing paired params", func(t *testing.T) {
		handler := NewDashboardHandler(newTestHTTPLogger(), newMockWeeklySummaryGetter(t), newMockTimelineSummaryGetter(t))
		req := httptest.NewRequest(http.MethodGet, "/dashboard/timeline?from=2026-06-01", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.GetTimeline))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		require.JSONEq(t, `{"message":"query params 'from' and 'to' must be provided together"}`, rec.Body.String())
	})

	t.Run("should reject invalid date format", func(t *testing.T) {
		handler := NewDashboardHandler(newTestHTTPLogger(), newMockWeeklySummaryGetter(t), newMockTimelineSummaryGetter(t))
		req := httptest.NewRequest(http.MethodGet, "/dashboard/timeline?from=2026/06/01&to=2026-06-07", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.GetTimeline))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		require.JSONEq(t, `{"message":"query params 'from' and 'to' must use YYYY-MM-DD"}`, rec.Body.String())
	})

	t.Run("should reject inverted ranges", func(t *testing.T) {
		handler := NewDashboardHandler(newTestHTTPLogger(), newMockWeeklySummaryGetter(t), newMockTimelineSummaryGetter(t))
		req := httptest.NewRequest(http.MethodGet, "/dashboard/timeline?from=2026-06-08&to=2026-06-07", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.GetTimeline))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		require.JSONEq(t, `{"message":"query param 'to' must be on or after 'from'"}`, rec.Body.String())
	})

	t.Run("should reject ranges longer than 30 days", func(t *testing.T) {
		handler := NewDashboardHandler(newTestHTTPLogger(), newMockWeeklySummaryGetter(t), newMockTimelineSummaryGetter(t))
		req := httptest.NewRequest(http.MethodGet, "/dashboard/timeline?from=2026-05-01&to=2026-06-01", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.GetTimeline))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		require.JSONEq(t, `{"message":"dashboard timeline range must not exceed 30 days"}`, rec.Body.String())
	})

	t.Run("should return internal server error when timeline lookup fails", func(t *testing.T) {
		getter := newMockTimelineSummaryGetter(t)
		handler := NewDashboardHandler(newTestHTTPLogger(), newMockWeeklySummaryGetter(t), getter)
		req := httptest.NewRequest(http.MethodGet, "/dashboard/timeline", nil)
		req.Header.Set("Authorization", "Bearer tok_123")
		rec := httptest.NewRecorder()

		getter.EXPECT().
			GetTimeline(mock.Anything, "usr_123", (*time.Time)(nil), (*time.Time)(nil)).
			Return(domain.DashboardTimeline{}, errors.New("database unavailable")).
			Once()

		protected := httpmiddleware.RequireAuth(chatHandlerAuthStub{user: domain.User{ID: "usr_123"}})(http.HandlerFunc(handler.GetTimeline))
		protected.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.JSONEq(t, `{"message":"failed to load dashboard timeline"}`, rec.Body.String())
	})
}
