package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// SocketModeConfig holds configuration for Socket Mode connection
type SocketModeConfig struct {
	AppToken string // xapp-* token
	BotToken string // xoxb-* token
}

// SocketModeConnection manages a WebSocket connection to Slack's Socket Mode
type SocketModeConnection struct {
	mu            sync.RWMutex
	conn          *websocket.Conn
	config        SocketModeConfig
	logger        *slog.Logger
	ctx           context.Context
	cancel        context.CancelFunc
	reconnects    int
	maxReconnects int
	connected     bool
	handlers      map[string]EventHandler
}

// EventHandler handles incoming Slack events
type EventHandler func(eventType string, data json.RawMessage)

// SocketModeURL is the Slack Socket Mode WebSocket endpoint
const SocketModeURL = "wss://wss.slack.com/ws"

// NewSocketModeConnection creates a new Socket Mode connection
func NewSocketModeConnection(config SocketModeConfig, logger *slog.Logger) *SocketModeConnection {
	ctx, cancel := context.WithCancel(context.Background())
	return &SocketModeConnection{
		config:        config,
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		reconnects:    0,
		maxReconnects: 5,
		handlers:      make(map[string]EventHandler),
	}
}

// RegisterHandler registers an event handler for a specific event type
func (s *SocketModeConnection) RegisterHandler(eventType string, handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[eventType] = handler
}

// Start begins the Socket Mode connection
func (s *SocketModeConnection) Start(ctx context.Context) error {
	s.mu.Lock()
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	return s.connect()
}

// Stop closes the Socket Mode connection
func (s *SocketModeConnection) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	if s.conn != nil {
		return s.conn.Close()
	}

	return nil
}

// IsConnected returns true if the connection is active
func (s *SocketModeConnection) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// connect establishes a WebSocket connection to Slack
func (s *SocketModeConnection) connect() error {
	s.logger.Info("Connecting to Slack Socket Mode", "url", SocketModeURL)

	header := http.Header{}
	header.Set("Authorization", "Bearer "+s.config.AppToken)
	header.Set("X-Slack-User", "bot")

	conn, _, err := websocket.DefaultDialer.Dial(SocketModeURL, header)
	if err != nil {
		s.logger.Error("Failed to connect to Slack", "error", err)
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	s.mu.Lock()
	s.conn = conn
	s.connected = true
	s.reconnects = 0
	s.mu.Unlock()

	s.logger.Info("Connected to Slack Socket Mode")

	// Start read loop
	go s.readLoop()

	return nil
}

// reconnect attempts to reconnect with exponential backoff
func (s *SocketModeConnection) reconnect() {
	s.mu.Lock()
	s.reconnects++
	reconnectCount := s.reconnects
	s.mu.Unlock()

	if reconnectCount > s.maxReconnects {
		s.logger.Error("Max reconnection attempts reached")
		return
	}

	// Exponential backoff: 1s, 2s, 4s, 8s, 16s
	delay := time.Duration(1<<uint(reconnectCount-1)) * time.Second
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}

	s.logger.Info("Attempting to reconnect", "attempt", reconnectCount, "delay", delay)

	select {
	case <-s.ctx.Done():
		return
	case <-time.After(delay):
	}

	if err := s.connect(); err != nil {
		s.logger.Error("Reconnection failed", "error", err)
	}
}

// readLoop continuously reads messages from the WebSocket
func (s *SocketModeConnection) readLoop() {
	defer func() {
		s.mu.Lock()
		s.connected = false
		s.conn = nil // Clear the connection on exit
		s.mu.Unlock()

		s.logger.Info("WebSocket read loop stopped")
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		_, message, err := s.conn.ReadMessage()
		if err != nil {
			s.logger.Error("Error reading message", "error", err)

			s.mu.RLock()
			connected := s.connected
			s.mu.RUnlock()

			if connected {
				// Connection lost, attempt reconnect
				go s.reconnect()
			}
			return
		}

		s.handleMessage(message)
	}
}

// handleMessage processes incoming WebSocket messages
func (s *SocketModeConnection) handleMessage(data []byte) {
	var msg struct {
		Type string          `json:"type"`
		Body json.RawMessage `json:"body,omitempty"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		s.logger.Error("Failed to parse message", "error", err)
		return
	}

	switch msg.Type {
	case "hello":
		s.logger.Info("Received hello from Slack")

	case "disconnect":
		s.logger.Warn("Received disconnect from Slack")
		s.mu.Lock()
		s.connected = false
		s.mu.Unlock()
		go s.reconnect()

	case "event_callback":
		s.handleEventCallback(msg.Body)

	case "ping":
		_ = s.sendPong()

	case "pong":
		// Keep-alive acknowledged

	default:
		s.logger.Debug("Unknown message type", "type", msg.Type)
	}
}

// handleEventCallback processes event_callback messages
func (s *SocketModeConnection) handleEventCallback(body json.RawMessage) {
	var event struct {
		Type   string          `json:"type"`
		Event  json.RawMessage `json:"event,omitempty"`
		Hidden bool            `json:"hidden,omitempty"`
	}

	if err := json.Unmarshal(body, &event); err != nil {
		s.logger.Error("Failed to parse event callback", "error", err)
		return
	}

	if event.Event == nil {
		return
	}

	// Call registered handler if exists
	s.mu.RLock()
	handler, exists := s.handlers[event.Type]
	s.mu.RUnlock()

	if exists && !event.Hidden {
		handler(event.Type, event.Event)
	}
}

// sendPong sends a pong response to keep the connection alive
func (s *SocketModeConnection) sendPong() error {
	pong := map[string]string{
		"type": "pong",
	}

	data, err := json.Marshal(pong)
	if err != nil {
		return err
	}

	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn != nil {
		return conn.WriteMessage(websocket.TextMessage, data)
	}

	return nil
}

// Send sends a message over the WebSocket connection
func (s *SocketModeConnection) Send(data map[string]any) error {
	s.mu.RLock()
	conn := s.conn
	s.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	msg, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return conn.WriteMessage(websocket.TextMessage, msg)
}
