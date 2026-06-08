package main

import (
	"errors"
	"net/http"
	"os"

	"github.com/ravilock/sentir-mais-backend/internal/api"
	"github.com/ravilock/sentir-mais-backend/internal/config"
	applog "github.com/ravilock/sentir-mais-backend/internal/log"
)

func main() {
	cfg := config.Load()
	logger := applog.New(cfg, map[string]string{
		"component": "main",
	})

	server, err := api.NewServer(cfg)
	if err != nil {
		logger.Error("failed to build api server", "error", err)
		os.Exit(1)
	}

	if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
