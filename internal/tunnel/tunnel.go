package tunnel

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mxcd/rabbithole/pkg/protocol"
	"github.com/rs/zerolog/log"
)

// Keepalive: cloud LBs (e.g. Hetzner Cloud LB) close idle TCP sessions after
// ~60 minutes. Sending WebSocket ping control frames keeps the path warm and
// lets us detect dead tunnels via the pong handler / read deadline.
const (
	pingInterval = 30 * time.Second
	pongWait     = 90 * time.Second
	writeWait    = 10 * time.Second
)

type Tunnel struct {
	Subdomain    string
	Connection   *websocket.Conn
	APIKeyLabel  string
	CreatedAt    time.Time
	RequestCount atomic.Int64
	LastRequest  atomic.Value // stores time string

	// HTTP request correlation
	PendingHTTP map[string]chan *protocol.HTTPResponse
	HttpMu      sync.Mutex

	// WebSocket stream tracking
	WsStreams map[string]*WSStream
	WsMu      sync.Mutex

	// Write serialization (only one goroutine writes to WS at a time)
	WriteMu sync.Mutex
}

type WSStream struct {
	BrowserConn *websocket.Conn
	FrameChan   chan *protocol.WSFrame
	OpenedChan  chan struct{}
	Done        chan struct{}
}

func NewTunnel(subdomain string, conn *websocket.Conn, apiKeyLabel string) *Tunnel {
	return &Tunnel{
		Subdomain:   subdomain,
		Connection:  conn,
		APIKeyLabel: apiKeyLabel,
		CreatedAt:   time.Now(),
		PendingHTTP: make(map[string]chan *protocol.HTTPResponse),
		WsStreams:   make(map[string]*WSStream),
	}
}

func (t *Tunnel) WriteJSON(v any) error {
	t.WriteMu.Lock()
	defer t.WriteMu.Unlock()
	return t.Connection.WriteJSON(v)
}

func (t *Tunnel) RegisterPendingRequest(id string) chan *protocol.HTTPResponse {
	t.HttpMu.Lock()
	defer t.HttpMu.Unlock()
	ch := make(chan *protocol.HTTPResponse, 1)
	t.PendingHTTP[id] = ch
	return ch
}

func (t *Tunnel) ResolvePendingRequest(id string, resp *protocol.HTTPResponse) {
	t.HttpMu.Lock()
	ch, ok := t.PendingHTTP[id]
	if ok {
		delete(t.PendingHTTP, id)
	}
	t.HttpMu.Unlock()
	if ok {
		ch <- resp
	}
}

func (t *Tunnel) RemovePendingRequest(id string) {
	t.HttpMu.Lock()
	defer t.HttpMu.Unlock()
	delete(t.PendingHTTP, id)
}

// StartKeepalive arms the WebSocket read deadline + pong handler and spawns a
// goroutine that sends ping control frames every pingInterval. The goroutine
// returns when stop is closed or a write fails (which also closes the
// underlying connection so the read loop unblocks).
func (t *Tunnel) StartKeepalive(stop <-chan struct{}) {
	t.Connection.SetReadDeadline(time.Now().Add(pongWait))
	t.Connection.SetPongHandler(func(string) error {
		t.Connection.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	go t.pingLoop(stop)
}

func (t *Tunnel) pingLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			t.WriteMu.Lock()
			err := t.Connection.WriteControl(
				websocket.PingMessage,
				nil,
				time.Now().Add(writeWait),
			)
			t.WriteMu.Unlock()
			if err != nil {
				log.Debug().Err(err).Str("subdomain", t.Subdomain).Msg("ping write failed; closing tunnel")
				_ = t.Connection.Close()
				return
			}
		}
	}
}
