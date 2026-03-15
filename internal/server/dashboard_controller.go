package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mxcd/rabbithole/internal/util"
	"github.com/rs/zerolog/log"
)

type dashboardClient struct {
	conn *websocket.Conn
	done chan struct{}
}

var (
	dashboardClients   = make(map[*dashboardClient]struct{})
	dashboardClientsMu sync.Mutex
)

func (s *Server) registerDashboardRoutes() {
	api := s.Engine.Group(s.Options.ApiBaseUrl)
	{
		api.GET("/tunnels", s.handleListTunnels)
		api.GET("/version", s.handleVersion)
		api.GET("/dashboard/ws", s.handleDashboardWS)
	}

	go s.dashboardBroadcastLoop()
}

func (s *Server) handleListTunnels(c *gin.Context) {
	tunnels := s.Options.Registry.List()
	c.JSON(http.StatusOK, gin.H{
		"tunnels": tunnels,
		"count":   len(tunnels),
	})
}

func (s *Server) handleVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": util.Version,
	})
}

func (s *Server) handleDashboardWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to upgrade dashboard WebSocket")
		return
	}

	client := &dashboardClient{
		conn: conn,
		done: make(chan struct{}),
	}

	dashboardClientsMu.Lock()
	dashboardClients[client] = struct{}{}
	dashboardClientsMu.Unlock()

	defer func() {
		dashboardClientsMu.Lock()
		delete(dashboardClients, client)
		dashboardClientsMu.Unlock()
		conn.Close()
	}()

	// Send initial state
	tunnels := s.Options.Registry.List()
	conn.WriteJSON(gin.H{
		"type":    "tunnels",
		"tunnels": tunnels,
	})

	// Keep connection alive by reading (handles close/ping)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) dashboardBroadcastLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		dashboardClientsMu.Lock()
		if len(dashboardClients) == 0 {
			dashboardClientsMu.Unlock()
			continue
		}

		tunnels := s.Options.Registry.List()
		msg := gin.H{
			"type":    "tunnels",
			"tunnels": tunnels,
		}

		for client := range dashboardClients {
			if err := client.conn.WriteJSON(msg); err != nil {
				client.conn.Close()
				delete(dashboardClients, client)
			}
		}
		dashboardClientsMu.Unlock()
	}
}
