package server

import (
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mxcd/rabbithole/internal/auth"
	"github.com/mxcd/rabbithole/internal/tunnel"
	"github.com/mxcd/rabbithole/pkg/protocol"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) registerTunnelRoutes() {
	s.Engine.GET(s.Options.ApiBaseUrl+"/tunnel/connect", s.handleTunnelConnect)
}

func (s *Server) handleTunnelConnect(c *gin.Context) {
	requestedName := c.Query("name")

	// Authenticate before upgrade
	apiKeyLabel := ""
	if s.Options.Auth != nil && s.Options.Auth.Enabled() {
		key := auth.ExtractAPIKeyFromRequest(c.Request)
		if key == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			return
		}
		label, ok := s.Options.Auth.Validate(key)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}
		apiKeyLabel = label
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to upgrade WebSocket connection")
		return
	}

	subdomain := requestedName
	if subdomain == "" {
		subdomain = generateSubdomain()
	}

	if _, exists := s.Options.Registry.Get(subdomain); exists {
		conn.WriteJSON(protocol.Message{
			Type: protocol.TypeWSError,
		})
		conn.Close()
		log.Warn().Str("subdomain", subdomain).Msg("subdomain already in use")
		return
	}

	t := tunnel.NewTunnel(subdomain, conn, apiKeyLabel)

	s.Options.Registry.Register(subdomain, t)
	log.Info().Str("subdomain", subdomain).Msg("tunnel registered")

	scheme := "https"
	if s.Options.DevMode {
		scheme = "http"
	}
	tunnelURL := fmt.Sprintf("%s://%s.%s", scheme, subdomain, s.Options.BaseDomain)

	err = t.WriteJSON(protocol.TunnelInfo{
		Message: protocol.Message{
			Type: protocol.TypeTunnelInfo,
		},
		Subdomain: subdomain,
		URL:       tunnelURL,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to send tunnel info")
		s.Options.Registry.Deregister(subdomain)
		conn.Close()
		return
	}

	stopKeepalive := make(chan struct{})
	t.StartKeepalive(stopKeepalive)

	t.ReadLoop()
	close(stopKeepalive)

	s.Options.Registry.Deregister(subdomain)
	conn.Close()
	log.Info().Str("subdomain", subdomain).Msg("tunnel deregistered")
}

func generateSubdomain() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
