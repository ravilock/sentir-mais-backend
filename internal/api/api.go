package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	authrepositories "github.com/ravilock/sentir-mais-backend/internal/auth/repositories"
	authservices "github.com/ravilock/sentir-mais-backend/internal/auth/services"
	chatrepositories "github.com/ravilock/sentir-mais-backend/internal/chat/repositories"
	chatservices "github.com/ravilock/sentir-mais-backend/internal/chat/services"
	"github.com/ravilock/sentir-mais-backend/internal/config"
	dashboardservices "github.com/ravilock/sentir-mais-backend/internal/dashboard/services"
	apihandlers "github.com/ravilock/sentir-mais-backend/internal/http/handlers"
	httpmiddleware "github.com/ravilock/sentir-mais-backend/internal/http/middleware"
	"github.com/ravilock/sentir-mais-backend/internal/llm"
	"github.com/ravilock/sentir-mais-backend/internal/storage/memory"
)

type Server interface {
	http.Handler
	Start() error
}

type server struct {
	httpServer *http.Server
	logger     *slog.Logger

	authHandler      *apihandlers.AuthHandler
	chatHandler      *apihandlers.ChatHandler
	dashboardHandler *apihandlers.DashboardHandler
	protect          func(http.Handler) http.Handler
}

func NewServer(cfg config.Config) (Server, error) {
	store := memory.NewStore()
	userRepository := authrepositories.NewUserRepository(store)
	sessionRepository := authrepositories.NewSessionRepository(store)
	chatRepository := chatrepositories.NewChatRepository(store)
	messageRepository := chatrepositories.NewMessageRepository(store)

	registerService := authservices.NewRegisterService(userRepository, userRepository, sessionRepository, cfg.SessionTTL)
	loginService := authservices.NewLoginService(userRepository, sessionRepository, cfg.SessionTTL)
	authenticateService := authservices.NewAuthenticateService(sessionRepository, userRepository)

	stubClient := llm.NewStubSupportClient()
	createChatService := chatservices.NewCreateChatService(chatRepository, messageRepository, stubClient)
	sendMessageService := chatservices.NewSendMessageService(chatRepository, messageRepository, messageRepository, chatRepository, stubClient)
	listMessagesService := chatservices.NewListMessagesService(chatRepository, messageRepository)
	dashboardService := dashboardservices.NewGetWeekService()

	logger := newLogger()
	srv := &server{
		logger:           logger,
		authHandler:      apihandlers.NewAuthHandler(registerService, loginService),
		chatHandler:      apihandlers.NewChatHandler(createChatService, sendMessageService, listMessagesService),
		dashboardHandler: apihandlers.NewDashboardHandler(dashboardService),
		protect:          httpmiddleware.RequireAuth(authenticateService),
	}

	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	srv.httpServer = &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           chain(mux, recoverMiddleware(logger), requestIDMiddleware(), corsMiddleware(cfg.CORSAllowedOrigins), loggingMiddleware(logger)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	return srv, nil
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.httpServer.Handler.ServeHTTP(w, r)
}

func (s *server) Start() error {
	s.logger.Info("starting api server", "address", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", healthcheck)
	mux.HandleFunc("GET /api/healthcheck", healthcheck)

	for _, prefix := range []string{"", "/api/v1"} {
		s.createAuthRoutes(mux, prefix)
		s.createChatRoutes(mux, prefix)
		s.createDashboardRoutes(mux, prefix)
	}
}

func (s *server) createAuthRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(routePattern(http.MethodPost, prefix, "/auth/register"), s.authHandler.Register)
	mux.HandleFunc(routePattern(http.MethodPost, prefix, "/auth/login"), s.authHandler.Login)
	mux.Handle(routePattern(http.MethodGet, prefix, "/auth/me"), s.protect(http.HandlerFunc(s.authHandler.Me)))
}

func (s *server) createChatRoutes(mux *http.ServeMux, prefix string) {
	mux.Handle(routePattern(http.MethodPost, prefix, "/chats"), s.protect(http.HandlerFunc(s.chatHandler.CreateChat)))
	mux.Handle(routePattern(http.MethodPost, prefix, "/chats/{chatId}/messages"), s.protect(http.HandlerFunc(s.chatHandler.SendMessage)))
	mux.Handle(routePattern(http.MethodGet, prefix, "/chats/{chatId}/messages"), s.protect(http.HandlerFunc(s.chatHandler.ListMessages)))
}

func (s *server) createDashboardRoutes(mux *http.ServeMux, prefix string) {
	mux.Handle(routePattern(http.MethodGet, prefix, "/dashboard/week"), s.protect(http.HandlerFunc(s.dashboardHandler.GetWeek)))
}

func routePattern(method, prefix, path string) string {
	return fmt.Sprintf("%s %s%s", method, prefix, path)
}
