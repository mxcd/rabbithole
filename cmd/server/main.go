package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/mxcd/go-config/config"
	"github.com/mxcd/rabbithole/internal/auth"
	"github.com/mxcd/rabbithole/internal/authentication"
	internalconfig "github.com/mxcd/rabbithole/internal/config"
	"github.com/mxcd/rabbithole/internal/server"
	"github.com/mxcd/rabbithole/internal/tunnel"
	"github.com/mxcd/rabbithole/internal/util"
)

func main() {
	if err := internalconfig.InitConfig(); err != nil {
		log.Panic().Err(err).Msg("error initializing config")
	}
	config.Print()

	if err := util.InitLogger(); err != nil {
		log.Panic().Err(err).Msg("error initializing logger")
	}

	registry := tunnel.NewRegistry()
	apiKeyAuth := auth.NewAPIKeyAuth(config.Get().String("API_KEYS"))

	srv := initServer(registry, apiKeyAuth)

	if _, err := authentication.Init(&authentication.Options{
		Engine:               srv.Engine,
		ApiBaseUrl:           config.Get().String("API_BASE_URL"),
		SessionSigningKey:    []byte(config.Get().String("SESSION_SECRET_KEY")),
		SessionEncryptionKey: []byte(config.Get().String("SESSION_ENCRYPTION_KEY")),
		DefaultAdminPassword: config.Get().String("DEFAULT_ADMIN_PASSWORD"),
		IsDev:                config.Get().Bool("DEV"),
	}); err != nil {
		log.Panic().Err(err).Msg("error initializing authentication")
	}

	if err := srv.RegisterRoutes(); err != nil {
		log.Panic().Err(err).Msg("error registering routes")
	}

	go func() {
		if err := srv.Run(); err != nil {
			log.Panic().Err(err).Msg("error running server")
		}
	}()

	log.Info().Int("port", config.Get().Int("PORT")).Msg("server started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("received shutdown signal")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv.Shutdown(ctx)
	log.Info().Msg("server shutdown complete")
}

func initServer(registry *tunnel.Registry, apiKeyAuth *auth.APIKeyAuth) *server.Server {
	srv, err := server.NewServer(&server.ServerOptions{
		DevMode:       config.Get().Bool("DEV"),
		Port:          config.Get().Int("PORT"),
		ApiBaseUrl:    config.Get().String("API_BASE_URL"),
		BaseDomain:    config.Get().String("BASE_DOMAIN"),
		TunnelTimeout: config.Get().Int("TUNNEL_TIMEOUT"),
		StaticHosting: config.Get().Bool("STATIC_HOSTING"),
		UiProxyUrl:    config.Get().String("UI_PROXY_URL"),
		Registry:      registry,
		Auth:          apiKeyAuth,
	})
	if err != nil {
		log.Panic().Err(err).Msg("error initializing server")
	}
	return srv
}
