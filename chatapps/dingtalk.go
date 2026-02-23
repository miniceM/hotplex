package chatapps

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

type DingTalkConfig struct {
	AppID         string
	AppSecret     string
	CallbackURL   string
	CallbackToken string
	CallbackKey   string
	ServerAddr    string
}

type DingTalkCallbackRequest struct {
	MsgType        string `json:"msgtype"`
	ConversationID string `json:"conversationId"`
	SenderID       string `json:"senderId"`
	SenderNick     string `json:"senderNick"`
	IsAdmin        bool   `json:"isAdmin"`
	RobotCode      string `json:"robotCode"`
	Text           struct {
		Content string `json:"content"`
	} `json:"text"`
	EventType string `json:"eventType"`
}

type DingTalkCallbackResponse struct {
	MsgType string `json:"msgtype"`
	Text    struct {
		Content string `json:"content"`
	} `json:"text"`
}

type DingTalkAdapter struct {
	config   DingTalkConfig
	logger   *slog.Logger
	server   *http.Server
	sessions map[string]*DingTalkSession
	mu       sync.RWMutex
	handler  MessageHandler
	running  bool
}

type DingTalkSession struct {
	SessionID  string
	UserID     string
	Platform   string
	LastActive time.Time
}

func NewDingTalkAdapter(config DingTalkConfig, logger *slog.Logger) *DingTalkAdapter {
	if config.ServerAddr == "" {
		config.ServerAddr = ":8080"
	}
	return &DingTalkAdapter{
		config:   config,
		logger:   logger,
		sessions: make(map[string]*DingTalkSession),
	}
}

func (a *DingTalkAdapter) Platform() string {
	return "dingtalk"
}

func (a *DingTalkAdapter) Start(ctx context.Context) error {
	if a.running {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", a.handleCallback)
	mux.HandleFunc("/health", a.handleHealth)

	a.server = &http.Server{
		Addr:    a.config.ServerAddr,
		Handler: mux,
	}

	go func() {
		a.logger.Info("Starting DingTalk adapter", "addr", a.config.ServerAddr)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("DingTalk server error", "error", err)
		}
	}()

	a.running = true
	return nil
}

func (a *DingTalkAdapter) Stop() error {
	if !a.running {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	a.running = false
	a.logger.Info("DingTalk adapter stopped")
	return nil
}

func (a *DingTalkAdapter) SetHandler(handler MessageHandler) {
	a.handler = handler
}

func (a *DingTalkAdapter) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		a.handleCallbackVerify(w, r)
		return
	}

	if r.Method == "POST" {
		a.handleCallbackMessage(w, r)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (a *DingTalkAdapter) handleCallbackVerify(w http.ResponseWriter, r *http.Request) {
	signature := r.URL.Query().Get("signature")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")

	if a.config.CallbackToken == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if !a.verifySignature(signature, timestamp, nonce, a.config.CallbackToken) {
		a.logger.Warn("Invalid callback signature")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, timestamp)
}

func (a *DingTalkAdapter) handleCallbackMessage(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.logger.Error("Read body failed", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var callback DingTalkCallbackRequest
	if err := json.Unmarshal(body, &callback); err != nil {
		a.logger.Error("Parse callback failed", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if callback.MsgType != "text" {
		a.logger.Debug("Ignoring non-text message", "type", callback.MsgType)
		w.WriteHeader(http.StatusOK)
		return
	}

	sessionID := a.getOrCreateSession(callback.SenderID, callback.ConversationID)

	msg := &ChatMessage{
		Platform:  "dingtalk",
		SessionID: sessionID,
		UserID:    callback.SenderID,
		Content:   callback.Text.Content,
		MessageID: callback.ConversationID + ":" + callback.SenderID,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"conversation_id": callback.ConversationID,
			"sender_nick":     callback.SenderNick,
			"robot_code":      callback.RobotCode,
		},
	}

	if a.handler != nil {
		go func() {
			if err := a.handler(context.Background(), msg); err != nil {
				a.logger.Error("Handle message failed", "error", err)
			}
		}()
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"msgtype":"text","text":{"content":"收到消息，正在处理..."}}`))
}

func (a *DingTalkAdapter) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func (a *DingTalkAdapter) SendMessage(ctx context.Context, sessionID string, msg *ChatMessage) error {
	conversationID, ok := msg.Metadata["conversation_id"].(string)
	if !ok || conversationID == "" {
		return fmt.Errorf("conversation_id not found in metadata")
	}

	payload := map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": msg.Content,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	accessToken, err := a.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	url := fmt.Sprintf("https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend?robotCode=%s", msg.Metadata["robot_code"])
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send failed: %d %s", resp.StatusCode, string(respBody))
	}

	a.logger.Debug("Message sent", "session", sessionID)
	return nil
}

func (a *DingTalkAdapter) HandleMessage(ctx context.Context, msg *ChatMessage) error {
	return nil
}

func (a *DingTalkAdapter) getOrCreateSession(userID, conversationID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := conversationID + ":" + userID
	if session, ok := a.sessions[key]; ok {
		session.LastActive = time.Now()
		return session.SessionID
	}

	session := &DingTalkSession{
		SessionID:  fmt.Sprintf("dt-%d", time.Now().UnixNano()),
		UserID:     userID,
		Platform:   "dingtalk",
		LastActive: time.Now(),
	}
	a.sessions[key] = session

	a.logger.Info("New session created", "session", session.SessionID, "user", userID)
	return session.SessionID
}

func (a *DingTalkAdapter) getAccessToken() (string, error) {
	if a.config.AppID == "" || a.config.AppSecret == "" {
		return "", nil
	}

	url := fmt.Sprintf("https://api.dingtalk.com/v1.0/oauth2/oAuth2/accessToken?appKey=%s&appSecret=%s",
		a.config.AppID, a.config.AppSecret)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.AccessToken, nil
}

func (a *DingTalkAdapter) verifySignature(signature, timestamp, nonce, token string) bool {
	stringToSign := timestamp + token + nonce
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return sign == signature
}
