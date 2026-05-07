package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
	"github.com/mxcd/rabbithole/internal/util"
	"github.com/mxcd/rabbithole/pkg/protocol"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

var version = util.Version

type Config struct {
	Server string `yaml:"server"`
	APIKey string `yaml:"apiKey"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "rh", "config.yaml")
}

func loadConfig() Config {
	var cfg Config
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	yaml.Unmarshal(data, &cfg)
	return cfg
}

func saveConfig(cfg Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func main() {
	app := &cli.Command{
		Name:    "rh",
		Usage:   "Rabbithole Dev Proxy",
		Version: version,
		Commands: []*cli.Command{
			tunnelCommand(),
			configCommand(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func tunnelCommand() *cli.Command {
	return &cli.Command{
		Name:      "tunnel",
		Usage:     "Expose a local port through a tunnel",
		ArgsUsage: "<port>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Aliases: []string{"n"}, Usage: "Request specific subdomain"},
			&cli.StringFlag{Name: "server", Aliases: []string{"s"}, Usage: "Override server URL"},
			&cli.StringFlag{Name: "key", Aliases: []string{"k"}, Usage: "Override API key"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() < 1 {
				return fmt.Errorf("port argument required")
			}
			port, err := strconv.Atoi(cmd.Args().First())
			if err != nil {
				return fmt.Errorf("invalid port: %s", cmd.Args().First())
			}

			cfg := loadConfig()
			serverURL := cmd.String("server")
			if serverURL == "" {
				serverURL = os.Getenv("RABBITHOLE_BASE_URL")
			}
			if serverURL == "" {
				serverURL = cfg.Server
			}
			if serverURL == "" {
				return fmt.Errorf("server URL not configured. Set RABBITHOLE_BASE_URL or run: rh config set server <url>")
			}

			apiKey := cmd.String("key")
			if apiKey == "" {
				apiKey = os.Getenv("RABBITHOLE_API_KEY")
			}
			if apiKey == "" {
				apiKey = cfg.APIKey
			}

			name := cmd.String("name")
			return runTunnel(serverURL, apiKey, name, port)
		},
	}
}

func configCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Manage CLI configuration",
		Commands: []*cli.Command{
			{
				Name:      "set",
				Usage:     "Set a config value",
				ArgsUsage: "<key> <value>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.NArg() < 2 {
						return fmt.Errorf("usage: rh config set <key> <value>")
					}
					key := cmd.Args().Get(0)
					val := cmd.Args().Get(1)
					cfg := loadConfig()
					switch key {
					case "server":
						cfg.Server = val
					case "apikey", "apiKey":
						cfg.APIKey = val
					default:
						return fmt.Errorf("unknown config key: %s (valid: server, apikey)", key)
					}
					if err := saveConfig(cfg); err != nil {
						return err
					}
					fmt.Printf("Set %s = %s\n", key, val)
					return nil
				},
			},
			{
				Name:  "show",
				Usage: "Show current config",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg := loadConfig()
					data, _ := yaml.Marshal(cfg)
					fmt.Print(string(data))
					return nil
				},
			},
		},
	}
}

func runTunnel(serverURL, apiKey, name string, port int) error {
	u, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	wsScheme := "ws"
	if u.Scheme == "https" {
		wsScheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s%s/tunnel/connect", wsScheme, u.Host, strings.TrimRight(u.Path, "/")+"/api/v1")
	if name != "" {
		wsURL += "?name=" + url.QueryEscape(name)
	}

	header := http.Header{}
	if apiKey != "" {
		header.Set("Authorization", "Bearer "+apiKey)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()

	const (
		pingInterval = 30 * time.Second
		pongWait     = 90 * time.Second
		writeWait    = 10 * time.Second
	)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	_, raw, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read tunnel info: %w", err)
	}

	var info protocol.TunnelInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return fmt.Errorf("failed to parse tunnel info: %w", err)
	}

	if info.Type == protocol.TypeWSError {
		return fmt.Errorf("server rejected tunnel (subdomain may be in use)")
	}

	bold := color.New(color.Bold)
	cyan := color.New(color.FgCyan)
	bold.Println("\nRabbithole tunnel active")
	fmt.Printf("  %s → http://localhost:%d\n\n", cyan.Sprint(info.URL), port)
	fmt.Println("Press Ctrl+C to close the tunnel.")
	fmt.Println()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	localClient := &http.Client{Timeout: 30 * time.Second}
	var writeMu sync.Mutex

	// Track local WS connections for passthrough
	wsConns := make(map[string]*websocket.Conn)
	var wsMu sync.Mutex

	writeJSON := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteJSON(v)
	}

	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				writeMu.Lock()
				err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait))
				writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer close(done)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var msg protocol.Message
			if err := json.Unmarshal(raw, &msg); err != nil {
				continue
			}

			switch msg.Type {
			case protocol.TypeHTTPRequest:
				var req protocol.HTTPRequest
				if err := json.Unmarshal(raw, &req); err != nil {
					continue
				}
				go handleHTTPRequest(localClient, writeJSON, port, &req)

			case protocol.TypeWSOpen:
				var open protocol.WSOpen
				if err := json.Unmarshal(raw, &open); err != nil {
					continue
				}
				go handleWSOpen(writeJSON, &wsConns, &wsMu, port, &open)

			case protocol.TypeWSFrame:
				var frame protocol.WSFrame
				if err := json.Unmarshal(raw, &frame); err != nil {
					continue
				}
				wsMu.Lock()
				localConn, ok := wsConns[frame.StreamID]
				wsMu.Unlock()
				if ok {
					localConn.WriteMessage(frame.FrameType, frame.Data)
				}

			case protocol.TypeWSClose:
				var close protocol.WSClose
				if err := json.Unmarshal(raw, &close); err != nil {
					continue
				}
				wsMu.Lock()
				if localConn, ok := wsConns[close.StreamID]; ok {
					localConn.Close()
					delete(wsConns, close.StreamID)
				}
				wsMu.Unlock()

			case protocol.TypePing:
				writeJSON(protocol.Message{Type: protocol.TypePong})
			}
		}
	}()

	select {
	case <-quit:
		fmt.Println("\nShutting down tunnel...")
	case <-done:
		fmt.Println("\nTunnel connection lost")
	}
	return nil
}

func handleHTTPRequest(client *http.Client, writeJSON func(any) error, port int, req *protocol.HTTPRequest) {
	start := time.Now()
	localURL := fmt.Sprintf("http://localhost:%d%s", port, req.Path)
	if req.Query != "" {
		localURL += "?" + req.Query
	}

	httpReq, err := http.NewRequest(req.Method, localURL, strings.NewReader(string(req.Body)))
	if err != nil {
		writeJSON(protocol.HTTPResponse{
			Message:    protocol.Message{ID: req.ID, Type: protocol.TypeHTTPResponse},
			StatusCode: http.StatusBadGateway,
			Headers:    map[string][]string{"Content-Type": {"text/plain"}},
			Body:       []byte("failed to create request"),
		})
		return
	}

	for k, v := range req.Headers {
		httpReq.Header[k] = v
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		writeJSON(protocol.HTTPResponse{
			Message:    protocol.Message{ID: req.ID, Type: protocol.TypeHTTPResponse},
			StatusCode: http.StatusBadGateway,
			Headers:    map[string][]string{"Content-Type": {"text/plain"}},
			Body:       []byte(fmt.Sprintf("local server error: %v", err)),
		})
		printRequest(req.Method, req.Path, 502, time.Since(start))
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	headers := make(map[string][]string)
	for k, v := range resp.Header {
		headers[k] = v
	}

	writeJSON(protocol.HTTPResponse{
		Message:    protocol.Message{ID: req.ID, Type: protocol.TypeHTTPResponse},
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	})
	printRequest(req.Method, req.Path, resp.StatusCode, time.Since(start))
}

func handleWSOpen(writeJSON func(any) error, wsConns *map[string]*websocket.Conn, wsMu *sync.Mutex, port int, open *protocol.WSOpen) {
	wsScheme := "ws"
	wsURL := fmt.Sprintf("%s://localhost:%d%s", wsScheme, port, open.Path)
	if open.Query != "" {
		wsURL += "?" + open.Query
	}

	localConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		writeJSON(protocol.WSClose{
			Message:  protocol.Message{Type: protocol.TypeWSClose},
			StreamID: open.StreamID,
			Code:     1011,
			Reason:   fmt.Sprintf("failed to connect to local WebSocket: %v", err),
		})
		return
	}

	wsMu.Lock()
	(*wsConns)[open.StreamID] = localConn
	wsMu.Unlock()

	writeJSON(protocol.WSOpened{
		Message:  protocol.Message{Type: protocol.TypeWSOpened},
		StreamID: open.StreamID,
	})

	printWSEvent(open.Path, "open")

	// Local WS → tunnel
	go func() {
		defer func() {
			wsMu.Lock()
			delete(*wsConns, open.StreamID)
			wsMu.Unlock()
			localConn.Close()
			writeJSON(protocol.WSClose{
				Message:  protocol.Message{Type: protocol.TypeWSClose},
				StreamID: open.StreamID,
			})
			printWSEvent(open.Path, "closed")
		}()
		for {
			msgType, data, err := localConn.ReadMessage()
			if err != nil {
				return
			}
			writeJSON(protocol.WSFrame{
				Message:   protocol.Message{Type: protocol.TypeWSFrame},
				StreamID:  open.StreamID,
				FrameType: msgType,
				Data:      data,
			})
		}
	}()
}

func printRequest(method, path string, status int, duration time.Duration) {
	methodColor := color.New(color.FgWhite, color.Bold)
	var statusColor *color.Color
	switch {
	case status < 300:
		statusColor = color.New(color.FgGreen)
	case status < 400:
		statusColor = color.New(color.FgYellow)
	default:
		statusColor = color.New(color.FgRed)
	}
	fmt.Printf("%-6s %-30s %s  %s\n",
		methodColor.Sprint(method),
		path,
		statusColor.Sprintf("%d", status),
		color.New(color.Faint).Sprintf("%dms", duration.Milliseconds()),
	)
}

func printWSEvent(path, event string) {
	methodColor := color.New(color.FgMagenta, color.Bold)
	fmt.Printf("%-6s %-30s [%s]\n", methodColor.Sprint("WS"), path, event)
}
