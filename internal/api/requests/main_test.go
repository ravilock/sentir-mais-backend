package requests

import (
	"log/slog"
	"os"
	"testing"

	"github.com/ravilock/sentir-mais-backend/internal/validations"
)

func TestMain(m *testing.M) {
	if err := validations.InitValidator(); err != nil {
		slog.New(slog.NewTextHandler(os.Stdout, nil)).Error("failed to load validator", "error", err)
		os.Exit(1)
	}

	code := m.Run()
	os.Exit(code)
}
