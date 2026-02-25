# OpenCode 用户交互机制实现 - 授权/询问/澄清请求响应

## 📋 Issue 概述

**目标**: 实现 HotPlex 对 OpenCode 用户交互请求 (授权、询问、澄清) 的完整响应机制，使 HotPlex 能够作为 OpenCode 的交互式桥接层，支持生产环境中的权限确认、用户提问和任务澄清场景。

**优先级**: High  
**标签**: `enhancement`, `permission`, `interaction`, `opencode`, `protocol`

---

## 🎯 背景与动机

### 当前问题

HotPlex 目前仅支持单向的事件流 (CLI → Client),缺少对交互式请求的双向通信支持:

1. **无法解析交互事件**: OpenCode 的 `permission_request`、`question_asked` 等事件未被识别和解析
2. **缺少响应机制**: 无法将用户决策 (allow/deny/answer) 发送回 OpenCode
3. **WAF 直接阻止**: `internal/security/detector.go` 直接阻止危险命令，未经过用户确认流程
4. **协议不完整**: OpenCode HTTP/SSE 协议缺少交互事件的广播和响应 API

### 用户需求

在生产环境中，AI 代理需要在以下场景与用户交互:

- **权限请求**: 执行敏感操作 (文件编辑、危险命令) 前请求用户授权
- **用户询问**: 任务执行过程中向用户提问获取更多信息
- **澄清请求**: 任务目标不明确时请求用户澄清

---

## 🔬 技术调研结果

### 1. OpenCode 交互协议

OpenCode 使用 **HTTP + Server-Sent Events **(SSE) 作为通信协议:

#### 1.1 权限请求 (Permission Request)

**事件格式** ([packages/opencode/src/permission/next.ts](https://github.com/anomalyco/opencode/blob/main/packages/opencode/src/permission/next.ts#L68-L107)):

```typescript
// 权限请求
{
  id: "perm_xxx",
  sessionID: "ses_xxx",
  permission: "bash",           // read, edit, bash, task, ...
  patterns: ["rm -rf *"],       // 匹配模式
  metadata: { /* 工具调用上下文 */ },
  always: ["bash:rm -rf *"],    // "always" 选项建议
  tool: { messageID: "msg_xxx", callID: "call_xxx" }
}

// 响应格式
{
  reply: "once" | "always" | "reject",  // once: 本次，always: 永久，reject: 拒绝
  message?: string                       // 拒绝原因
}
```

**API 端点**:
- `GET /permission/` - 获取所有待处理权限请求
- `POST /permission/:requestID/reply` - 响应权限请求
- `GET /global/event` - SSE 事件订阅 (接收 `permission.asked`、`permission.replied` 事件)

#### 1.2 用户询问 (Question Request)

**事件格式** ([packages/opencode/src/question/index.ts](https://github.com/anomalyco/opencode/blob/main/packages/opencode/src/question/index.ts#L21-L80)):

```typescript
// 问题请求
{
  id: "quest_xxx",
  sessionID: "ses_xxx",
  questions: [{
    question: "Which framework do you prefer?",
    header: "Framework choice",
    options: [
      { label: "React", description: "Component-based UI library" },
      { label: "Vue", description: "Progressive JavaScript framework" }
    ],
    multiple: false,  // 是否允许多选
    custom: true      // 是否允许自定义输入
  }]
}

// 响应格式
{
  answers: [["React"]]  // 按问题顺序返回选中的选项标签
}
```

**API 端点**:
- `GET /question/` - 获取所有待处理问题
- `POST /question/:requestID/reply` - 回复问题
- `POST /question/:requestID/reject` - 拒绝回答

#### 1.3 事件流

OpenCode Bus 事件系统定义的事件类型:

```typescript
// 权限事件
permission.asked      // 权限请求发起
permission.replied    // 用户已响应
permission.updated    // 权限状态更新

// 问题事件
question.asked        // 问题发起
question.replied      // 用户已回答
question.rejected     // 用户拒绝回答

// 消息事件
message.updated       // 消息更新
message.part.updated  // 消息片段更新
```

### 2. HotPlex 当前架构

#### 2.1 事件处理架构

**当前事件类型** (`provider/event.go`):

```go
const (
    EventTypeThinking    = "thinking"
    EventTypeAnswer      = "answer"
    EventTypeToolUse     = "tool_use"
    EventTypeToolResult  = "tool_result"
    EventTypeError       = "error"
    EventTypeResult      = "result"
    EventTypeStepStart   = "step_start"
    EventTypeStepFinish  = "step_finish"
    // ❌ 缺少：EventTypePermissionRequest, EventTypeQuestion
)
```

**事件流转**:
```
CLI Output → Provider.ParseEvent() → ProviderEvent → Engine Callback → 协议层 (WS/SSE)
```

#### 2.2 OpenCode 协议层

**当前实现** (`internal/server/opencode_http.go`):

```go
func (s *OpenCodeHTTPHandler) mapToOpenCodePart(pevt *event.EventWithMeta, sessionID, messageID string) map[string]any {
    switch pevt.EventType {
    case "answer":
        base["type"] = "text"
        base["text"] = pevt.EventData
    case "thinking":
        base["type"] = "reasoning"
        base["text"] = pevt.EventData
    case "tool_use":
        base["type"] = "tool"
        // ...
    // ❌ 缺少:permission、question 事件映射
    }
}
```

**缺少的 API**:
- ❌ `POST /session/{id}/permissions/{permissionID}` - 权限响应
- ❌ `POST /question/:requestID/reply` - 问题回复
- ❌ SSE 事件广播:`permission.asked`、`question.asked`

---

## 🏗️ 架构设计

### 3.1 系统架构图

```
┌─────────────────────┐     ┌──────────────────────┐     ┌─────────────────────┐
│   OpenCode CLI      │────▶│   HotPlex Engine     │────▶│   SSE Client        │
│   (permission_asked)│     │   (Event Parser)     │     │   (Web UI / SDK)    │
└─────────────────────┘     └──────────────────────┘     └─────────────────────┘
         ▲                          │                          │
         │                          │                          │
         │              ┌───────────▼───────────┐     ┌──────▼──────────────┐
         │              │  Interaction Manager  │     │  User Decision UI   │
         │              │  (Permission Bridge)  │     │  (Allow/Deny/Answer)│
         │              └───────────┬───────────┘     └─────────────────────┘
         │                          │
         └──────────────────────────┘
              POST /permissions/:id/reply
              { reply: "once" | "always" | "reject" }
```

### 3.2 核心组件

#### 1. 事件层扩展

**文件**: `provider/event.go`

需要添加:
- 新增事件类型:`EventTypePermissionRequest`, `EventTypeQuestion`, `EventTypeClarification`
- 新增数据结构:`PermissionRequest`, `QuestionRequest`, `ClarificationRequest`
- 新增响应结构:`PermissionReply`, `QuestionReply`

#### 2. Provider 层实现

**文件**: `provider/opencode_provider.go`

需要添加:
- `parsePermissionRequest()` - 解析权限请求事件
- `parseQuestionRequest()` - 解析问题请求事件
- 扩展 `ParseEvent()` 支持交互事件检测

#### 3. 交互管理器

**文件**: `chatapps/interaction_manager.go` (新建)

核心职责:
- 管理所有待处理的交互请求 (pending interactions)
- 提供注册、响应、超时处理机制
- 支持回调函数通知用户决策

#### 4. 权限桥接器

**文件**: `chatapps/permission_bridge.go` (新建)

核心职责:
- 桥接 HotPlex Engine 与 OpenCode HTTP API
- 处理权限请求的完整生命周期
- 发送用户决策到 OpenCode (`POST /permission/:id/reply`)

#### 5. HTTP API 扩展

**文件**: `internal/server/opencode_http.go`

需要添加的端点:
```
POST /permission/{id}/reply     - 响应权限请求
POST /question/{id}/reply       - 回复问题
POST /question/{id}/reject      - 拒绝回答
```

#### 6. SSE 事件广播

**文件**: `internal/server/opencode_http.go`

需要添加的事件广播:
- `broadcastPermissionAsked()` - 广播权限请求
- `broadcastQuestionAsked()` - 广播问题请求
- `broadcastPermissionReplied()` - 广播权限响应
- `broadcastQuestionReplied()` - 广播问题回答

---

## ✅ 实施计划

### Phase 1: 基础架构 (Week 1)

- [ ] **Task 1.1**: 扩展 `provider/event.go` - 添加交互事件类型和数据结构
- [ ] **Task 1.2**: 扩展 `provider/opencode_provider.go` - 实现交互事件解析
- [ ] **Task 1.3**: 创建 `chatapps/interaction_manager.go` - 交互管理器
- [ ] **Task 1.4**: 创建 `chatapps/permission_bridge.go` - 权限桥接器

### Phase 2: 协议层实现 (Week 2)

- [ ] **Task 2.1**: 扩展 `internal/server/opencode_http.go` - 添加交互 API 端点
- [ ] **Task 2.2**: 实现 SSE 事件广播机制
- [ ] **Task 2.3**: 实现 `mapToOpenCodePart()` 支持 permission/question 事件映射
- [ ] **Task 2.4**: 添加 API Key 认证支持

### Phase 3: 安全与集成 (Week 3)

- [ ] **Task 3.1**: 修改 `internal/security/detector.go` - 支持交互式确认而非直接阻止
- [ ] **Task 3.2**: 实现超时处理机制 (默认 5 分钟无响应自动拒绝)
- [ ] **Task 3.3**: 实现"always"选项 - 持久化权限配置
- [ ] **Task 3.4**: 编写单元测试和集成测试

### Phase 4: 测试与文档 (Week 4)

- [ ] **Task 4.1**: 编写端到端测试 (模拟完整交互流程)
- [ ] **Task 4.2**: 更新 SDK 文档 - 添加交互机制使用指南
- [ ] **Task 4.3**: 创建示例代码 (`_examples/go_opencode_interaction/`)
- [ ] **Task 4.4**: 更新 CHANGELOG 和 README

---

## 📝 API 设计示例

### 权限请求响应

```bash
# 客户端响应权限请求
curl -X POST http://localhost:8080/permission/perm_abc123/reply \
  -H "Content-Type: application/json" \
  -d '{
    "reply": "once",
    "message": "允许本次执行"
  }'
```

### 问题回复

```bash
# 客户端回复问题
curl -X POST http://localhost:8080/question/quest_xyz789/reply \
  -H "Content-Type: application/json" \
  -d '{
    "answers": [["React"]]
  }'
```

### SSE 事件订阅

```javascript
// 客户端订阅事件
const eventSource = new EventSource('http://localhost:8080/global/event');

eventSource.addEventListener('permission.asked', (event) => {
  const data = JSON.parse(event.data);
  console.log('权限请求:', data.payload.request);
  // 显示 UI 让用户选择 Allow/Deny
});

eventSource.addEventListener('question.asked', (event) => {
  const data = JSON.parse(event.data);
  console.log('用户问题:', data.payload.request);
  // 显示问题 UI 让用户回答
});
```

---

## 🔗 参考资料

### 相关 Issues

- [ ] #39 - Claude Code 权限确认 ↔ Slack 交互桥接调研
- [ ] #37 - Slack 消息处理链增强 - 分块/线程/交互支持
- [ ] #38 - Engine Events → Slack Block Kit 最佳展现映射

### 外部资源

- [OpenCode Permissions 文档](https://opencode.ai/docs/permissions/)
- [OpenCode Server API](https://opencode.ai/docs/server/)
- [Claude Code CLI Reference](https://m.runoob.com/claude-code/claude-code-cli-ref.html)
- [Slack Block Kit Builder](https://api.slack.com/tools/block-kit-builder)

### 代码参考

- [OpenCode Permission Implementation](https://github.com/anomalyco/opencode/blob/main/packages/opencode/src/permission/next.ts)
- [OpenCode Question Implementation](https://github.com/anomalyco/opencode/blob/main/packages/opencode/src/question/index.ts)
- [OpenCode Server Routes](https://github.com/anomalyco/opencode/blob/main/packages/opencode/src/server/routes/permission.ts)

---

## 🎯 成功标准

实现完成后，HotPlex 应能够:

1. ✅ 正确解析 OpenCode 的 `permission_request`和`question` 事件
2. ✅ 通过 SSE 实时广播交互事件到客户端
3. ✅ 接收客户端响应并通过 HTTP API 发送回 OpenCode
4. ✅ 处理超时场景 (无响应自动拒绝)
5. ✅ 支持"always"选项的持久化配置
6. ✅ 所有单元测试和集成测试通过
7. ✅ 提供完整的示例代码和文档

---

## 📌 备注

本实现参考了 Issue #39 中 Claude Code 的权限交互机制，并针对 OpenCode 的 HTTP/SSE 协议进行了适配。实现完成后，HotPlex 将同时支持 Claude Code 和 OpenCode 的交互式权限确认功能。
