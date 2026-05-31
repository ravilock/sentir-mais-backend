package http

import (
	"encoding/json"
	"net/http"

	"github.com/ravilock/sentir-mais-backend/internal/http/handlers"
)

type RouterDependencies struct {
	Auth      *handlers.AuthHandler
	Chat      *handlers.ChatHandler
	Dashboard *handlers.DashboardHandler
	Protect   func(http.Handler) http.Handler
}

func NewRouter(deps RouterDependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /auth/register", deps.Auth.Register)
	mux.HandleFunc("POST /auth/login", deps.Auth.Login)
	mux.Handle("GET /auth/me", deps.Protect(http.HandlerFunc(deps.Auth.Me)))

	mux.Handle("POST /chats", deps.Protect(http.HandlerFunc(deps.Chat.CreateChat)))
	mux.Handle("POST /chats/{chatId}/messages", deps.Protect(http.HandlerFunc(deps.Chat.SendMessage)))
	mux.Handle("GET /chats/{chatId}/messages", deps.Protect(http.HandlerFunc(deps.Chat.ListMessages)))

	mux.Handle("GET /dashboard/week", deps.Protect(http.HandlerFunc(deps.Dashboard.GetWeek)))

	return mux
}
