# ChatApps 接入层用户手册

HotPlex ChatApps 接入层允许用户通过各种聊天应用与 AI Agent 进行交互。参考 OpenClaw 的设计理念，将 HotPlex 打造成 **ChatApps-as-a-Service** 平台。

## 支持的平台

| 平台 | 本地运行 | 难度 | 说明 |
|------|:--------:|:----:|------|
| Telegram ⭐ | ✅ | 最简单 | Bot API |
| 钉钉 | ✅ | 中等 | 需要企业应用 |
| WhatsApp | ✅ | 简单 | QR 配对 |
| Slack | ✅ | 中等 | Socket Mode |

## 目录

1. [架构概述](#1-架构概述)
2. [快速开始](#2-快速开始)
3. [钉钉接入指南](#3-钉钉接入指南)
4. [适配器开发](#4-适配器开发)
5. [配置参考](#5-配置参考)

---

## 1. 架构概述

### 1.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        HotPlex Engine                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Engine    │  │  Session    │  │   AI CLI Providers  │ │
│  │    Core     │  │    Pool     │  │ Claude Code/OpenCode│ │
│  └─────────────┘  └─────────────┘  └─────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                              ▲
                              │ Event Stream
                              │
┌─────────────────────────────────────────────────────────────┐
│                    ChatApps Adapter Layer                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │   Adapter   │  │   Adapter   │  │     Adapter     │  │
│  │  Manager   │◄─┤ (DingTalk)  │  │   (Telegram)   │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              ▲
                              │
┌─────────────────────────────┴─────────────────────────────┐
│                      Chat Platforms                        │
│  ┌─────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐ │
│  │ DingTalk │  │ Telegram  │  │  Slack   │  │ Discord│ │
│  └─────────┘  └──────────┘  └──────────┘  └────────┘ │
└───────────────────────────────────────────────────────────┘
```

### 1.2 核心组件

| 组件 | 说明 |
|------|------|
| `ChatAdapter` | 聊天平台适配器接口 |
| `AdapterManager` | 适配器管理器 |
| `ChatMessage` | 统一消息格式 |
| `DingTalkAdapter` | 钉钉实现 |

### 1.3 消息流程

```
用户发送消息
    │
    ▼
Chat Platform ──callback──► Adapter.HandleMessage()
    │                           │
    │                      转换为 ChatMessage
    │                           │
    ▼                           ▼
HotPlex Engine.Execute() ◄──────
    │
    │ Event Stream
    ▼
Adapter.SendMessage() ──► Chat Platform (回复用户)
```

---

## 2. 快速开始

### 2.1 环境要求

- Go 1.24+
- 可访问的公网地址（用于接收回调）

### 2.2 运行示例

```bash
# 克隆项目
git clone https://github.com/hrygo/hotplex.git
cd hotplex

# 运行钉钉示例
go run _examples/chatapps_dingtalk_local/main.go
```

### 2.3 本地测试

由于钉钉回调需要公网地址，使用内网穿透：

```bash
# 终端 1: 启动服务
go run _examples/chatapps_dingtalk_local/main.go

# 终端 2: 启动 ngrok
ngrok http 8080

# 终端 3: 将 ngrok 地址配置到钉钉开放平台
```

---

## 3. 钉钉接入指南

### 3.1 创建钉钉应用

1. 登录 [钉钉开放平台](https://open.dingtalk.com)
2. 创建企业内部应用
3. 添加「机器人」能力
4. 配置回调地址

### 3.2 配置回调地址

在钉钉应用配置页面：

| 配置项 | 说明 |
|--------|------|
| 回调地址 | `https://your-domain.com/webhook` |
| 签名密钥 | 可选，建议启用 |

### 3.3 环境变量

```bash
# 基础配置
HOTPLEX_CHATAPPS_ADDR=:8080

# 钉钉应用凭证
HOTPLEX_DINGTALK_APP_ID=your_app_id
HOTPLEX_DINGTALK_APP_SECRET=your_app_secret
HOTPLEX_DINGTALK_CALLBACK_TOKEN=your_callback_token
```

### 3.4 运行完整版

```bash
export HOTPLEX_DINGTALK_APP_ID=your_app_id
export HOTPLEX_DINGTALK_APP_SECRET=your_app_secret
export HOTPLEX_DINGTALK_CALLBACK_TOKEN=your_token

go run _examples/chatapps_dingtalk/main.go
```

---

## 4. 适配器开发

### 4.1 实现 ChatAdapter 接口

```go
type ChatAdapter interface {
    // Platform 返回平台标识
    Platform() string
    
    // Start 启动适配器
    Start(ctx context.Context) error
    
    // Stop 停止适配器
    Stop() error
    
    // SendMessage 发送消息
    SendMessage(ctx context.Context, sessionID string, msg *ChatMessage) error
    
    // HandleMessage 处理接收消息
    HandleMessage(ctx context.Context, msg *ChatMessage) error
}
```

### 4.2 ChatMessage 结构

```go
type ChatMessage struct {
    Platform   string            // 平台标识
    SessionID  string           // 会话 ID
    UserID     string           // 用户 ID
    Content    string           // 消息内容
    MessageID  string           // 消息 ID
    Timestamp  time.Time        // 时间戳
    Metadata   map[string]any   // 平台特定数据
}
```

### 4.3 示例：创建新适配器

```go
package chatapps

type MyAdapter struct {
    config MyConfig
    logger *slog.Logger
}

func NewMyAdapter(config MyConfig, logger *slog.Logger) *MyAdapter {
    return &MyAdapter{
        config: config,
        logger: logger,
    }
}

func (a *MyAdapter) Platform() string {
    return "my-platform"
}

func (a *MyAdapter) Start(ctx context.Context) error {
    // 启动服务监听
    return nil
}

func (a *MyAdapter) Stop() error {
    // 停止服务
    return nil
}

func (a *MyAdapter) SendMessage(ctx context.Context, sessionID string, msg *ChatMessage) error {
    // 发送消息到平台
    return nil
}

func (a *MyAdapter) HandleMessage(ctx context.Context, msg *ChatMessage) error {
    // 处理收到的消息
    return nil
}
```

---

## 5. 配置参考

### 5.1 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `HOTPLEX_CHATAPPS_ADDR` | 服务监听地址 | `:8080` |
| `HOTPLEX_DINGTALK_APP_ID` | 钉钉应用 ID | - |
| `HOTPLEX_DINGTALK_APP_SECRET` | 钉钉应用密钥 | - |
| `HOTPLEX_DINGTALK_CALLBACK_TOKEN` | 回调签名密钥 | - |

### 5.2 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/webhook` | GET/POST | 聊天平台回调接口 |
| `/health` | GET | 健康检查 |

### 5.3 DingTalk 配置

```go
type DingTalkConfig struct {
    AppID         string  // 应用 ID
    AppSecret     string  // 应用密钥
    CallbackURL   string  // 回调地址
    CallbackToken string  // 签名密钥
    CallbackKey   string  // AES 密钥
    ServerAddr    string  // 服务地址
}
```

---

## 常见问题

### Q: 本地测试需要公网地址吗？

A: 是的，钉钉回调需要公网地址。可使用 ngrok 或 cloudflared 进行内网穿透。

### Q: 支持哪些消息类型？

A: 当前支持文本消息。Markdown、图片等格式在规划中。

### Q: 如何接入其他聊天平台？

A: 实现 `ChatAdapter` 接口即可。当前支持钉钉，其他平台（Telegram、Slack 等）按需开发。

---

## 相关文档

- [ChatApps 架构设计](./chatapps-design.md)
- [Hooks 事件系统](./hooks-architecture.md)
- [SDK 使用指南](./sdk-guide.md)
