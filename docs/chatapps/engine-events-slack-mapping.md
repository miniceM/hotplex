# Engine Events → Slack Block Kit 映射最佳实践

> **状态**: ✅ 已实现  
> **最后更新**: 2026-02-26  
> **相关 Issue**: [#38](https://github.com/hrygo/hotplex/issues/38)  
> **实现文件**: `chatapps/slack/block_builder.go` (1081 行)

---

## 📋 目录

- [概述](#概述)
- [Engine Events 完整类型](#engine-events-完整类型)
- [Slack Block Kit 映射方案](#slack-block-kit-映射方案)
  - [1. Thinking 事件](#1-thinking-事件)
  - [2. Tool Use 事件](#2-tool-use-事件)
  - [3. Tool Result 事件](#3-tool-result-事件)
  - [4. Answer 事件](#4-answer-事件)
  - [5. Error 事件](#5-error-事件)
  - [6. Danger Block 事件](#6-danger-block-事件)
  - [7. Session Stats 事件](#7-session-stats-事件)
  - [8. Permission Request 事件](#8-permission-request-事件)
- [事件聚合策略](#事件聚合策略)
- [Block Builder API 参考](#block-builder-api-参考)
- [最佳实践与限制](#最佳实践与限制)

---

## 概述

本文档定义了 HotPlex Engine Events 到 Slack Block Kit 的完整映射方案，确保 AI 代理的执行过程在 Slack 中以最佳 UX/UI 形式展现。

### 核心设计原则

| 原则 | 说明 |
|------|------|
| **即时反馈** | Thinking、Error 等关键事件立即发送，不聚合 |
| **同类聚合** | Tool Use/Result 等相似事件可合并展示 |
| **流式更新** | Answer 事件使用 `chat.update` API 节流更新 (1 次/秒) |
| **丰富上下文** | 使用 Context Blocks 展示元数据（时长、Token 等） |
| **交互友好** | 长输出提供"View Full Output"按钮 |

---

## Engine Events 完整类型

### ProviderEventType 枚举

| 事件类型 | 值 | 描述 | 触发时机 |
|---------|-----|------|---------|
| `EventTypeThinking` | `"thinking"` | AI 推理中 | CLI 输出 `type="thinking"` |
| `EventTypeAnswer` | `"answer"` | AI 文本输出 | CLI 输出 `type="assistant"` |
| `EventTypeToolUse` | `"tool_use"` | 工具调用开始 | CLI 输出 `type="tool_use"` |
| `EventTypeToolResult` | `"tool_result"` | 工具执行结果 | CLI 输出 `type="tool_result"` |
| `EventTypeError` | `"error"` | 错误发生 | CLI 输出 `type="error"` |
| `EventTypeResult` | `"result"` | Turn 完成 | CLI 输出 `type="result"` |
| `EventTypePermissionRequest` | `"permission_request"` | 权限请求 | Claude Code 请求用户审批 |

### EventMeta 字段结构

```go
type EventMeta struct {
    // 时长
    DurationMs      int64 `json:"duration_ms"`
    TotalDurationMs int64 `json:"total_duration_ms"`
    
    // 工具信息
    ToolName string `json:"tool_name"`  // e.g., "Bash", "Edit"
    ToolID   string `json:"tool_id"`
    Status   string `json:"status"`     // "running" | "success" | "error"
    ErrorMsg string `json:"error_msg"`
    
    // Token 使用
    InputTokens  int32 `json:"input_tokens"`
    OutputTokens int32 `json:"output_tokens"`
    
    // 文件操作
    FilePath  string `json:"file_path"`
    LineCount int32  `json:"line_count"`
}
```

---

## Slack Block Kit 映射方案

### 1. Thinking 事件

**Block 类型**: `context`  
**聚合策略**: ❌ 不聚合 - 立即发送  
**UI 目标**: 即时反馈，让用户知道 AI 正在思考

```json
{
  "type": "context",
  "elements": [{
    "type": "mrkdwn",
    "text": ":brain: _Thinking..._"
  }]
}
```

**设计说明**:
- 使用 `context` block（低视觉权重）
- 支持流式更新（节流 1 次/秒）
- 混合内容展示（有内容时展示，无内容用默认文案）

---

### 2. Tool Use 事件

**Block 类型**: `section` + `fields`  
**聚合策略**: ✅ 500ms 时间窗口聚合  
**UI 目标**: 清晰展示工具名称和输入参数

**Tool Emoji 映射表**:

| 工具类型 | Emoji | 示例 |
|---------|-------|------|
| `Bash` | `:computer:` | 执行 shell 命令 |
| `Edit` / `MultiEdit` | `:pencil:` | 文件编辑 |
| `Write` / `FileWrite` | `:page_facing_up:` | 创建文件 |
| `Read` / `FileRead` | `:books:` | 文件读取 |
| `FileSearch` / `Glob` | `:mag:` | 文件搜索 |
| `WebFetch` / `WebSearch` | `:globe_with_meridians:` | 网络请求 |
| `Grep` | `:magnifying_glass_tilted_left:` | 内容搜索 |
| `LS` / `List` | `:file_folder:` | 目录列表 |
| 其他 | `:hammer_and_wrench:` | 默认 |

**设计说明**:
- 按工具类型映射 Emoji（更直观）
- 输入内容使用可折叠代码块展示
- 500ms 缓冲收集多个工具后合并发送

---

### 3. Tool Result 事件

**Block 类型**: `section` + `context` + `actions` (可选)  
**聚合策略**: ✅ 同类聚合  
**UI 目标**: 展示执行状态、时长、输出预览

**状态展示格式**: `{Emoji} {ToolName} {Status}`

示例：`:computer: *Bash Completed*` / `:pencil: *Edit Failed*`

**关键实现细节**:

1. **时长阈值逻辑**: 仅当 `durationMs > 500ms` 时才展示 Duration 信息
   ```go
   const toolResultDurationThreshold = 500 // ms
   
   if durationMs > toolResultDurationThreshold {
       blocks = append(blocks, map[string]any{
           "type": "context",
           "elements": []map[string]any{
               mrkdwnText(fmt.Sprintf(":timer_clock: *Duration:* %s", formatDuration(durationMs))),
           },
       })
   }
   ```

2. **文件路径展示**: 当工具涉及文件操作时，展示文件图标 + 路径
   ```go
   if filePath != "" {
       displayPath := truncatePath(filePath, 50)
       blocks = append(blocks, map[string]any{
           "type": "context",
           "elements": []map[string]any{
               mrkdwnText(fmt.Sprintf(":page_facing_up: *File:* %s", displayPath)),
           },
       })
   }
   ```

3. **输出截断**: 预览限制 300 字符，提供"View Full Output"按钮

4. **Thread 展开**: 点击按钮后在 Thread 中回复完整输出

---

### 4. Answer 事件

**Block 类型**: `section` (mrkdwn)  
**聚合策略**: ✅ 流式更新 - 使用 `chat.update` API  
**UI 目标**: 展示 AI 的最终回答，支持 Markdown 格式

**设计说明**:
- 流式更新节流：1 次/秒
- 使用 Slack `metadata` 字段标记最终性
- 长消息自动拆分为多条连续消息
- 代码块保持语言标识高亮

---

### 5. Error 事件

**Block 类型**: `section`  
**聚合策略**: ❌ 不聚合 - 立即发送  
**UI 目标**: 醒目的错误提示

**视觉设计**:
- 使用 `> ` 引用格式包裹错误消息
- Emoji 区分：`:warning:` (普通错误) / `:x:` (危险拦截)
- 添加 "View Full Error" 按钮（Thread 展开完整错误）

**上下文一致性**:
- 错误消息发送到与原始消息相同的位置（主频道或 Thread）
- Thread 中的错误同时广播到主频道 (`reply_broadcast: true`)

---

### 6. Danger Block 事件

**Block 类型**: `section`  
**聚合策略**: ❌ 不聚合 - 立即发送  
**UI 目标**: 安全 WAF 拦截警告

**触发条件** (`internal/security/detector.go`):
- `rm -rf /`
- `mkfs`
- `dd if=/dev/zero`
- 其他危险命令模式

**设计说明**:
- 使用 `:x:` Emoji 标识安全拦截
- 引用格式展示被拦截命令
- 与普通错误使用相同 Block 结构（通过 Emoji 区分）

---

### 7. Session Stats 事件

**Block 类型**: `header` + `section` + `context`  
**聚合策略**: 最后发送 - 会话总结  
**UI 目标**: 丰富的统计信息卡片

**样式选项**:

| 样式 | 说明 | 展示字段 |
|------|------|----------|
| `Compact` (默认) | 单行摘要 | ⏱️ Duration + 📊 Tokens(In/Out) |
| `Card` | 卡片式 | Duration, Tokens, Cost, Model, Tools, Files |
| `Detailed` | 完整报告 | 所有指标，包括缓存、分节展示 |

**Compact 样式示例**:
```json
{
  "type": "context",
  "elements": [{
    "type": "mrkdwn",
    "text": "⏱️ 12.5s • 📊 1234 in / 567 out"
  }]
}
```

**设计说明**:
- Compact 样式仅展示 Duration 和 Tokens（最核心指标）
- 缓存信息仅在 Detailed 样式中展示
- 会话结束后立即发送

---

### 8. Permission Request 事件

**Block 类型**: `header` + `section` + `actions`  
**聚合策略**: ❌ 不聚合 - 需要用户立即决策  
**UI 目标**: 权限审批交互

**按钮设计**:
- `✅ Allow` (primary style) - 批准本次操作
- `🚫 Deny` (danger style) - 拒绝本次操作

**设计说明**:
- 仅展示基础 Allow/Deny 选项
- 无超时处理，无限期等待用户响应
- 审批后更新按钮为禁用状态 + 状态文本
- 命令截断阈值：500 字符

**Claude Code 权限模式**:
| 模式 | 参数 | 行为 |
|------|------|------|
| `bypass-permissions` | `--permission-mode=bypass-permissions` | 无需审批 |
| `default` | (默认) | 危险操作需要审批 |

---

## 事件聚合策略

### 策略矩阵

| 事件类型 | Block 类型 | 聚合策略 | 发送时机 | 说明 |
|---------|-----------|---------|---------|------|
| `thinking` | context | ❌ 不聚合 | 立即 | 用户需要即时反馈 |
| `tool_use` | section | ✅ 500ms 聚合 | 节流 | 多工具可合并展示 |
| `tool_result` | section+actions | ✅ 同类聚合 | 节流 | 可合并 + 展开按钮 |
| `answer` | section | ✅ 流式更新 | 1 次/秒 | 使用 `chat.update` |
| `error` | section | ❌ 不聚合 | 立即 | 关键错误信息 |
| `danger_block` | section | ❌ 不聚合 | 立即 | 安全拦截警告 |
| `session_stats` | header+section | 最后发送 | 单次 | 会话总结 |
| `permission_request` | header+actions | ❌ 不聚合 | 立即 | 需要用户决策 |

---

## Block Builder API 参考

### 核心方法

| 方法 | 参数 | 返回 | 用途 |
|------|------|------|------|
| `BuildThinkingBlock(content string)` | thinking 文本 | `[]map[string]any` | thinking 事件 |
| `BuildToolUseBlock(toolName, input string, truncated bool)` | 工具名、输入、是否截断 | `[]map[string]any` | tool_use 事件 |
| `BuildToolResultBlock(success bool, durationMs int64, output string, hasButton bool, toolName string, filePath ...string)` | 成功、时长、输出、按钮、工具名、文件路径 | `[]map[string]any` | tool_result 事件 |
| `BuildErrorBlock(message string, isDangerBlock bool)` | 错误消息、是否危险 | `[]map[string]any` | error/danger_block |
| `BuildAnswerBlock(content string)` | Markdown 内容 | `[]map[string]any` | answer 事件 |
| `BuildSessionStatsBlock(stats *event.SessionStatsData, style SessionStatsStyle)` | 统计数据、样式 | `[]map[string]any` | session_stats 事件 |
| `BuildPermissionRequestBlocks(req *provider.PermissionRequest, sessionID string)` | 权限请求、会话 ID | `[]map[string]any` | permission_request 事件 |

### 辅助函数

```go
// 工具 Emoji 映射
func getToolEmoji(toolName string) string

// 路径截断 (超过 50 字符时缩短)
func truncatePath(path string, maxLen int) string

// 时长格式化
func formatDuration(ms int64) string
```

---

## 最佳实践与限制

### Slack Block Kit 限制

| 限制类型 | 数值 | 说明 |
|---------|------|------|
| 单消息最大字符数 | 4000 | 包括所有 blocks 的文本内容 |
| 单消息最大 Blocks 数 | 50 | 超过会返回错误 |
| Section block fields 最大数 | 10 | 2 列布局，最多 5 行 |
| 按钮 value 最大长度 | 2000 | URL 编码后 |
| chat.update 速率限制 | ~1 次/秒 | 超过会返回 `rate_limited` |

### 消息发送最佳实践

1. **使用 `chat.postMessage` vs `chat.update`**:
   - 新消息 → `chat.postMessage`
   - 更新现有消息 → `chat.update` (需要 `ts` 时间戳)

2. **Thread 上下文一致性**:
   ```go
   // 检测原始消息是否在 Thread 中
   if threadTS, ok := metadata["thread_ts"]; ok && threadTS != "" {
       // 错误也发送到同一 Thread
       msg.Metadata["thread_ts"] = threadTS
       msg.Metadata["reply_broadcast"] = true  // 广播到主频道
   }
   ```

3. **长消息分块**: 超过 4000 字符自动拆分为多条连续消息

---

## 相关资源

### 官方文档
- [Slack Block Kit 文档](https://api.slack.com/block-kit)
- [Block Kit Builder](https://app.slack.com/block-kit-builder)
- [mrkdwn 格式参考](https://api.slack.com/reference/surfaces/formatting)

### 内部文件
| 文件 | 说明 |
|------|------|
| `chatapps/slack/block_builder.go` | Block Builder 完整实现 (1081 行) |
| `chatapps/engine_handler.go` | 事件处理核心逻辑 |
| `chatapps/slack/adapter.go` | Slack API 封装 |
| `provider/event.go` | ProviderEventType 枚举定义 |

---

**维护者**: HotPlex Team  
**最后审查**: 2026-02-26
