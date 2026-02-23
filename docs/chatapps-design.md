# ChatApps 接入层调研与设计

## 1. OpenClaw 分析

### 1.1 什么是 OpenClaw

OpenClaw 是一个开源的对话优先(dialogue-first) AI 助手，可以接入多种聊天应用。用户通过自然语言与 AI 交互，无需复杂的配置文件。

### 1.2 支持的聊天平台

| 平台 | 接入方式 |
|------|---------|
| WhatsApp | QR pairing via Baileys |
| Telegram | Bot API via grammY |
| Discord | Servers, channels & DMs |
| Slack | Workspace apps via Bolt |
| Signal | Privacy-focused via signal-cli |
| iMessage | AppleScript bridge |
| Microsoft Teams | Enterprise support |
| Nextcloud Talk | Self-hosted |
| Matrix | Matrix protocol |
| Zalo | Zalo Bot API |
| WebChat | Browser-based UI |

### 1.3 核心特性

- **对话优先**: 使用自然语言交互，无需复杂配置
- **多平台支持**: 一个实例对接多个聊天平台
- **自托管**: 完全本地运行，保护隐私
- **灵活 AI**: 支持任意模型 (OpenAI, Anthropic, Google, 本地模型等)

---

## 2. HotPlex 现有架构

### 2.1 Provider 层 (AI CLI 适配)

```
Provider 接口
    ├── ClaudeCodeProvider (Claude Code CLI)
    └── OpenCodeProvider (OpenCode CLI)
```

每个 Provider 负责:
- 构建 CLI 启动参数
- 解析输出事件
- 处理输入输出格式

### 2.2 Server 层 (WebSocket/HTTP)

```
Server
    ├── HotPlexWS (原生 WebSocket 协议)
    └── OpenCodeHTTP (OpenCode 兼容层)
```

### 2.3 Hooks 层 (事件通知)

```
Hooks
    ├── WebhookHook (通用 Webhook)
    ├── DingTalkHook (钉钉通知) ✓ 已实现
    ├── SlackHook
    └── FeishuHook
```

---

## 3. ChatApps 接入层设计

### 3.1 架构目标

将 HotPlex 打造为 **ChatApps-as-a-Service** 平台，让用户通过任意聊天应用与 AI Agent 交互。

### 3.2 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        HotPlex Engine                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │   Engine    │  │  Session    │  │      AI CLI Providers    │ │
│  │   Core      │  │   Pool      │  │  Claude Code / OpenCode  │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              ▲
                              │ Event Stream (thinking/tool/answer)
                              │
┌─────────────────────────────────────────────────────────────────┐
│                    ChatApps Adapter Layer                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │  Adapter    │  │  Adapter    │  │      Adapter            │ │
│  │  Manager    │◄─┤  (Telegram) │  │  (DingTalk) [WIP]      │ │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘ │
│         │                │                    │                 │
│  ┌──────┴──────┐  ┌─────┴─────┐      ┌──────┴──────┐         │
│  │  Session    │  │  Message  │      │   Webhook   │         │
│  │  Manager    │  │  Router   │      │   Callback  │         │
│  └─────────────┘  └───────────┘      └─────────────┘         │
└─────────────────────────────────────────────────────────────────┘
                              ▲
                              │
┌─────────────────────────────┴─────────────────────────────────┐
│                    Chat Platforms                               │
│  ┌─────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐  │
│  │ DingTalk │  │ Telegram │  │  Slack  │  │   Discord    │  │
│  │  Bot    │  │    Bot   │  │   Bot   │  │      Bot     │  │
│  └─────────┘  └──────────┘  └──────────┘  └──────────────┘  │
└───────────────────────────────────────────────────────────────┘
```

### 3.3 核心组件

#### 3.3.1 Adapter 接口

```go
type ChatAdapter interface {
    // Platform 平台标识
    Platform() string
    
    // Start 启动适配器（监听消息）
    Start(ctx context.Context) error
    
    // Stop 停止适配器
    Stop() error
    
    // SendMessage 发送消息到用户
    SendMessage(ctx context.Context, sessionID string, msg *ChatMessage) error
    
    // HandleMessage 处理接收到的消息
    HandleMessage(ctx context.Context, msg *ChatMessage) error
}

type ChatMessage struct {
    Platform    string                 // 平台标识
    SessionID   string                 // 会话 ID
    UserID     string                 // 用户 ID
    Content    string                 // 消息内容
    MessageID  string                 // 消息 ID (用于回复)
    Timestamp  time.Time              // 时间戳
    Metadata   map[string]interface{} // 平台特定元数据
}
```

#### 3.3.2 Adapter Manager

```go
type AdapterManager struct {
    adapters map[string]ChatAdapter
    engine   *Engine
    logger   *slog.Logger
}

func (m *AdapterManager) Register(adapter ChatAdapter) error
func (m *AdapterManager) Unregister(platform string) error
func (m *AdapterManager) SendMessage(platform, sessionID string, msg *ChatMessage) error
func (m *AdapterManager) BroadcastEvent(platform, sessionID string, event *ProviderEvent) error
```

#### 3.3.3 DingTalk Adapter 设计

```go
type DingTalkAdapter struct {
    config    DingTalkConfig
    webhook   *WebhookServer
    client    *DingTalkClient
  logger   *slog.Logger
  sessions map[string]*DingTalkSession
}

type DingTalkConfig struct {
    // Webhook 模式
    WebhookURL string
    Secret    string
    
    // 回调模式 (可选)
    CallbackURL string
    CallbackToken string
    CallbackAESKey string
    
    // 会话管理
    SessionTimeout time.Duration
    MaxConcurrent int
}
```

### 3.4 消息流程

```
用户发送消息
    │
    ▼
DingTalk Server ──callback──► DingTalkAdapter.HandleMessage()
    │                                 │
    │                            转换消息格式
    │                                 │
    ▼                                 ▼
HotPlex Engine.Execute() ◄────── ChatMessage
    │
    │  Event Stream
    ▼
AdapterManager.BroadcastEvent()
    │
    ├──► DingTalkAdapter.SendMessage() ──► 钉钉消息
    │
    └──► TelegramAdapter.SendMessage() (如果多平台)
```

---

## 4. 钉钉机器人接入模式

### 4.1 两种模式

| 模式 | 说明 | 适用场景 |
|------|------|---------|
| **Webhook 模式** | 钉钉主动推送到服务器 | 已实现的 Hooks 通知 |
| **回调模式** | 服务器接收钉钉回调 | **需要实现** - 接收用户消息 |

### 4.2 回调模式 API

```go
// 钉钉回调消息结构
type DingTalkCallback struct {
    MsgType string `json:"msgtype"`
    Text    struct {
        Content string `json:"content"`
    } `json:"text"`
    ConversationID   string `json:"conversationId"`
    SenderID        string `json:"senderId"`
    IsAdmin         bool   `json:"isAdmin"`
    RobotCode       string `json:"robotCode"`
}

// 回调验证 (签名验证)
func VerifyCallback(signature, timestamp, secret string) bool
```

### 4.3 实现计划

1. **Phase 1: DingTalk 回调接入** (本次)
   - 创建 `chatapps/` 目录
   - 实现 `DingTalkAdapter`
   - 支持文本消息交互

2. **Phase 2: 消息处理增强**
   - 支持 Markdown 渲染
   - 支持多轮对话
   - 会话状态管理

3. **Phase 3: 多平台接入**
   - Telegram Bot
   - Slack Bot
   - Discord Bot

---

## 5. 环境配置

```bash
# .env

# ChatApps Adapter Configuration
HOTPLEX_CHATAPPS_ENABLED=true
HOTPLEX_CHATAPPS_PLATFORMS=dingtalk

# DingTalk
HOTPLEX_DINGTALK_APP_KEY=
HOTPLEX_DINGTALK_APP_SECRET=
HOTPLEX_DINGTALK_CALLBACK_URL=https://your-domain.com/webhook/dingtalk
HOTPLEX_DINGTALK_CALLBACK_TOKEN=
```

---

## 6. 风险与挑战

| 挑战 | 解决方案 |
|------|---------|
| 消息并发处理 | 每个用户独立 session，利用现有 SessionPool |
| 长响应处理 | 流式响应，分片发送 |
| 消息可靠性 | 消息队列 + 重试机制 |
| 安全隔离 | 每个用户独立 work_dir |
| 平台限制 | 钉钉消息大小限制，需分片 |

---

## 7. 参考资料

- [OpenClaw Integrations](https://openclaw.ai/integrations)
- [钉钉自定义机器人文档](https://open.dingtalk.com/document/dingstart/custom-bot-to-send-group-chat-messages)
- [钉钉回调事件](https://open.dingtalk.com/document/robot/event-list)
