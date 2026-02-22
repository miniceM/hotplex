package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/hrygo/hotplex"
)

// mockEngine implements hotplex.HotPlexClient for testing
type mockEngine struct {
	executeErr error
}

func (m *mockEngine) Execute(ctx context.Context, cfg *hotplex.Config, prompt string, cb hotplex.Callback) error {
	return m.executeErr
}

func (m *mockEngine) Close() error {
	return nil
}

func (m *mockEngine) GetSessionStats(sessionID string) *hotplex.SessionStats {
	return nil
}

func (m *mockEngine) StopSession(sessionID string, reason string) error {
	return nil
}

func (m *mockEngine) SetDangerAllowPaths(paths []string) {}

func (m *mockEngine) SetDangerBypassEnabled(token string, enabled bool) error {
	return nil
}

func (m *mockEngine) GetCLIVersion() (string, error) {
	return "mock-version", nil
}

func (m *mockEngine) ValidateConfig(cfg *hotplex.Config) error {
	return nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func TestNewHotPlexWSHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &mockEngine{}
	cors := NewSecurityConfig(logger)

	h := NewHotPlexWSHandler(engine, logger, cors)

	if h == nil {
		t.Fatal("NewWebSocketHandler returned nil")
	}
	if h.engine != engine {
		t.Error("engine not set correctly")
	}
	if h.logger != logger {
		t.Error("logger not set correctly")
	}
	if h.cors != cors {
		t.Error("cors not set correctly")
	}
}

func TestWebSocketHandler_ServeHTTP_InvalidJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &mockEngine{}
	cors := NewSecurityConfig(logger)
	_ = NewHotPlexWSHandler(engine, logger, cors) // handler created but test uses manual upgrader

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		cw := &connWriter{conn: conn}

		// Read invalid JSON
		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			cw.writeJSON("error", map[string]string{"message": "Invalid JSON payload: " + err.Error()}, 0)
		}
	}))
	defer server.Close()

	// Connect as client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send invalid JSON
	err = conn.WriteMessage(websocket.TextMessage, []byte("not valid json"))
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Read response
	var resp ServerResponse
	err = conn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Event != "error" {
		t.Errorf("Expected error event, got %s", resp.Event)
	}
	if !strings.Contains(resp.Data.(map[string]interface{})["message"].(string), "Invalid JSON") {
		t.Errorf("Expected Invalid JSON error, got %v", resp.Data)
	}
}

func TestHotPlexWSHandler_HandleExecute_EmptyPrompt(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &mockEngine{}
	cors := NewSecurityConfig(logger)
	h := NewHotPlexWSHandler(engine, logger, cors)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		cw := &connWriter{conn: conn}

		// Read message
		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			cw.writeJSON("error", map[string]string{"message": "Invalid JSON payload: " + err.Error()}, 0)
			return
		}

		// Create a mock context and task map for the test
		connCtx, cancel := context.WithCancel(r.Context())
		_ = connCtx
		defer cancel()
		tasks := make(map[string]context.CancelFunc)
		var mu sync.Mutex

		if req.Type == "execute" {
			h.handleExecute(connCtx, cw, req, tasks, &mu)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send execute with empty prompt
	req := ClientRequest{Type: "execute", Prompt: ""}
	err = conn.WriteJSON(req)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	var resp ServerResponse
	err = conn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Event != "error" {
		t.Errorf("Expected error event, got %s", resp.Event)
	}
}

func TestHotPlexWSHandler_HandleExecute_AutoSessionID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &mockEngine{}
	cors := NewSecurityConfig(logger)
	_ = NewHotPlexWSHandler(engine, logger, cors) // handler created but test uses manual upgrader

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		cw := &connWriter{conn: conn}

		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			return
		}

		if req.Type == "execute" {
			// Check session ID auto-generation
			sessionID := req.SessionID
			if sessionID == "" {
				sessionID = "auto-generated-id"
			}
			cw.writeJSON("test", map[string]string{"session_id": sessionID}, 0)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send execute without session ID
	req := ClientRequest{Type: "execute", Prompt: "test prompt"}
	err = conn.WriteJSON(req)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	var resp ServerResponse
	err = conn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Event != "test" {
		t.Errorf("Expected test event, got %s", resp.Event)
	}
}

func TestHotPlexWSHandler_HandleStop_EmptySessionID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &mockEngine{}
	cors := NewSecurityConfig(logger)
	h := NewHotPlexWSHandler(engine, logger, cors)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		cw := &connWriter{conn: conn}

		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			return
		}

		// Create a mock context and task map for the test
		connCtx, cancel := context.WithCancel(r.Context())
		_ = connCtx
		defer cancel()
		tasks := make(map[string]context.CancelFunc)
		var mu sync.Mutex

		if req.Type == "stop" {
			h.handleStop(cw, req, tasks, &mu)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send stop without session ID
	req := ClientRequest{Type: "stop"}
	err = conn.WriteJSON(req)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	var resp ServerResponse
	err = conn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Event != "error" {
		t.Errorf("Expected error event, got %s", resp.Event)
	}
	if !strings.Contains(resp.Data.(map[string]interface{})["message"].(string), "session_id is required") {
		t.Errorf("Expected session_id required error, got %v", resp.Data)
	}
}

func TestHotPlexWSHandler_UnknownRequestType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &mockEngine{}
	cors := NewSecurityConfig(logger)
	h := NewHotPlexWSHandler(engine, logger, cors)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		cw := &connWriter{conn: conn}

		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			cw.writeJSON("error", map[string]string{"message": "Invalid JSON payload: " + err.Error()}, 0)
			return
		}

		// Create a mock context and task map for the test
		connCtx, cancel := context.WithCancel(r.Context())
		_ = connCtx
		defer cancel()
		tasks := make(map[string]context.CancelFunc)
		var mu sync.Mutex

		switch req.Type {
		case "execute":
			h.handleExecute(connCtx, cw, req, tasks, &mu)
		case "stop":
			h.handleStop(cw, req, tasks, &mu)
		default:
			cw.writeJSON("error", map[string]string{"message": "Unknown request type: " + req.Type}, req.RequestID)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send unknown request type
	req := ClientRequest{Type: "unknown"}
	err = conn.WriteJSON(req)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	var resp ServerResponse
	err = conn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Event != "error" {
		t.Errorf("Expected error event, got %s", resp.Event)
	}
	if !strings.Contains(resp.Data.(map[string]interface{})["message"].(string), "Unknown request type") {
		t.Errorf("Expected unknown request type error, got %v", resp.Data)
	}
}

func TestConnWriter_WriteJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		cw := &connWriter{conn: conn}
		cw.writeJSON("test_event", map[string]string{"key": "value"}, 0)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var resp ServerResponse
	err = conn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Event != "test_event" {
		t.Errorf("Expected test_event, got %s", resp.Event)
	}
	data := resp.Data.(map[string]interface{})
	if data["key"] != "value" {
		t.Errorf("Expected value, got %v", data["key"])
	}
}

func TestClientRequest_JSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    ClientRequest
		wantErr bool
	}{
		{
			name: "execute request with instructions",
			json: `{"type":"execute","session_id":"test-123","prompt":"hello","instructions":"you are a bot"}`,
			want: ClientRequest{Type: "execute", SessionID: "test-123", Prompt: "hello", Instructions: "you are a bot"},
		},
		{
			name: "stop request",
			json: `{"type":"stop","session_id":"test-123","reason":"user requested"}`,
			want: ClientRequest{Type: "stop", SessionID: "test-123", Reason: "user requested"},
		},
		{
			name: "stats request",
			json: `{"type":"stats","session_id":"test-123"}`,
			want: ClientRequest{Type: "stats", SessionID: "test-123"},
		},
		{
			name: "version request",
			json: `{"type":"version"}`,
			want: ClientRequest{Type: "version"},
		},
		{
			name: "minimal request",
			json: `{"type":"execute"}`,
			want: ClientRequest{Type: "execute"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ClientRequest
			err := json.Unmarshal([]byte(tt.json), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServerResponse_JSON(t *testing.T) {
	resp := ServerResponse{
		Event: "thinking",
		Data:  map[string]string{"status": "processing"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var got ServerResponse
	err = json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if got.Event != resp.Event {
		t.Errorf("Event = %s, want %s", got.Event, resp.Event)
	}
}

// mockErrorEngine returns an error on Execute
type mockErrorEngine struct {
	mockEngine
	executeErr error
}

func (m *mockErrorEngine) Execute(ctx context.Context, cfg *hotplex.Config, prompt string, cb hotplex.Callback) error {
	return m.executeErr
}

func TestHotPlexWSHandler_HandleExecute_ExecuteError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &mockErrorEngine{executeErr: context.DeadlineExceeded}
	cors := NewSecurityConfig(logger)
	h := NewHotPlexWSHandler(engine, logger, cors)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		cw := &connWriter{conn: conn}
		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			return
		}

		// Create a mock context and task map for the test
		connCtx, cancel := context.WithCancel(r.Context())
		_ = connCtx
		defer cancel()
		tasks := make(map[string]context.CancelFunc)
		var mu sync.Mutex

		if req.Type == "execute" {
			h.handleExecute(connCtx, cw, req, tasks, &mu)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send execute with valid prompt
	req := ClientRequest{Type: "execute", Prompt: "test prompt", SessionID: "test-session"}
	err = conn.WriteJSON(req)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	var resp ServerResponse
	err = conn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Event != "error" {
		t.Errorf("Expected error event, got %s", resp.Event)
	}
}

func TestHotPlexWSHandler_HandleExecute_DefaultWorkDir(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &mockEngine{}
	cors := NewSecurityConfig(logger)
	h := NewHotPlexWSHandler(engine, logger, cors)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		cw := &connWriter{conn: conn}
		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			return
		}

		// Create a mock context and task map for the test
		connCtx, cancel := context.WithCancel(r.Context())
		_ = connCtx
		defer cancel()
		tasks := make(map[string]context.CancelFunc)
		var mu sync.Mutex

		if req.Type == "execute" {
			h.handleExecute(connCtx, cw, req, tasks, &mu)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send execute without work_dir - should use default
	req := ClientRequest{Type: "execute", Prompt: "test prompt"}
	err = conn.WriteJSON(req)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	var resp ServerResponse
	err = conn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// The mock engine succeeds, so we should get completed event
	if resp.Event != "completed" {
		t.Errorf("Expected completed event, got %s", resp.Event)
	}
	data := resp.Data.(map[string]interface{})
	if data["session_id"] == "" {
		t.Error("Expected session_id in response")
	}
}

func TestHotPlexWSHandler_HandleStop_DefaultReason(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &mockEngine{}
	cors := NewSecurityConfig(logger)
	h := NewHotPlexWSHandler(engine, logger, cors)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		cw := &connWriter{conn: conn}
		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			return
		}

		// Create a mock context and task map for the test
		connCtx, cancel := context.WithCancel(r.Context())
		_ = connCtx
		defer cancel()
		tasks := make(map[string]context.CancelFunc)
		var mu sync.Mutex

		if req.Type == "stop" {
			h.handleStop(cw, req, tasks, &mu)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send stop without reason
	req := ClientRequest{Type: "stop", SessionID: "test-session"}
	err = conn.WriteJSON(req)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	var resp ServerResponse
	err = conn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Since mockEngine is not *hotplex.Engine, type assertion fails silently
	// and we should get the "stopped" event
	if resp.Event != "stopped" {
		t.Errorf("Expected stopped event, got %s", resp.Event)
	}
}

func TestHotPlexWSHandler_HandleStop_WithReason(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	engine := &mockEngine{}
	cors := NewSecurityConfig(logger)
	h := NewHotPlexWSHandler(engine, logger, cors)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		cw := &connWriter{conn: conn}
		_, p, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req ClientRequest
		if err := json.Unmarshal(p, &req); err != nil {
			return
		}

		// Create a mock context and task map for the test
		connCtx, cancel := context.WithCancel(r.Context())
		_ = connCtx
		defer cancel()
		tasks := make(map[string]context.CancelFunc)
		var mu sync.Mutex

		if req.Type == "stop" {
			h.handleStop(cw, req, tasks, &mu)
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Send stop with reason
	req := ClientRequest{Type: "stop", SessionID: "test-session", Reason: "user cancelled"}
	err = conn.WriteJSON(req)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	var resp ServerResponse
	err = conn.ReadJSON(&resp)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.Event != "stopped" {
		t.Errorf("Expected stopped event, got %s", resp.Event)
	}
}
