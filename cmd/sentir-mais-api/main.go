package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/ravilock/sentir-mais-backend/internal/api"
	"github.com/ravilock/sentir-mais-backend/internal/config"
)

func main() {
	cfg := config.Load()

	server, err := api.NewServer(cfg)
	if err != nil {
		log.Fatalf("failed to build api server: %v", err)
	}

	if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped: %v", err)
	}
}
