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
		handler := NewDashboardHandler(newTestHTTPLogger(), getter)
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
		handler := NewDashboardHandler(newTestHTTPLogger(), newMockWeeklySummaryGetter(t))

		req := httptest.NewRequest(http.MethodGet, "/dashboard/week", nil)
		rec := httptest.NewRecorder()

		handler.GetWeek(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
		require.JSONEq(t, `{"message":"unauthorized"}`, rec.Body.String())
	})

	t.Run("should return internal server error when summary lookup fails", func(t *testing.T) {
		getter := newMockWeeklySummaryGetter(t)
		handler := NewDashboardHandler(newTestHTTPLogger(), getter)

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
