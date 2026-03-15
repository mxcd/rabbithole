package protocol

type MessageType string

const (
	TypeTunnelInfo   MessageType = "tunnel_info"
	TypeHTTPRequest  MessageType = "http_request"
	TypeHTTPResponse MessageType = "http_response"
	TypeWSOpen       MessageType = "ws_open"
	TypeWSOpened     MessageType = "ws_opened"
	TypeWSFrame      MessageType = "ws_frame"
	TypeWSClose      MessageType = "ws_close"
	TypeWSError      MessageType = "ws_error"
	TypePing         MessageType = "ping"
	TypePong         MessageType = "pong"
)

type Message struct {
	ID   string      `json:"id"`
	Type MessageType `json:"type"`
}

type TunnelInfo struct {
	Message
	Subdomain string `json:"subdomain"`
	URL       string `json:"url"`
}

type HTTPRequest struct {
	Message
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   string              `json:"query"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`
}

type HTTPResponse struct {
	Message
	StatusCode int                 `json:"statusCode"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
}

type WSOpen struct {
	Message
	StreamID string              `json:"streamId"`
	Path     string              `json:"path"`
	Query    string              `json:"query"`
	Headers  map[string][]string `json:"headers"`
}

type WSOpened struct {
	Message
	StreamID string `json:"streamId"`
}

type WSFrame struct {
	Message
	StreamID  string `json:"streamId"`
	FrameType int    `json:"frameType"`
	Data      []byte `json:"data"`
}

type WSClose struct {
	Message
	StreamID string `json:"streamId"`
	Code     int    `json:"code"`
	Reason   string `json:"reason"`
}
