package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	analysisrepositories "github.com/ravilock/sentir-mais-backend/internal/analysis/repositories"
	authrepositories "github.com/ravilock/sentir-mais-backend/internal/auth/repositories"
	authservices "github.com/ravilock/sentir-mais-backend/internal/auth/services"
	chatrepositories "github.com/ravilock/sentir-mais-backend/internal/chat/repositories"
	chatservices "github.com/ravilock/sentir-mais-backend/internal/chat/services"
	"github.com/ravilock/sentir-mais-backend/internal/classifier"
	"github.com/ravilock/sentir-mais-backend/internal/config"
	dashboardservices "github.com/ravilock/sentir-mais-backend/internal/dashboard/services"
	apihandlers "github.com/ravilock/sentir-mais-backend/internal/http/handlers"
	httpmiddleware "github.com/ravilock/sentir-mais-backend/internal/http/middleware"
	"github.com/ravilock/sentir-mais-backend/internal/llm"
	"github.com/ravilock/sentir-mais-backend/internal/storage/mongodb"
	"github.com/ravilock/sentir-mais-backend/internal/validations"
)

type Server interface {
	http.Handler
	Start() error
}

type server struct {
	httpServer *http.Server
	logger     *slog.Logger
	close      func() error

	authHandler      *apihandlers.AuthHandler
	chatHandler      *apihandlers.ChatHandler
	dashboardHandler *apihandlers.DashboardHandler
	protect          func(http.Handler) http.Handler
}

func NewServer(cfg config.Config) (Server, error) {
	if err := validations.InitValidator(); err != nil {
		return nil, err
	}

	storeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connection, err := mongodb.Connect(storeCtx, cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		return nil, err
	}

	userRepository, err := authrepositories.NewUserRepository(storeCtx, connection.Database)
	if err != nil {
		_ = connection.Close(context.Background())
		return nil, err
	}

	sessionRepository, err := authrepositories.NewSessionRepository(storeCtx, connection.Database)
	if err != nil {
		_ = connection.Close(context.Background())
		return nil, err
	}

	chatRepository, err := chatrepositories.NewChatRepository(storeCtx, connection.Database)
	if err != nil {
		_ = connection.Close(context.Background())
		return nil, err
	}

	messageRepository, err := chatrepositories.NewMessageRepository(storeCtx, connection.Database)
	if err != nil {
		_ = connection.Close(context.Background())
		return nil, err
	}

	messageAnalysisRepository, err := analysisrepositories.NewMessageAnalysisRepository(storeCtx, connection.Database)
	if err != nil {
		_ = connection.Close(context.Background())
		return nil, err
	}

	registerService := authservices.NewRegisterService(userRepository, userRepository, sessionRepository, cfg.SessionTTL)
	loginService := authservices.NewLoginService(userRepository, sessionRepository, cfg.SessionTTL)
	authenticateService := authservices.NewAuthenticateService(sessionRepository, userRepository)

	responder := llm.SupportClient(llm.NewStubSupportClient())
	var extractor llm.Extractor
	if cfg.PrompterBaseURL != "" {
		prompterClient := llm.NewPrompterClient(cfg.PrompterBaseURL, cfg.PrompterAPIKey, cfg.PrompterTimeout)
		responder = prompterClient
		extractor = prompterClient
	}
	classifierClient := classifier.NewClient(cfg.ClassifierBaseURL, cfg.ClassifierAPIKey, cfg.ClassifierTimeout)
	createChatService := chatservices.NewCreateChatService(chatRepository, messageRepository, responder).WithAnalysis(classifierClient, messageAnalysisRepository).WithExtraction(extractor)
	sendMessageService := chatservices.NewSendMessageService(chatRepository, messageRepository, messageRepository, chatRepository, responder).WithAnalysis(classifierClient, messageAnalysisRepository).WithExtraction(extractor)
	listMessagesService := chatservices.NewListMessagesService(chatRepository, messageRepository)
	dashboardService := dashboardservices.NewGetWeekService()

	logger := newLogger()
	srv := &server{
		logger:           logger,
		close:            func() error { return connection.Close(context.Background()) },
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
	err := s.httpServer.ListenAndServe()
	if s.close != nil {
		if closeErr := s.close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
	}

	return err
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
