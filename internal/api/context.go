package api

import (
	"context"
	"net/http"
)

type contextKey string

const requestIDContextKey contextKey = "request-id"

func requestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey).(string)
	return value
}

func withRequestID(r *http.Request, requestID string) *http.Request {
	ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
	return r.WithContext(ctx)
}
