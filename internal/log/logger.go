package log

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/ravilock/sentir-mais-backend/internal/config"
)

const (
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
)

type contextKey string

const requestIDContextKey contextKey = "request-id"

type requestHandler struct {
	handler slog.Handler
}

func New(cfg config.Config, logContext map[string]string) *slog.Logger {
	attrs := []slog.Attr{}
	for key, value := range logContext {
		attrs = append(attrs, slog.Attr{Key: key, Value: slog.StringValue(value)})
	}

	jsonHandler := slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{
			Level: getLogLevel(cfg.LogLevel),
		},
	).WithAttrs(attrs)

	return slog.New(requestHandler{handler: jsonHandler})
}

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDContextKey).(string)
	return value
}

func WithRequestID(r *http.Request, requestID string) *http.Request {
	ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
	return r.WithContext(ctx)
}

func getLogLevel(level string) slog.Level {
	switch level {
	case logLevelDebug:
		return slog.LevelDebug
	case logLevelInfo:
		return slog.LevelInfo
	case logLevelWarn:
		return slog.LevelWarn
	case logLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (h requestHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		r.Add("request_id", slog.StringValue(requestID))
	}

	return h.handler.Handle(ctx, r)
}

func (h requestHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h requestHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return requestHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h requestHandler) WithGroup(name string) slog.Handler {
	return requestHandler{handler: h.handler.WithGroup(name)}
}
