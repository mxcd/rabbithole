package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/mxcd/go-config/config"
	"github.com/mxcd/rabbithole/internal/auth"
	"github.com/mxcd/rabbithole/internal/tunnel"
	"github.com/mxcd/rabbithole/internal/web"
	"github.com/rs/zerolog/log"
)

type ServerOptions struct {
	DevMode       bool
	Port          int
	ApiBaseUrl    string
	BaseDomain    string
	TunnelTimeout int
	StaticHosting bool
	UiProxyUrl    string
	Registry      *tunnel.Registry
	Auth          *auth.APIKeyAuth
}

type Server struct {
	Options    *ServerOptions
	Engine     *gin.Engine
	HttpServer *http.Server
}

func NewServer(options *ServerOptions) (*Server, error) {
	if options == nil {
		return nil, fmt.Errorf("server options cannot be nil")
	}

	server := &Server{
		Options: options,
	}

	if !server.Options.DevMode {
		log.Info().Msg("Running Gin in production mode")
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	server.Engine = engine
	server.Engine.Use(gin.Recovery())

	server.Engine.Use(server.routingMiddleware())

	server.HttpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", options.Port),
		Handler: engine,
	}

	if server.Options.DevMode {
		log.Info().Msg("Running Gin in development mode")
		log.Warn().Msg("CORS is enabled")
		corsConfig := cors.DefaultConfig()
		corsConfig.AllowHeaders = []string{"Authorization", "Content-Type", "X-Requested-With"}
		corsConfig.AllowOrigins = config.Get().StringArray("CORS_ALLOWED_ORIGINS")
		corsConfig.AllowCredentials = true
		server.Engine.Use(cors.New(corsConfig))
	}

	return server, nil
}

func (s *Server) RegisterRoutes() error {
	s.registerHealthRoute()
	s.registerTunnelRoutes()
	s.registerDashboardRoutes()

	if err := web.RegisterUI(&web.WebHostingOptions{
		DevMode:       s.Options.DevMode,
		StaticHosting: s.Options.StaticHosting,
		UIProxyUrl:    s.Options.UiProxyUrl,
		Engine:        s.Engine,
	}); err != nil {
		return err
	}

	return nil
}

func (s *Server) Run() error {
	if err := s.HttpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) {
	s.HttpServer.Shutdown(ctx)
}
