package server

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/mxcd/rabbithole/internal/tunnel"
	"github.com/mxcd/rabbithole/pkg/protocol"
	"github.com/rs/zerolog/log"
)

func (s *Server) routingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		host := strings.Split(c.Request.Host, ":")[0]
		if host == s.Options.BaseDomain || host == "localhost" {
			c.Next()
			return
		}

		subdomain := strings.TrimSuffix(host, "."+s.Options.BaseDomain)
		if subdomain == host {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		if isWebSocketUpgrade(c.Request) {
			s.proxyWebSocket(c, subdomain)
		} else {
			s.proxyHTTP(c, subdomain)
		}
		c.Abort()
	}
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func (s *Server) proxyHTTP(c *gin.Context, subdomain string) {
	t, ok := s.Options.Registry.Get(subdomain)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found", "subdomain": subdomain})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to read request body"})
		return
	}

	requestID := uuid.New().String()
	headers := make(map[string][]string)
	for k, v := range c.Request.Header {
		headers[k] = v
	}

	req := protocol.HTTPRequest{
		Message: protocol.Message{
			ID:   requestID,
			Type: protocol.TypeHTTPRequest,
		},
		Method:  c.Request.Method,
		Path:    c.Request.URL.Path,
		Query:   c.Request.URL.RawQuery,
		Headers: headers,
		Body:    body,
	}

	respChan := t.RegisterPendingRequest(requestID)
	defer t.RemovePendingRequest(requestID)

	if err := t.WriteJSON(req); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "tunnel connection lost"})
		return
	}

	t.RequestCount.Add(1)
	t.LastRequest.Store(time.Now().Format(time.RFC3339))

	timeout := time.Duration(s.Options.TunnelTimeout) * time.Second
	select {
	case resp := <-respChan:
		for k, values := range resp.Headers {
			for _, v := range values {
				c.Writer.Header().Add(k, v)
			}
		}
		c.Data(resp.StatusCode, "", resp.Body)

	case <-time.After(timeout):
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "tunnel request timed out"})
	}
}

func (s *Server) proxyWebSocket(c *gin.Context, subdomain string) {
	t, ok := s.Options.Registry.Get(subdomain)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found", "subdomain": subdomain})
		return
	}

	streamID := uuid.New().String()

	stream := &tunnel.WSStream{
		FrameChan:  make(chan *protocol.WSFrame, 64),
		OpenedChan: make(chan struct{}, 1),
		Done:       make(chan struct{}),
	}
	t.RegisterWSStream(streamID, stream)
	defer t.DeregisterWSStream(streamID)

	headers := make(map[string][]string)
	for k, v := range c.Request.Header {
		headers[k] = v
	}

	wsOpen := protocol.WSOpen{
		Message: protocol.Message{
			ID:   uuid.New().String(),
			Type: protocol.TypeWSOpen,
		},
		StreamID: streamID,
		Path:     c.Request.URL.Path,
		Query:    c.Request.URL.RawQuery,
		Headers:  headers,
	}

	if err := t.WriteJSON(wsOpen); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "tunnel connection lost"})
		return
	}

	select {
	case <-stream.OpenedChan:
	case <-time.After(10 * time.Second):
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "WebSocket tunnel open timed out"})
		return
	case <-stream.Done:
		c.JSON(http.StatusBadGateway, gin.H{"error": "tunnel closed"})
		return
	}

	browserConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("failed to upgrade browser WebSocket")
		return
	}
	defer browserConn.Close()
	stream.BrowserConn = browserConn

	t.RequestCount.Add(1)
	t.LastRequest.Store(time.Now().Format(time.RFC3339))

	done := make(chan struct{})

	// Browser → tunnel
	go func() {
		defer close(done)
		for {
			msgType, data, err := browserConn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debug().Err(err).Str("streamId", streamID).Msg("browser WS read error")
				}
				t.WriteJSON(protocol.WSClose{
					Message:  protocol.Message{ID: uuid.New().String(), Type: protocol.TypeWSClose},
					StreamID: streamID,
				})
				return
			}
			t.WriteJSON(protocol.WSFrame{
				Message:   protocol.Message{ID: uuid.New().String(), Type: protocol.TypeWSFrame},
				StreamID:  streamID,
				FrameType: msgType,
				Data:      data,
			})
		}
	}()

	// Tunnel → browser
	go func() {
		for {
			select {
			case frame := <-stream.FrameChan:
				if err := browserConn.WriteMessage(frame.FrameType, frame.Data); err != nil {
					log.Debug().Err(err).Str("streamId", streamID).Msg("browser WS write error")
					return
				}
			case <-stream.Done:
				browserConn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			case <-done:
				return
			}
		}
	}()

	select {
	case <-done:
	case <-stream.Done:
	}
}
