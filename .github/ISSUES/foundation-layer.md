# HotPlex 共享基础层实施 - Issues #39/#41 公共依赖

## 📋 Issue 概述

**目标**: 实现 HotPlex 交互机制的共享基础层，为 Issue #39 (Claude Code + Slack) 和 Issue #41 (OpenCode + HTTP/SSE) 提供统一的事件模型、交互管理器、权限桥接器抽象和安全层扩展。

**优先级**: **Critical** (阻塞 #39, #41)  
**标签**: `enhancement`, `architecture`, `foundation`, `permission`  
**依赖**: 无  
**被依赖**: #39, #41

---

## 🎯 背景与动机

### 问题陈述

Issue #39 和 #41 都涉及 AI 代理的用户交互机制，但存在大量共享需求：

| 共享需求 | #39 (Claude Code) | #41 (OpenCode) | 重复风险 |
|---------|------------------|----------------|---------|
| 事件类型扩展 | `permission_request` | `permission_request`, `question` | ⚠️ 高 |
| 交互管理器 | 需要 | 需要 | ⚠️ 高 |
| 权限桥接器 | stdin 响应 | HTTP API 响应 | ⚠️ 中 |
| 数据结构定义 | `PermissionRequest` | `PermissionRequest` | ⚠️ 高 |
| 安全层集成 | WAF 绕过/确认 | WAF 绕过/确认 | ⚠️ 中 |

**如果不先实施共享基础层**:
- ❌ 代码重复 (两个团队各自实现 InteractionManager)
- ❌ 数据结构不一致 (PermissionRequest 字段可能不同)
- ❌ 后续重构成本高 (需要合并重复实现)
- ❌ 测试用例重复编写

### 设计原则

1. **抽象而非实现**: 提供接口和抽象类，具体 Provider 逻辑由 #39/#41 各自实现
2. **最小共享**: 仅共享真正通用的部分，避免过度设计
3. **向后兼容**: 不影响现有功能，新增类型和接口均为扩展
4. **清晰边界**: 共享层与 Provider 特定逻辑有明确分界

---

## 🔬 架构调研结果

### 1. 现有事件系统分析

#### 1.1 当前事件类型 (`provider/event.go`)

```go
const (
    EventTypeThinking    ProviderEventType = "thinking"
    EventTypeAnswer      ProviderEventType = "answer"
    EventTypeToolUse     ProviderEventType = "tool_use"
    EventTypeToolResult  ProviderEventType = "tool_result"
    EventTypeError       ProviderEventType = "error"
    EventTypeResult      ProviderEventType = "result"
    EventTypeStepStart   ProviderEventType = "step_start"    // OpenCode only
    EventTypeStepFinish  ProviderEventType = "step_finish"   // OpenCode only
    // ❌ 缺少: permission_request, question, clarification
)
```

#### 1.2 事件数据结构

```go
type ProviderEvent struct {
    Type      ProviderEventType
    Content   string
    Blocks    []ProviderContentBlock
    ToolName  string
    ToolInput map[string]any
    Status    string
    Error     string
    Metadata  *ProviderEventMeta
}

// ❌ 缺少交互请求专用的数据结构
// ❌ 缺少 PermissionRequest, QuestionRequest, PermissionReply 等
```

#### 1.3 事件流转路径

```
CLI Output → Provider.ParseEvent() → ProviderEvent → Engine Callback → 协议层
                                                             ↓
                            ┌────────────────────────────────┼────────────────────────────────┐
                            ↓                                ↓                                ↓
                    StreamCallback (ChatApps)        WebSocket Handler              SSE Handler (OpenCode)
                            ↓                                ↓                                ↓
                      Slack/Telegram                  Frontend Client                   Frontend Client
```

**扩展点**: 需要在事件流转中插入交互事件的特殊处理逻辑。

---

### 2. Provider 接口分析

#### 2.1 当前接口定义 (`provider/provider.go`)

```go
type Provider interface {
    Metadata() ProviderMeta
    BuildCLIArgs(providerSessionID string, opts *ProviderSessionOptions) []string
    BuildInputMessage(prompt string, taskInstructions string) (map[string]any, error)
    ParseEvent(line string) (*ProviderEvent, error)
    DetectTurnEnd(event *ProviderEvent) bool
    ValidateBinary() (string, error)
    Name() string
}
```

**评估**: 接口设计稳定，交互事件解析可通过扩展现有 `ParseEvent()` 实现，无需修改接口。

#### 2.2 Provider 差异对比

| 特性 | Claude Code | OpenCode | 对共享层的影响 |
|------|-------------|----------|---------------|
| **交互方式** | stdin/stdout | HTTP API | 需抽象响应接口 |
| **会话 ID** | `--session-id` | `--session` (ses_前缀) | Provider 特定逻辑 |
| **权限模式** | `--permission-mode` | Plan/Build | Provider 特定逻辑 |
| **事件格式** | stream-json | Part-based JSON | Provider 特定解析 |

**结论**: 共享层应聚焦于**事件模型**和**交互管理**，Provider 特定解析由各自实现。

---

### 3. ChatApps 架构分析

#### 3.1 现有组件

| 文件 | 职责 | 可复用性 |
|------|------|---------|
| `engine_handler.go` | Engine 事件 → ChatApp 消息桥接 | ⚠️ 部分复用 (Slack 特定) |
| `manager.go` | Adapter 生命周期管理 | ✅ 可复用 |
| `processor_chain.go` | 消息处理链 | ✅ 可复用 |

#### 3.2 AdapterManager 模式

```go
type AdapterManager struct {
    adapters map[string]ChatAdapter
    engines  []*engine.Engine
    mu       sync.RWMutex
}

// ✅ 现有模式已支持扩展，无需修改
```

---

### 4. 协议层分析

#### 4.1 WebSocket vs HTTP/SSE

| 方面 | WebSocket | HTTP/SSE | 共享需求 |
|------|-----------|---------|---------|
| **事件广播** | `writeJSON()` | `broadcastEvent()` | ✅ 需统一接口 |
| **客户端响应** | WS 消息 | HTTP POST | ⚠️ 不同路径 |
| **会话管理** | SessionID 复用 | Session 资源 | ⚠️ 不同模型 |

#### 4.2 需要统一的抽象

```go
// 协议无关的事件广播器
type EventBroadcaster interface {
    BroadcastPermissionAsked(req *PermissionRequest) error
    BroadcastQuestionAsked(req *QuestionRequest) error
    BroadcastPermissionReplied(reply *PermissionReply) error
    BroadcastQuestionReplied(reply *QuestionReply) error
}
```

---

### 5. 安全层分析

#### 5.1 当前 WAF 架构 (`internal/security/detector.go`)

```go
type Detector struct {
    rules        []SecurityRule
    allowPaths   []string
    bypassEnabled bool
    adminToken   string
}

type DangerBlockEvent struct {
    Operation      string
    Reason         string
    Level          DangerLevel  // Critical, High, Moderate
    BypassAllowed  bool         // ✅ 已存在，但未实际使用
    Suggestions    []string
}
```

#### 5.2 需要扩展的部分

**当前流程**:
```
用户输入 → Detector.CheckInput() → 检测到危险 → 直接阻断 (danger_block 事件)
```

**需要的流程**:
```
用户输入 → Detector.CheckInput() → 检测到危险
    ↓
    ├─→ Critical 级别 → 直接阻断 (不可绕过)
    └─→ High/Moderate 级别 → 触发权限请求 → 用户确认 → 允许/拒绝
```

**需要添加**:
- `InteractionEnabled` 配置 (启用交互式确认)
- `DangerBlockEvent` → `PermissionRequest` 转换逻辑
- 用户确认回调机制

---

## 🏗️ 架构设计

### 系统架构图

```
┌──────────────────────────────────────────────────────────────────────────┐
│                           Shared Foundation Layer                        │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  1. Event System (事件模型扩展)                                      │ │
│  │     - EventTypePermissionRequest, EventTypeQuestion, ...           │ │
│  │     - PermissionRequest, QuestionRequest 数据结构                   │ │
│  │     - PermissionReply, QuestionReply 响应结构                      │ │
│  │     文件：provider/event.go                                         │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  2. Interaction Manager (交互管理器)                                 │ │
│  │     - PendingInteraction 管理                                       │ │
│  │     - 注册/响应/超时处理                                            │ │
│  │     - 回调通知机制                                                  │ │
│  │     文件：chatapps/interaction_manager.go (新建)                    │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  3. Permission Bridge Abstract (权限桥接器抽象)                      │ │
│  │     - PermissionBridge 接口定义                                     │ │
│  │     - 生命周期方法：HandleRequest(), SendResponse()                │ │
│  │     - 具体实现由 #39/#41 各自完成                                   │ │
│  │     文件：chatapps/permission_bridge.go (新建)                      │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  4. Event Broadcaster Abstract (事件广播器抽象)                      │ │
│  │     - EventBroadcaster 接口定义                                     │ │
│  │     - 协议无关的广播方法                                            │ │
│  │     - WebSocket/SSE 各自实现                                        │ │
│  │     文件：server/event_broadcaster.go (新建)                        │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  5. Security Layer Extension (安全层扩展)                            │ │
│  │     - InteractionEnabled 配置                                       │ │
│  │     - DangerBlockEvent → PermissionRequest 转换                     │ │
│  │     - 用户确认回调接口                                              │ │
│  │     文件：internal/security/detector.go (修改)                      │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
                                      │
                    ┌─────────────────┴─────────────────┐
                    │                                   │
                    ▼                                   ▼
        ┌───────────────────┐              ┌───────────────────┐
        │   Issue #39       │              │   Issue #41       │
        │   Claude Code     │              │   OpenCode        │
        │   + Slack         │              │   + HTTP/SSE      │
        │                   │              │                   │
        │ - stdin 响应实现  │              │ - HTTP API 响应   │
        │ - Block Kit UI    │              │ - SSE 广播实现    │
        │ - Slack Adapter   │              │ - Web UI          │
        └───────────────────┘              └───────────────────┘
```

---

## ✅ 实施任务

### Task 1: 事件模型扩展

**文件**: `provider/event.go`

**内容**:
- [ ] **Task 1.1**: 添加交互事件类型枚举
  ```go
  const (
      EventTypePermissionRequest ProviderEventType = "permission_request"
      EventTypeQuestion          ProviderEventType = "question"
      EventTypeClarification     ProviderEventType = "clarification"
  )
  ```

- [ ] **Task 1.2**: 定义交互请求数据结构
  ```go
  type PermissionRequest struct {
      ID        string
      SessionID string
      Permission string  // bash, read, edit, task, ...
      Patterns   []string
      Metadata   map[string]any
      Always     []string
      Tool       *ToolReference
  }
  
  type QuestionRequest struct {
      ID        string
      SessionID string
      Questions []QuestionInfo
      Tool      *ToolReference
  }
  
  type QuestionInfo struct {
      Question  string
      Header    string
      Options   []QuestionOption
      Multiple  bool
      Custom    bool
  }
  ```

- [ ] **Task 1.3**: 定义响应数据结构
  ```go
  type PermissionReply struct {
      Reply   string  // "once", "always", "reject"
      Message string  // optional
  }
  
  type QuestionReply struct {
      Answers [][]string  // 二维数组，支持多选
  }
  ```

**验收标准**:
- [ ] 新增类型通过 `go test ./provider/...`
- [ ] 现有代码无编译错误
- [ ] 添加单元测试覆盖新数据结构

---

### Task 2: 交互管理器

**文件**: `chatapps/interaction_manager.go` (新建)

**内容**:
- [ ] **Task 2.1**: 定义 PendingInteraction 结构
  ```go
  type PendingInteraction struct {
      RequestID   string
      SessionID   string
      Type        string  // "permission" | "question"
      Request     any     // *PermissionRequest | *QuestionRequest
      CreatedAt   time.Time
      ExpiresAt   time.Time
      Status      string  // "pending", "replied", "expired"
  }
  ```

- [ ] **Task 2.2**: 实现 InteractionManager
  ```go
  type InteractionManager struct {
      mu        sync.RWMutex
      pending   map[string]*PendingInteraction
      callbacks map[string]InteractionCallback
  }
  
  type InteractionCallback func(response any) error
  
  func (m *InteractionManager) Register(req *PendingInteraction, cb InteractionCallback)
  func (m *InteractionManager) Reply(requestID string, response any) error
  func (m *InteractionManager) Get(requestID string) *PendingInteraction
  func (m *InteractionManager) Remove(requestID string)
  func (m *InteractionManager) CleanupExpired()
  ```

- [ ] **Task 2.3**: 实现超时处理
  ```go
  func (m *InteractionManager) StartExpirationChecker(interval time.Duration)
  // 定期检查过期请求，自动标记为 "expired"
  ```

**验收标准**:
- [ ] 单元测试覆盖所有公开方法
- [ ] 并发安全测试 (`go test -race`)
- [ ] 超时处理测试 (模拟过期场景)

---

### Task 3: 权限桥接器抽象

**文件**: `chatapps/permission_bridge.go` (新建)

**内容**:
- [ ] **Task 3.1**: 定义 PermissionBridge 接口
  ```go
  type PermissionBridge interface {
      // 处理权限请求
      HandlePermissionRequest(ctx context.Context, req *PermissionRequest) error
      
      // 发送响应 (具体实现由 Provider 决定)
      SendResponse(requestID string, reply any) error
      
      // 处理问题请求 (可选)
      HandleQuestionRequest(ctx context.Context, req *QuestionRequest) error
  }
  ```

- [ ] **Task 3.2**: 定义抽象基类 (可选)
  ```go
  type BasePermissionBridge struct {
      interactionMgr *InteractionManager
      logger         *slog.Logger
  }
  
  // 提供通用实现
  func (b *BasePermissionBridge) RegisterInteraction(req *PendingInteraction) error
  func (b *BasePermissionBridge) WaitForResponse(timeout time.Duration) (any, error)
  ```

**验收标准**:
- [ ] 接口定义清晰，文档完整
- [ ] 抽象基类提供有用的通用方法
- [ ] 无 Provider 特定逻辑

---

### Task 4: 事件广播器抽象

**文件**: `internal/server/event_broadcaster.go` (新建)

**内容**:
- [ ] **Task 4.1**: 定义 EventBroadcaster 接口
  ```go
  type EventBroadcaster interface {
      BroadcastPermissionAsked(req *PermissionRequest) error
      BroadcastQuestionAsked(req *QuestionRequest) error
      BroadcastPermissionReplied(reply *PermissionReply) error
      BroadcastQuestionReplied(reply *QuestionReply) error
  }
  ```

- [ ] **Task 4.2**: 实现 WebSocket 广播器
  ```go
  type WebSocketBroadcaster struct {
      clients map[string]*WebSocketClient
      mu      sync.RWMutex
  }
  
  func (b *WebSocketBroadcaster) BroadcastPermissionAsked(req *PermissionRequest) error {
      // 通过 WebSocket 发送事件
  }
  ```

- [ ] **Task 4.3**: 实现 SSE 广播器
  ```go
  type SSEBroadcaster struct {
      subscribers sync.Map  // map[string]chan string
  }
  
  func (b *SSEBroadcaster) BroadcastPermissionAsked(req *PermissionRequest) error {
      // 通过 SSE 发送事件
  }
  ```

**验收标准**:
- [ ] 两个实现均通过单元测试
- [ ] 并发安全测试 (`go test -race`)
- [ ] 接口使用示例文档

---

### Task 5: 安全层扩展

**文件**: `internal/security/detector.go` (修改)

**内容**:
- [ ] **Task 5.1**: 添加交互模式配置
  ```go
  type DetectorOptions struct {
      InteractionEnabled bool     // 启用交互式确认
      InteractionTimeout time.Duration  // 交互超时时间
      AutoDenyCategories []string // 自动拒绝的类别 (即使启用交互)
  }
  ```

- [ ] **Task 5.2**: 扩展 DangerBlockEvent
  ```go
  type DangerBlockEvent struct {
      // ... 现有字段 ...
      
      // 新增交互相关字段
      CanInteract      bool   // 是否支持用户确认
      InteractionType  string // "permission" | "question"
      DefaultAction    string // "deny" | "timeout" (超时后默认动作)
  }
  ```

- [ ] **Task 5.3**: 实现 DangerBlockEvent → PermissionRequest 转换
  ```go
  func (e *DangerBlockEvent) ToPermissionRequest(sessionID string) *PermissionRequest {
      return &PermissionRequest{
          ID: fmt.Sprintf("danger_%s", uuid.New().String()),
          SessionID: sessionID,
          Permission: "bash",
          Patterns: []string{e.Operation},
          Metadata: map[string]any{
              "reason": e.Reason,
              "level": e.Level.String(),
          },
      }
  }
  ```

- [ ] **Task 5.4**: 添加用户确认回调接口
  ```go
  type ConfirmationCallback func(requestID string, allowed bool) error
  
  func (d *Detector) CheckInputWithInteraction(
      input string, 
      sessionID string,
      cb ConfirmationCallback,
  ) (*DangerBlockEvent, error)
  ```

**验收标准**:
- [ ] 向后兼容测试 (现有 API 行为不变)
- [ ] 交互模式测试 (模拟用户确认流程)
- [ ] 超时处理测试

---

## 📝 接口使用示例

### 示例 1: Provider 事件解析

```go
// opencode_provider.go 或 claude_provider.go
func (p *OpenCodeProvider) ParseEvent(line string) (*ProviderEvent, error) {
    var msg OpenCodeMessage
    json.Unmarshal([]byte(line), &msg)
    
    for _, part := range msg.Parts {
        // 检测权限请求
        if part.Type == "permission_request" {
            return &ProviderEvent{
                Type: EventTypePermissionRequest,
                Content: part.Content,
                Metadata: &ProviderEventMeta{
                    // 解析权限请求详情
                },
            }, nil
        }
    }
    // ... 其他事件处理 ...
}
```

### 示例 2: 交互管理器使用

```go
// chatapps/engine_handler.go
func (h *EngineMessageHandler) handlePermissionRequest(req *PermissionRequest) error {
    // 1. 创建待处理交互
    pending := &PendingInteraction{
        RequestID: req.ID,
        SessionID: req.SessionID,
        Type: "permission",
        Request: req,
        ExpiresAt: time.Now().Add(5 * time.Minute),
    }
    
    // 2. 注册到管理器
    h.interactionMgr.Register(pending, func(response any) error {
        // 3. 用户响应回调
        reply := response.(*PermissionReply)
        return h.permissionBridge.SendResponse(req.ID, reply)
    })
    
    // 4. 广播到客户端
    return h.broadcaster.BroadcastPermissionAsked(req)
}
```

### 示例 3: HTTP API 响应处理

```go
// internal/server/opencode_http.go
func (s *OpenCodeHTTPHandler) handlePermissionReply(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    requestID := vars["id"]
    
    var req PermissionReply
    json.NewDecoder(r.Body).Decode(&req)
    
    // 通过交互管理器发送响应
    err := s.interactionMgr.Reply(requestID, &req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
```

---

## 🔗 依赖关系

### 本 Issue 的依赖

| Issue | 关系 | 说明 |
|-------|------|------|
| 无 | - | 本 Issue 为独立基础层实施 |

### 依赖本 Issue 的 Issues

| Issue | 标题 | 依赖内容 |
|-------|------|---------|
| **#39** | Claude Code 权限确认 ↔ Slack 交互桥接调研 | - 事件类型定义<br>- InteractionManager<br>- PermissionBridge 接口<br>- 安全层扩展 |
| **#41** | OpenCode 用户交互机制实现 | - 事件类型定义<br>- InteractionManager<br>- PermissionBridge 接口<br>- EventBroadcaster 接口<br>- 安全层扩展 |

### 相关 Issues

| Issue | 标题 | 关系 |
|-------|------|------|
| **#37** | Slack 消息处理链增强 | 前置工作，提供了部分基础 |
| **#38** | Engine Events → Slack Block Kit 映射 | 并行实施，有交叉但无直接依赖 |

---

## 📐 架构决策记录 (ADR)

### ADR-001: 为什么需要共享基础层？

**背景**: Issue #39 和 #41 都涉及用户交互机制，存在大量共享需求。

**决策**: 优先实施共享基础层，再由 #39/#41 各自实现 Provider 特定逻辑。

**理由**:
1. 避免代码重复 (InteractionManager 只需实现一次)
2. 统一数据结构 (PermissionRequest 字段一致)
3. 降低维护成本 (一处修改，多处受益)
4. 便于后续扩展 (新 Provider 可直接复用)

**后果**:
- ✅ 减少重复代码
- ✅ 统一接口设计
- ⚠️ 增加初期实施复杂度
- ⚠️ #39/#41 需等待本 Issue 完成

---

### ADR-002: 为什么 InteractionManager 放在 chatapps 层？

**背景**: InteractionManager 需要与 Engine 和 Protocol 层交互。

**决策**: 将 InteractionManager 放在 `chatapps/` 包中。

**理由**:
1. ChatApps 层已有 AdapterManager 模式，风格一致
2. InteractionManager 主要服务于平台适配器 (Slack, Web UI)
3. Engine 层保持无状态，专注于执行

**后果**:
- ✅ 与现有架构风格一致
- ✅ 便于平台适配器访问
- ⚠️ Protocol 层需要通过接口访问，不能直接依赖

---

### ADR-003: 为什么 PermissionBridge 是接口而非具体实现？

**背景**: Claude Code 和 OpenCode 的响应方式完全不同。

**决策**: 定义 PermissionBridge 接口，具体实现由 #39/#41 各自完成。

**理由**:
1. Claude Code: stdin 写入 `{"behavior": "allow"}`
2. OpenCode: HTTP POST `/permission/:id/reply`
3. 强制统一实现会导致过度抽象和复杂条件判断

**后果**:
- ✅ 各 Provider 实现最优响应方式
- ✅ 接口清晰，易于测试
- ⚠️ 需要实现两次 (但共享逻辑可复用 BasePermissionBridge)

---

## ✅ 验收标准

### 代码质量

- [ ] 所有新增代码通过 `go build ./...`
- [ ] 所有测试通过 `go test -race ./...`
- [ ] 无 lint 错误 (`make lint`)
- [ ] 新增代码覆盖率 ≥ 80%

### 功能验收

- [ ] 事件类型扩展正确定义
- [ ] InteractionManager 并发安全
- [ ] PermissionBridge 接口清晰
- [ ] EventBroadcaster 接口可互换实现
- [ ] 安全层向后兼容

### 文档验收

- [ ] 所有公开 API 有 godoc 注释
- [ ] 架构决策记录完整
- [ ] 接口使用示例清晰
- [ ] 更新 CHANGELOG

---

## 📚 参考资料

### 内部文档

- [Provider 接口定义](provider/provider.go)
- [事件系统](provider/event.go)
- [ChatApps 架构](chatapps/)
- [安全检测器](internal/security/detector.go)

### 外部参考

- [Issue #39: Claude Code 权限确认](https://github.com/hrygo/hotplex/issues/39)
- [Issue #41: OpenCode 用户交互机制](https://github.com/hrygo/hotplex/issues/41)
- [OpenCode Permissions API](https://opencode.ai/docs/permissions/)
- [Claude Code CLI Reference](https://m.runoob.com/claude-code/claude-code-cli-ref.html)

---

## 📌 备注

本 Issue 是 HotPlex 交互机制的**基础设施工程**，实施完成后将为 #39 和 #41 提供统一的开发基础。

**关键设计决策**:
1. 抽象而非实现：提供接口，具体逻辑由 #39/#41 实现
2. 最小共享：仅共享真正通用的部分
3. 向后兼容：不影响现有功能

**实施建议**:
- 按 Task 顺序逐步实施
- 每个 Task 完成后运行完整测试套件
- 保持与 #39/#41 负责人的沟通，确保接口满足需求
