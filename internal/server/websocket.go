package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/hrygo/hotplex"
)

// ClientRequest represents the JSON payload expected from the WebSocket client.
type ClientRequest struct {
	RequestID    int    `json:"request_id,omitempty"`    // Optional request ID for request-response correlation
	Type         string `json:"type"`                    // "execute", "stop", "stats", "version"
	SessionID    string `json:"session_id"`              // Provide session_id to hot-multiplex
	Prompt       string `json:"prompt,omitempty"`        // The user input (for "execute")
	SystemPrompt string `json:"system_prompt,omitempty"` // Override system prompt (for "execute")
	WorkDir      string `json:"work_dir,omitempty"`      // Working directory for CLI (for "execute")
	Reason       string `json:"reason,omitempty"`        // Reason for stopping (for "stop")
}

// ServerResponse represents the JSON payload sent to the WebSocket client.
type ServerResponse struct {
	RequestID int    `json:"request_id,omitempty"` // Echo back request_id for correlation
	Event     string `json:"event"`
	Data      any    `json:"data"`
}

// connWriter wraps a websocket.Conn with a mutex to prevent concurrent writes.
// gorilla/websocket does NOT support concurrent WriteMessage calls;
// this wrapper serializes all writes to the connection.
type connWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (cw *connWriter) writeJSON(event string, data any, requestID int) {
	resp := ServerResponse{Event: event, Data: data}
	if requestID > 0 {
		resp.RequestID = requestID
	}
	val, err := json.Marshal(resp)
	if err != nil {
		// This should rarely happen with our internal types, but log it for enterprise observability
		return
	}
	cw.mu.Lock()
	defer cw.mu.Unlock()
	_ = cw.conn.WriteMessage(websocket.TextMessage, val)
}

// WebSocketHandler manages a WebSocket connection to a HotPlex Engine.
type WebSocketHandler struct {
	engine   hotplex.HotPlexClient
	logger   *slog.Logger
	cors     *CORSConfig
	upgrader websocket.Upgrader
}

// NewWebSocketHandler creates a new handler with CORS configuration.
func NewWebSocketHandler(engine hotplex.HotPlexClient, logger *slog.Logger, cors *CORSConfig) *WebSocketHandler {
	return &WebSocketHandler{
		engine: engine,
		logger: logger,
		cors:   cors,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     cors.CheckOrigin(),
		},
	}
}

// ServeHTTP upgrades the HTTP connection and starts the read loop.
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade websocket connection", "error", err)
		return
	}

	// Create a top-level context for the entire connection lifecycle
	connCtx, connCancel := context.WithCancel(r.Context())
	defer connCancel() // Cancels all child tasks when connection closes

	defer func() { _ = conn.Close() }()

	cw := &connWriter{conn: conn}

	// Track active tasks for this specific connection to allow concurrent 'stop'
	tasks := make(map[string]context.CancelFunc)
	var mu sync.Mutex

	h.logger.Info("Client connected via WebSocket", "addr", r.RemoteAddr)

	for {
		// Read message from client
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("WebSocket closed unexpectedly", "error", err)
			} else {
				h.logger.Info("WebSocket closed normally", "addr", r.RemoteAddr)
			}
			return
		}

		if messageType != websocket.TextMessage {
			h.logger.Warn("Ignoring non-text message type", "type", messageType)
			continue
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			cw.writeJSON("error", map[string]string{"message": "Invalid JSON payload: " + err.Error()}, 0)
			continue
		}

		switch req.Type {
		case "execute":
			// Process execution asynchronously to keep the read loop open
			go h.handleExecute(connCtx, cw, req, tasks, &mu)
		case "stop":
			h.handleStop(cw, req, tasks, &mu)
		case "stats":
			h.handleStats(cw, req)
		case "version":
			h.handleVersion(cw, req)
		default:
			cw.writeJSON("error", map[string]string{"message": "Unknown request type: " + req.Type}, req.RequestID)
		}
	}
}

func (h *WebSocketHandler) handleExecute(connCtx context.Context, cw *connWriter, req ClientRequest, tasks map[string]context.CancelFunc, mu *sync.Mutex) {
	if req.Prompt == "" {
		cw.writeJSON("error", map[string]string{"message": "prompt cannot be empty"}, req.RequestID)
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Create task-specific context linked to the connection lifecycle
	taskCtx, taskCancel := context.WithTimeout(connCtx, 10*time.Minute)

	mu.Lock()
	if oldCancel, exists := tasks[sessionID]; exists {
		oldCancel() // Cancel previous execution if same sessionID is re-used concurrently
	}
	tasks[sessionID] = taskCancel
	mu.Unlock()

	defer func() {
		mu.Lock()
		delete(tasks, sessionID)
		mu.Unlock()
		taskCancel()
	}()

	workDir := req.WorkDir
	if workDir == "" {
		workDir = "/tmp/hotplex_sandbox"
	}

	cfg := &hotplex.Config{
		WorkDir:          workDir,
		SessionID:        sessionID,
		TaskSystemPrompt: req.SystemPrompt,
	}

	h.logger.Info("Async execute start", "session_id", sessionID)

	// Capture request_id for use in callback
	requestID := req.RequestID
	cb := func(eventType string, data any) error {
		cw.writeJSON(eventType, data, requestID)
		return nil
	}

	err := h.engine.Execute(taskCtx, cfg, req.Prompt, cb)
	if err != nil {
		// Only send error if it wasn't a normal cancellation/stop
		if taskCtx.Err() == nil {
			cw.writeJSON("error", map[string]string{"message": "Execution failed: " + err.Error()}, requestID)
		}
		return
	}

	stats := h.engine.GetSessionStats()
	cw.writeJSON("completed", map[string]any{
		"session_id": sessionID,
		"stats":      stats,
	}, requestID)
}

func (h *WebSocketHandler) handleStop(cw *connWriter, req ClientRequest, tasks map[string]context.CancelFunc, mu *sync.Mutex) {
	if req.SessionID == "" {
		cw.writeJSON("error", map[string]string{"message": "session_id is required for stop"}, req.RequestID)
		return
	}

	// 1. Locally cancel the execution context to unblock the goroutine immediately
	mu.Lock()
	if cancel, exists := tasks[req.SessionID]; exists {
		cancel()
		delete(tasks, req.SessionID)
	}
	mu.Unlock()

	// 2. Instruct the engine to stop the underlying session (process cleanup)
	reason := req.Reason
	if reason == "" {
		reason = "client_request:manual_stop"
	}

	h.logger.Info("Stop request", "session_id", req.SessionID, "reason", reason)
	if err := h.engine.StopSession(req.SessionID, reason); err != nil {
		cw.writeJSON("error", map[string]string{"message": "Stop failed: " + err.Error()}, req.RequestID)
		return
	}

	cw.writeJSON("stopped", map[string]string{"session_id": req.SessionID}, req.RequestID)
}

func (h *WebSocketHandler) handleStats(cw *connWriter, req ClientRequest) {
	stats := h.engine.GetSessionStats()
	cw.writeJSON("stats", stats, req.RequestID)
}

func (h *WebSocketHandler) handleVersion(cw *connWriter, req ClientRequest) {
	version, err := h.engine.GetCLIVersion()
	if err != nil {
		cw.writeJSON("error", map[string]string{"message": "Failed to get version: " + err.Error()}, req.RequestID)
		return
	}
	cw.writeJSON("version", map[string]string{"version": version}, req.RequestID)
}
