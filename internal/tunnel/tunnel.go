package tunnel

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mxcd/rabbithole/pkg/protocol"
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
