# HotPlex ChatApps 代码审查与修复方案

> **文档版本**: 1.0  
> **创建日期**: 2026-02-26  
> **审查范围**: `chatapps/` 层 (Slack, Telegram, Discord 等社交平台集成)  
> **审查标准**: Slack 官方 SDK 最佳实践、OpenClaw 参考实现

---

## 📋 执行摘要

本次审查对 HotPlex `chatapps/` 层进行了全面代码审计，共发现 **13 个问题**，按严重程度分类如下：

| 严重级别 | 数量 | 修复优先级 | 建议完成时间 |
|---------|------|-----------|-------------|
| **CRITICAL** | 2 | P0 | 1 周内 |
| **HIGH** | 4 | P1 | 2 周内 |
| **MEDIUM** | 4 | P2 | 1 个月内 |
| **LOW** | 3 | P3 | 2 个月内 |

**生产就绪度评估**: ⚠️ **需要修复 CRITICAL 和 HIGH 问题后方可上线**

---

## 🎯 修复阶段总览

| 阶段 | 时间估算 | 修复问题 | 风险等级 | 负责人 |
|------|---------|---------|---------|--------|
| **Phase 1** | 2-3 天 | CRITICAL #1, #2 | 🔴 高 | 核心开发 |
| **Phase 2** | 3-4 天 | HIGH #3, #4, #5, #6 | 🟠 高 | 核心开发 |
| **Phase 3** | 2-3 天 | MEDIUM #7, #8, #9, #10 | 🟡 中 | 一般开发 |
| **Phase 4** | 1-2 天 | LOW #11, #12, #13 | 🟢 低 | 一般开发 |
| **总计** | **8-12 天** | 13 个问题 | - | - |

---

## Phase 1: CRITICAL 问题修复 (P0)

### 🔴 问题 #1: Socket Mode ACK 格式错误

**文件**: `chatapps/slack/socket_mode.go:442-464`

**问题描述**: 

ACK 发送格式不符合 Slack Socket Mode 协议要求。当前实现发送 `{"envelope_id": "xxx"}`，但 Slack 对于 `events_api` 和 `interactive` 类型事件期望完整的响应对象。

**Slack 官方要求**:

```json
{
  "envelope_id": "xxx",
  "payload": {...}  // 可选，仅在需要响应时
}
```

**影响**: 

- Slack 可能认为事件未被确认，导致事件重复投递
- 交互式组件（按钮点击等）可能无响应

**修复方案**:

```go
// chatapps/slack/socket_mode.go

// sendACK 发送纯 ACK (用于事件确认)
func (s *SocketModeConnection) sendACK(envelopeID string) error {
	ack := map[string]string{
		"envelope_id": envelopeID,
	}
	
	s.logger.Debug("Sending ACK", "envelope_id", envelopeID)
	return s.Send(ack)
}

// sendResponseACK 发送带响应的 ACK (用于 interactive 事件)
func (s *SocketModeConnection) sendResponseACK(envelopeID string, payload map[string]any) error {
	resp := map[string]any{
		"envelope_id": envelopeID,
		"payload":     payload,
	}
	
	s.logger.Debug("Sending response ACK", "envelope_id", envelopeID)
	return s.Send(resp)
}

// sendACKWithRetry 使用指数退避重试发送 ACK
func (s *SocketModeConnection) sendACKWithRetry(envelopeID string, maxRetries int, baseDelay time.Duration) error {
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1))
			s.logger.Debug("Retrying ACK send", 
				"envelope_id", envelopeID, 
				"attempt", attempt+1, 
				"maxRetries", maxRetries+1,
				"delay", delay)
			time.Sleep(delay)
		}
		
		if err := s.sendACK(envelopeID); err != nil {
			lastErr = err
			s.logger.Warn("ACK send attempt failed", 
				"envelope_id", envelopeID, 
				"attempt", attempt+1, 
				"error", err)
			continue
		}
		return nil // Success
	}
	
	return fmt.Errorf("ACK send failed after %d attempts: %w", maxRetries+1, lastErr)
}
```

**测试要求**:
- [ ] 单元测试：验证 ACK 格式符合 Slack 规范
- [ ] 集成测试：模拟 Slack 事件，验证 ACK 被正确接收
- [ ] 压力测试：连续发送 100 个事件，确认无 ACK 丢失

**验收标准**: Slack 事件无重复投递，交互式组件响应正常

---

### 🔴 问题 #2: 缺少 Token 过期错误处理

**文件**: `chatapps/slack/adapter.go:841-916`, `chatapps/slack/socket_mode.go:106-194`

**问题描述**: 

未处理 Slack API 认证错误（`invalid_auth`, `token_revoked`, `account_inactive`）。Token 过期或被撤销时，系统会无限重试而不是通知管理员。

**Slack API 错误响应**:

```json
{"ok": false, "error": "invalid_auth"}
{"ok": false, "error": "token_revoked"}
{"ok": false, "error": "account_inactive"}
```

**影响**: 

- Token 过期后无限重试，浪费资源
- 管理员无法及时获知认证失败
- 用户请求静默失败

**修复方案**:

```go
// chatapps/slack/adapter.go

// Slack API 错误类型常量
const (
	ErrInvalidAuth     = "invalid_auth"
	ErrTokenRevoked    = "token_revoked"
	ErrAccountInactive = "account_inactive"
	ErrAuthFailed      = "auth_failed"
)

// IsAuthError 判断是否为认证错误 (不可恢复)
func IsAuthError(errStr string) bool {
	switch errStr {
	case ErrInvalidAuth, ErrTokenRevoked, ErrAccountInactive, ErrAuthFailed:
		return true
	default:
		return false
	}
}

// sendToChannelOnce 修改错误处理部分
func (a *Adapter) sendToChannelOnce(ctx context.Context, channelID, text, threadTS string) error {
	// ... 现有请求构建代码 ...
	
	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &slackResp); err != nil {
		a.Logger().Warn("Failed to parse Slack response", "body", string(respBody))
		return nil
	}
	
	if !slackResp.OK {
		// 新增：检查是否为认证错误
		if IsAuthError(slackResp.Error) {
			a.Logger().Error("Authentication failure - requires token refresh",
				"error", slackResp.Error,
				"channel", channelID)
			// 返回特殊错误，调用方可据此触发告警
			return fmt.Errorf("auth_failed: %s", slackResp.Error)
		}
		return fmt.Errorf("slack API error: %s", slackResp.Error)
	}
	
	a.Logger().Debug("Message sent successfully", "channel", channelID)
	return nil
}

// retryWithBackoff 修改：认证错误不重试
func retryWithBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := config.BaseDelay * time.Duration(1<<uint(attempt-1))
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
			time.Sleep(delay)
		}
		
		err := fn()
		if err == nil {
			return nil
		}
		
		lastErr = err
		// 新增：认证错误立即返回，不重试
		if strings.Contains(err.Error(), "auth_failed") {
			return err
		}
	}
	
	return lastErr
}
```

```go
// chatapps/slack/socket_mode.go

// connect 添加认证错误处理
func (s *SocketModeConnection) connect() error {
	s.logger.Info("Opening Slack Socket Mode connection", "has_app_token", s.config.AppToken != "")
	
	wsURL, err := s.getWebSocketURL()
	if err != nil {
		// 检查是否为认证错误
		if strings.Contains(err.Error(), "invalid_auth") || 
		   strings.Contains(err.Error(), "token_revoked") {
			s.logger.Error("Authentication failure - cannot reconnect", "error", err)
			return fmt.Errorf("auth_failed: %w", err)
		}
		return fmt.Errorf("failed to get WebSocket URL: %w", err)
	}
	
	// ... 剩余连接逻辑 ...
}
```

**测试要求**:
- [ ] 单元测试：使用过期 token 模拟 API 调用，验证返回 `auth_failed` 错误
- [ ] 集成测试：模拟 Slack 返回 `invalid_auth`，验证不进入重试循环
- [ ] 告警测试：认证错误触发日志告警

**验收标准**: Token 过期时立即返回错误，不重试，记录错误日志

---

## Phase 2: HIGH 问题修复 (P1)

### 🟠 问题 #3: 消息发送无速率限制

**文件**: `chatapps/slack/adapter.go:841-916`, `chatapps/slack/rate_limiter.go`

**问题描述**: 

`SendToChannel` 实现了重试逻辑，但没有速率限制。Slack 限制每个频道约 1 条消息/秒，违反限制会返回 HTTP 429。

**Slack 速率限制**:
- 每个频道：~1 消息/秒
- 每个工作区：数百消息/分钟
- 违反限制返回 HTTP 429

**影响**: 

- 高频消息（如流式思考内容更新）触发速率限制
- 消息丢失或延迟
- 可能被 Slack 临时封禁

**修复方案**:

```go
// chatapps/slack/adapter.go
import "golang.org/x/time/rate"

type Adapter struct {
	// ... 现有字段 ...
	msgRateLimiters map[string]*rate.Limiter  // 每个频道一个限制器
	rateMu          sync.RWMutex              // 保护 rate limiters map
	rateLimit       rate.Limit                // 每秒消息数
	rateBurst       int                       // 突发容量
}

// NewAdapter 初始化速率限制器
func NewAdapter(config *Config, logger *slog.Logger, opts ...base.AdapterOption) *Adapter {
	// ... 现有初始化代码 ...
	
	a := &Adapter{
		// ...
		msgRateLimiters: make(map[string]*rate.Limiter),
		rateLimit:       rate.Every(time.Second), // 1 消息/秒
		rateBurst:       5,                        // 允许 5 条突发
	}
	
	// ...
	return a
}

// getRateLimiter 获取或创建频道的速率限制器
func (a *Adapter) getRateLimiter(channelID string) *rate.Limiter {
	a.rateMu.RLock()
	limiter, exists := a.msgRateLimiters[channelID]
	a.rateMu.RUnlock()
	
	if exists {
		return limiter
	}
	
	a.rateMu.Lock()
	defer a.rateMu.Unlock()
	
	// 双重检查
	if limiter, exists = a.msgRateLimiters[channelID]; exists {
		return limiter
	}
	
	limiter = rate.NewLimiter(a.rateLimit, a.rateBurst)
	a.msgRateLimiters[channelID] = limiter
	return limiter
}

// SendToChannel 添加速率限制
func (a *Adapter) SendToChannel(ctx context.Context, channelID, text, threadTS string) error {
	// 获取频道速率限制器
	limiter := a.getRateLimiter(channelID)
	
	// 等待令牌 (带超时)
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	if err := limiter.Wait(waitCtx); err != nil {
		a.Logger().Warn("Rate limit wait failed", 
			"channel", channelID, "error", err)
		// 超时也尝试发送，Slack 会返回 429
	}
	
	retryConfig := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    5 * time.Second,
	}
	
	return retryWithBackoff(ctx, retryConfig, func() error {
		return a.sendToChannelOnce(ctx, channelID, text, threadTS)
	})
}
```

**测试要求**:
- [ ] 单元测试：1 秒内发送 10 条消息，验证前 5 条立即发送，后续等待
- [ ] 压力测试：持续发送 100 条消息，验证无 429 错误
- [ ] 多频道测试：同时向 3 个频道发送，验证独立限速

**验收标准**: 单频道消息发送速率 ≤ 1 条/秒，无 429 错误

---

### 🟠 问题 #4: Socket Mode 连接状态竞态条件

**文件**: `chatapps/slack/socket_mode.go:252-286` (readLoop)

**问题描述**: 

`readLoop` defer 中修改 `s.connected` 和 `s.conn` 时没有持有锁，可能导致并发访问问题。在重连过程中，可能导致 nil pointer dereference。

**影响**: 

- 极端情况下 panic
- 连接状态不一致
- 重连逻辑失效

**修复方案**:

```go
// chatapps/slack/socket_mode.go

// readLoop 修复竞态条件
func (s *SocketModeConnection) readLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		
		// 安全获取连接引用
		s.mu.RLock()
		conn := s.conn
		connected := s.connected
		s.mu.RUnlock()
		
		// 连接已关闭，退出
		if !connected || conn == nil {
			return
		}
		
		_, message, err := conn.ReadMessage()
		if err != nil {
			s.logger.Error("Error reading message", "error", err)
			
			// 仅在仍连接时触发重连
			s.mu.RLock()
			stillConnected := s.connected
			s.mu.RUnlock()
			
			if stillConnected {
				s.logger.Info("Connection lost, attempting reconnect")
				go s.reconnect()
			}
			return
		}
		
		s.handleMessage(message)
	}
}

// 清理 defer
defer func() {
	s.mu.Lock()
	s.connected = false
	s.conn = nil
	s.mu.Unlock()
	s.logger.Info("WebSocket read loop stopped")
}()

// Send 修复竞态条件
func (s *SocketModeConnection) Send(data map[string]any) error {
	s.mu.RLock()
	conn := s.conn
	connected := s.connected
	s.mu.RUnlock()
	
	if !connected || conn == nil {
		return fmt.Errorf("not connected")
	}
	
	msg, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	return conn.WriteMessage(websocket.TextMessage, msg)
}
```

**测试要求**:
- [ ] 并发测试：启动 10 个 goroutine 同时调用 Send，验证无 panic
- [ ] 断线测试：在 readLoop 运行时关闭连接，验证优雅退出
- [ ] 重连测试：模拟断线，验证重连期间无竞态

**验收标准**: 并发压力测试无 panic，重连逻辑正常

---

### 🟠 问题 #5: 流式更新节流缺陷

**文件**: `chatapps/engine_handler.go:576-637` (updateThrottled)

**问题描述**: 

虽然有 `updateThrottled` 实现，但节流逻辑存在缺陷：直接丢弃消息且不告知调用者，导致用户看到的内容不完整。

**影响**: 

- 调用者认为消息已发送，实际被丢弃
- 用户看到的内容不完整
- 没有重试或补偿机制

**修复方案**:

```go
// chatapps/engine_handler.go

// StreamState 添加待发送缓冲区
type StreamState struct {
	ChannelID   string
	MessageTS   string
	LastUpdated time.Time
	pending     string          // 待发送的内容
	mu          sync.Mutex
}

// updateThrottled 修复：返回错误而非静默丢弃
func (s *StreamState) updateThrottled(ctx context.Context, adapters *AdapterManager, platform, sessionID, content string, blockBuilder *slack.BlockBuilder, metadata map[string]any) error {
	s.mu.Lock()
	
	// 累积待发送内容
	if s.pending != "" {
		s.pending += content
	} else {
		s.pending = content
	}
	
	// 检查节流
	if time.Since(s.LastUpdated) < time.Second {
		s.mu.Unlock()
		// 返回特殊错误，调用方知道被节流
		return fmt.Errorf("throttled: next update in %v", 
			time.Second - time.Since(s.LastUpdated))
	}
	
	// 重置时间标记
	s.LastUpdated = time.Time{}
	pendingContent := s.pending
	s.pending = ""
	s.mu.Unlock()
	
	// 构建并发送 blocks
	blocks := blockBuilder.BuildAnswerBlock(pendingContent)
	
	var blocksAny []any
	for _, b := range blocks {
		blocksAny = append(blocksAny, b)
	}
	
	msg := &ChatMessage{
		Platform:  platform,
		SessionID: sessionID,
		Content:   pendingContent,
		RichContent: &RichContent{
			Blocks: blocksAny,
		},
		Metadata: make(map[string]any),
	}
	
	for k, v := range metadata {
		msg.Metadata[k] = v
	}
	
	if s.ChannelID != "" && s.MessageTS != "" {
		msg.Metadata["message_ts"] = s.MessageTS
		msg.Metadata["channel_id"] = s.ChannelID
	}
	
	err := adapters.SendMessage(ctx, platform, sessionID, msg)
	
	s.mu.Lock()
	if err == nil {
		s.LastUpdated = time.Now()
	} else {
		s.pending = pendingContent // 发送失败，保留内容重试
	}
	s.mu.Unlock()
	
	return err
}

// handleAnswer 修改：处理节流错误
func (c *StreamCallback) handleAnswer(data any) error {
	// ... 现有内容提取代码 ...
	
	if c.streamState != nil {
		err := c.streamState.updateThrottled(c.ctx, c.adapters, c.platform, c.sessionID, answerContent, c.blockBuilder, c.metadata)
		if err != nil {
			if strings.Contains(err.Error(), "throttled") {
				// 节流是正常现象，记录 debug 日志
				c.logger.Debug("Message throttled, will send next batch", "error", err)
				return nil
			}
			c.logger.Error("Stream update failed", "error", err)
		}
		return nil
	}
	
	// ... 现有非节流逻辑 ...
}
```

**测试要求**:
- [ ] 单元测试：1 秒内调用 10 次 updateThrottled，验证内容累积发送
- [ ] 集成测试：模拟流式回答，验证无内容丢失
- [ ] 错误恢复测试：模拟发送失败，验证内容保留重试

**验收标准**: 流式回答内容完整，无丢失

---

### 🟠 问题 #6: Slash Command 速率限制配置问题

**文件**: `chatapps/slack/adapter.go:49`, `chatapps/slack/rate_limiter.go`

**问题描述**: 

速率限制器使用了硬编码的 `rateBurst`，但 `rateBurst` 未定义或配置不正确，可能导致编译错误或运行时 panic。

**修复方案**:

```go
// chatapps/slack/rate_limiter.go

const (
	defaultSlashCommandRateLimit = 10 // 每秒 10 次
	defaultSlashCommandBurst     = 20 // 突发 20 次
)

// NewSlashCommandRateLimiterWithConfig 创建速率限制器
func NewSlashCommandRateLimiterWithConfig(rateLimit, burst int) *SlashCommandRateLimiter {
	if rateLimit <= 0 {
		rateLimit = defaultSlashCommandRateLimit
	}
	if burst <= 0 {
		burst = defaultSlashCommandBurst
	}
	
	return &SlashCommandRateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rateLimit), burst),
	}
}

// chatapps/slack/adapter.go

// NewAdapter 修改速率限制器初始化
func NewAdapter(config *Config, logger *slog.Logger, opts ...base.AdapterOption) *Adapter {
	// ...
	
	// 从配置读取速率限制，使用默认值
	rateLimit := config.SlashCommandRateLimit
	if rateLimit <= 0 {
		rateLimit = defaultSlashCommandRateLimit
	}
	
	a := &Adapter{
		// ...
		rateLimiter: NewSlashCommandRateLimiterWithConfig(rateLimit, defaultSlashCommandBurst),
	}
	
	// ...
	return a
}
```

**测试要求**:
- [ ] 单元测试：验证默认速率限制值
- [ ] 配置测试：自定义速率限制生效
- [ ] 压力测试：快速连续发送 50 个 slash command，验证限流生效

**验收标准**: Slash command 限流正常工作，无编译错误

---

## Phase 3: MEDIUM 问题修复 (P2)

### 🟡 问题 #7: 缺少 Socket Mode 事件处理

**文件**: `chatapps/slack/socket_mode.go:305-347`

**问题描述**: 

未处理以下重要事件类型：
- `app_uninstalled`: 应用被卸载
- `tokens_revoked`: Token 被撤销

**影响**: 

- 无法清理被卸载应用的资源
- Token 撤销后无法检测
- 可能错过重要生命周期事件

**修复方案**:

```go
// chatapps/slack/socket_mode.go

// handleMessage 添加缺失的事件处理
func (s *SocketModeConnection) handleMessage(data []byte) {
	var msg struct {
		Type       string          `json:"type"`
		EnvelopeID string          `json:"envelope_id,omitempty"`
		Payload    json.RawMessage `json:"payload,omitempty"`
		Body       json.RawMessage `json:"body,omitempty"`
	}
	
	if err := json.Unmarshal(data, &msg); err != nil {
		s.logger.Error("Failed to parse message", "error", err)
		return
	}
	
	s.logger.Debug("Received WebSocket message", "type", msg.Type)
	
	switch msg.Type {
	case "hello":
		s.logger.Info("Received hello from Slack")
		
	case "disconnect":
		s.logger.Warn("Received disconnect from Slack")
		s.mu.Lock()
		s.connected = false
		s.mu.Unlock()
		go s.reconnect()
		
	case "events_api":
		s.logger.Debug("events_api received", "envelope_id", msg.EnvelopeID)
		if len(msg.Payload) > 0 {
			s.handleEventsAPI(msg.Payload, msg.EnvelopeID)
		}
		
	case "slash_commands":
		s.logger.Debug("slash_commands received", "envelope_id", msg.EnvelopeID)
		if len(msg.Payload) > 0 {
			s.handleSlashCommands(msg.Payload, msg.EnvelopeID)
		}
		
	// 新增：应用卸载事件
	case "app_uninstalled":
		s.logger.Warn("App was uninstalled")
		if handler, ok := s.handlers["app_uninstalled"]; ok {
			handler("app_uninstalled", msg.Payload)
		}
		// 触发清理
		s.cancel()
		
	// 新增：Token 撤销事件
	case "tokens_revoked":
		s.logger.Warn("Tokens were revoked")
		if handler, ok := s.handlers["tokens_revoked"]; ok {
			handler("tokens_revoked", msg.Payload)
		}
		
	case "ping":
		_ = s.sendPong()
		
	case "pong":
		// Keep-alive acknowledged
		
	default:
		s.logger.Debug("Unknown message type", "type", msg.Type)
	}
}
```

**测试要求**:
- [ ] 单元测试：模拟 `app_uninstalled` 事件，验证 handler 被调用
- [ ] 集成测试：模拟 `tokens_revoked`，验证连接关闭

**验收标准**: 重要生命周期事件被正确处理

---

### 🟡 问题 #8: UpdateMessage 无重试逻辑

**文件**: `chatapps/slack/adapter.go:1158-1214`

**问题描述**: 

`UpdateMessage` 直接调用 API，没有像 `SendToChannel` 那样的重试逻辑。网络抖动时，消息更新失败但不会重试，导致 UI 状态不一致。

**修复方案**:

```go
// chatapps/slack/adapter.go

// UpdateMessage 添加重试逻辑
func (a *Adapter) UpdateMessage(ctx context.Context, channelID, messageTS string, blocks []any, fallbackText string) error {
	retryConfig := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    5 * time.Second,
	}
	
	return retryWithBackoff(ctx, retryConfig, func() error {
		return a.updateMessageOnce(ctx, channelID, messageTS, blocks, fallbackText)
	})
}

// updateMessageOnce 单次更新尝试
func (a *Adapter) updateMessageOnce(ctx context.Context, channelID, messageTS string, blocks []any, fallbackText string) error {
	payload := map[string]any{
		"channel": channelID,
		"ts":      messageTS,
		"text":    fallbackText,
		"blocks":  blocks,
	}
	
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", 
		"https://slack.com/api/chat.update", bytes.NewReader(body))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.config.BotToken)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	
	// 速率限制检查
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("rate limited: 429")
	}
	
	if resp.StatusCode >= 400 {
		return fmt.Errorf("update failed: %d %s", resp.StatusCode, string(respBody))
	}
	
	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &slackResp); err != nil {
		a.Logger().Warn("Failed to parse Slack response", "body", string(respBody))
		return nil
	}
	
	if !slackResp.OK {
		// 认证错误不重试
		if IsAuthError(slackResp.Error) {
			return fmt.Errorf("auth_failed: %s", slackResp.Error)
		}
		return fmt.Errorf("slack API error: %s", slackResp.Error)
	}
	
	a.Logger().Debug("Message updated successfully", "channel", channelID, "ts", slackResp.TS)
	return nil
}
```

**测试要求**:
- [ ] 单元测试：模拟网络失败，验证重试 3 次
- [ ] 集成测试：消息更新成功验证

**验收标准**: 网络抖动时消息更新成功率高

---

### 🟡 问题 #9: Session index 未清理

**文件**: `chatapps/base/adapter.go:340-361`

**问题描述**: 

`FindSessionByUserAndChannel` 虽然有 secondary index，但 index 只在 `GetOrCreateSession` 时更新，session 过期删除时没有同步清理 index，导致内存泄漏和错误查找。

**修复方案**:

```go
// chatapps/base/adapter.go

// cleanupSessions 修复：同步清理 secondary index
func (a *Adapter) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-a.cleanupDone:
			a.logger.Info("Session cleanup stopped", "platform", a.platformName)
			return
		case <-ticker.C:
			a.mu.Lock()
			now := time.Now()
			
			for key, session := range a.sessions {
				if now.Sub(session.LastActive) > a.sessionTimeout {
					// 同时清理 secondary index
					userChannelKey := session.UserID + ":" + a.extractChannelFromKey(key)
					
					a.indexMu.Lock()
					delete(a.sessionsByUserChannel, userChannelKey)
					a.indexMu.Unlock()
					
					delete(a.sessions, key)
					a.logger.Debug("Session removed", 
						"session", session.SessionID, 
						"inactive", now.Sub(session.LastActive))
				}
			}
			a.mu.Unlock()
		}
	}
}

// extractChannelFromKey 从 session key 提取 channel ID
// key format: "platform:user_id:bot_user_id:channel_id"
func (a *Adapter) extractChannelFromKey(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}
```

**测试要求**:
- [ ] 单元测试：创建 session，等待过期，验证 index 同步清理
- [ ] 内存测试：运行 24 小时，验证 index 内存不泄漏

**验收标准**: Session 过期后 index 同步清理，无内存泄漏

---

### 🟡 问题 #10: 错误消息未传达用户

**文件**: `chatapps/engine_handler.go:550-570`

**问题描述**: 

Engine 执行错误时，日志记录了但用户可能收不到错误消息，导致用户看到 AI"沉默"。

**修复方案**:

```go
// chatapps/engine_handler.go

// Handle 修改：确保错误总是传达给用户
func (h *EngineMessageHandler) Handle(ctx context.Context, msg *ChatMessage) error {
	// ... 现有配置代码 ...
	
	callback := NewStreamCallback(ctx, msg.SessionID, msg.Platform, h.adapters, h.logger, msg.Metadata)
	wrappedCallback := func(eventType string, data any) error {
		return callback.Handle(eventType, data)
	}
	
	h.logger.Info("Executing prompt via Engine",
		"session_id", msg.SessionID,
		"platform", msg.Platform,
		"prompt_len", len(msg.Content))
	
	err := h.engine.Execute(ctx, cfg, msg.Content, wrappedCallback)
	if err != nil {
		h.logger.Error("Engine execution failed",
			"session_id", msg.SessionID,
			"error", err)
		
		// 修复：总是尝试发送错误消息
		h.sendUserErrorMessage(ctx, msg.Platform, msg.SessionID, err, msg.Metadata)
		
		return err
	}
	
	return nil
}

// sendUserErrorMessage 向用户发送错误消息
func (h *EngineMessageHandler) sendUserErrorMessage(ctx context.Context, platform, sessionID string, err error, metadata map[string]any) {
	if h.adapters == nil {
		h.logger.Error("Cannot send error message: adapters is nil",
			"platform", platform, "error", err)
		return
	}
	
	// 构建用户友好的错误消息
	userErrMsg := h.formatUserErrorMessage(err)
	
	errMsg := &ChatMessage{
		Platform:  platform,
		SessionID: sessionID,
		Content:   userErrMsg,
		Metadata: map[string]any{
			"event_type": string(provider.EventTypeError),
		},
	}
	
	// 复制原始消息的 metadata（频道信息等）
	for k, v := range metadata {
		errMsg.Metadata[k] = v
	}
	
	if sendErr := h.adapters.SendMessage(ctx, platform, sessionID, errMsg); sendErr != nil {
		h.logger.Error("Failed to send error message to user",
			"session_id", sessionID,
			"original_error", err,
			"send_error", sendErr)
	} else {
		h.logger.Info("Error message sent to user",
			"session_id", sessionID,
			"error", err)
	}
}

// formatUserErrorMessage 格式化用户友好的错误消息
func (h *EngineMessageHandler) formatUserErrorMessage(err error) string {
	errStr := err.Error()
	
	// 认证错误
	if strings.Contains(errStr, "auth_failed") || 
	   strings.Contains(errStr, "invalid_auth") ||
	   strings.Contains(errStr, "token_revoked") {
		return "⚠️ **认证失败**: Bot Token 已过期或被撤销，请联系管理员更新配置。"
	}
	
	// 会话超时
	if strings.Contains(errStr, "timeout") || 
	   strings.Contains(errStr, "deadline exceeded") {
		return "⏱️ **请求超时**: AI 处理时间过长，请重试或简化请求。"
	}
	
	// 速率限制
	if strings.Contains(errStr, "rate limited") {
		return "🚦 **请求频繁**: 请稍后再试。"
	}
	
	// 通用错误
	return fmt.Sprintf("❌ **处理失败**: %s", errStr)
}
```

**测试要求**:
- [ ] 单元测试：各种错误类型验证正确格式化
- [ ] 集成测试：模拟 engine 错误，验证用户收到错误消息

**验收标准**: Engine 错误时用户收到友好错误消息

---

## Phase 4: LOW 问题修复 (P3)

### 🟢 问题 #11: 日志级别不一致

**文件**: 多处

**修复指南**:

```go
// 用户消息处理 - Debug 级别
a.Logger().Debug("Forwarding message to handler", 
	"sessionID", sessionID, "content", msg.Content)

// 权限相关 - Info 级别
a.Logger().Info("Permission callback received",
	"user_id", userID, "action", action)

// 错误 - Error 级别 + 上下文
a.Logger().Error("Engine execution failed",
	"session_id", sessionID, "error", err, "platform", platform)

// 性能敏感操作 - 添加 Warn 级别超时检测
start := time.Now()
// ... 操作 ...
if duration := time.Since(start); duration > 5*time.Second {
	a.Logger().Warn("Slow operation detected",
		"operation", "send_message", "duration", duration)
}
```

**日志级别规范**:

| 场景 | 级别 | 示例 |
|------|------|------|
| 用户消息处理 | Debug | 消息接收、转发 |
| 权限/安全 | Info | 权限检查通过/拒绝 |
| 业务操作 | Info | Session 创建/销毁 |
| 性能问题 | Warn | 操作耗时 > 5s |
| 可恢复错误 | Warn | 网络重试 |
| 不可恢复错误 | Error | 认证失败、配置错误 |

---

### 🟢 问题 #12: 配置验证不完整

**文件**: `chatapps/slack/config.go`

**修复方案**:

```go
// chatapps/slack/config.go

// Validate 完善配置验证
func (c *Config) Validate() error {
	var errs []string
	
	// 基本 token 验证
	if c.BotToken == "" && !c.IsSocketMode() {
		errs = append(errs, "bot token required for HTTP mode")
	}
	
	// Socket Mode 验证
	if c.IsSocketMode() {
		if c.AppToken == "" {
			errs = append(errs, "app token required for socket mode")
		}
		if c.AppToken != "" && !strings.HasPrefix(c.AppToken, "xapp-") {
			errs = append(errs, "app token must start with 'xapp-'")
		}
	}
	
	// Bot Token 格式验证
	if c.BotToken != "" && !strings.HasPrefix(c.BotToken, "xoxb-") {
		errs = append(errs, "bot token must start with 'xoxb-'")
	}
	
	// Signing Secret 验证 (HTTP mode)
	if !c.IsSocketMode() && c.SigningSecret == "" {
		errs = append(errs, "signing secret required for HTTP webhook mode")
	}
	
	// 工作区策略验证
	if c.GroupPolicy != "" && c.GroupPolicy != "all" && c.GroupPolicy != "mention" {
		errs = append(errs, "group_policy must be 'all' or 'mention'")
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("invalid config: %s", strings.Join(errs, "; "))
	}
	
	return nil
}
```

**测试要求**:
- [ ] 单元测试：各种无效配置验证失败
- [ ] 集成测试：启动时配置验证生效

---

### 🟢 问题 #13: 缺少单元测试覆盖

**文件**: 新建测试文件

**测试清单**:

```go
// chatapps/slack/socket_mode_test.go
func TestSocketMode_SendACK(t *testing.T) {
	// 测试 ACK 格式
}

func TestSocketMode_Reconnect(t *testing.T) {
	// 测试重连逻辑
}

func TestSocketMode_ConcurrentSend(t *testing.T) {
	// 测试并发 Send 无竞态
}

// chatapps/slack/adapter_test.go
func TestAdapter_SendRateLimiting(t *testing.T) {
	// 测试速率限制
}

func TestAdapter_AuthErrorHandling(t *testing.T) {
	// 测试认证错误处理
}

func TestAdapter_UpdateMessageRetry(t *testing.T) {
	// 测试 UpdateMessage 重试
}

// chatapps/base/adapter_test.go
func TestAdapter_SessionCleanup(t *testing.T) {
	// 测试 session 过期清理
}

func TestAdapter_SessionIndexCleanup(t *testing.T) {
	// 测试 secondary index 同步清理
}
```

**目标覆盖率**: > 80%

---

## 📋 验证清单

### Phase 1 验证
- [ ] Socket Mode ACK 格式通过 Slack 验证
- [ ] Token 过期时返回 `auth_failed` 错误
- [ ] 认证错误不触发无限重试

### Phase 2 验证
- [ ] 单频道 1 秒发送 10 条消息，前 5 条立即发送，后续节流
- [ ] 并发 10 goroutine 调用 Send 无 panic
- [ ] 流式回答无内容丢失
- [ ] Slash command 限流生效

### Phase 3 验证
- [ ] `app_uninstalled` 事件触发清理
- [ ] UpdateMessage 网络失败重试 3 次
- [ ] Session 过期后 index 同步清理
- [ ] Engine 错误时用户收到友好消息

### Phase 4 验证
- [ ] 日志级别符合规范
- [ ] 无效配置启动时报错
- [ ] 单元测试覆盖率 > 80%

---

## 🔄 回滚计划

### 回滚步骤

1. **Git 回滚**: 
   ```bash
   git revert <commit-hash>
   # 或
   git checkout <previous-tag>
   ```

2. **配置回滚**: 
   ```bash
   cp config/backup/*.yaml config/
   ```

3. **服务回滚**: 
   ```bash
   # Systemd
   systemctl restart hotplex
   
   # Kubernetes
   kubectl rollout undo deployment/hotplex
   ```

### 监控告警

修复部署后监控以下指标：

| 指标 | 告警阈值 | 说明 |
|------|---------|------|
| Slack API 错误率 | > 5% | 可能配置错误或限流 |
| 消息发送延迟 P99 | > 2s | 速率限制或网络问题 |
| Socket Mode 断线频率 | > 1 次/小时 | 连接不稳定 |
| 认证错误数 | > 0 | Token 过期 |

---

## 📅 执行时间表

| 日期 | 阶段 | 交付物 | 负责人 |
|------|------|--------|--------|
| Day 1-2 | Phase 1 | ACK 修复 + Token 错误处理 | 核心开发 |
| Day 3-5 | Phase 2 | 速率限制 + 竞态修复 | 核心开发 |
| Day 6-8 | Phase 3 | 事件处理 + 重试逻辑 | 一般开发 |
| Day 9-10 | Phase 4 | 日志 + 配置 + 测试 | 一般开发 |
| Day 11-12 | 验证 | 全面测试 + 文档 | 全体 |

**总工作量**: 约 80-100 小时  
**建议人员**: 2 名 Go 工程师并行开发

---

## 📚 参考资源

### Slack 官方文档
- [Socket Mode 规范](https://api.slack.com/apis/connections/socket)
- [chat.postMessage API](https://api.slack.com/methods/chat.postMessage)
- [速率限制](https://api.slack.com/apis/web-api/rate-limits)

### 相关代码
- `chatapps/slack/adapter.go` - Slack 适配器主逻辑
- `chatapps/slack/socket_mode.go` - Socket Mode 连接管理
- `chatapps/engine_handler.go` - Engine 集成
- `chatapps/base/adapter.go` - 基础适配器

### 测试命令
```bash
# 运行单元测试
go test ./chatapps/... -v

# 运行竞态检测
go test ./chatapps/... -race -v

# 生成覆盖率报告
go test ./chatapps/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## 📝 修订历史

| 版本 | 日期 | 修改内容 | 作者 |
|------|------|---------|------|
| 1.0 | 2026-02-26 | 初始版本 | AI Agent |

---

**文档状态**: ✅ 已完成  
**下次审查**: 2026-03-26 (修复完成后)
