package handlers

import (
	"log/slog"
	"os"
)

func newTestHTTPLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}
