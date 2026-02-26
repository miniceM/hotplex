# Claude Code Plan Mode 调研报告总结

**调研完成日期**: 2026-02-26  
**关键发现**: 存在隐藏的 `exit_plan_mode` 工具

---

## 🎯 核心问题

**用户询问**: Plan Mode 阶段结束后，会让用户选择，然后退出 plan mode，调研此功能

---

## ✅ 调研结论

### Plan Mode 完整生命周期

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐     ┌─────────────┐
│  激活 Plan  │ ──> │  研究和分析  │ ──> │  生成计划    │ ──> │  用户审查   │
│  Mode       │     │  (只读)      │     │              │     │  和编辑     │
└─────────────┘     └──────────────┘     └─────────────┘     └─────────────┘
                                                                      │
                                                                      ▼
┌─────────────┐     ┌──────────────┐     ┌─────────────┐     ┌─────────────┐
│  执行计划   │ <── │  用户批准    │ <── │  额外谨慎    │ <── │  调用       │
│  (Normal)   │     │  (stdin)     │     │  确认        │     │  exit_plan  │
└─────────────┘     └──────────────┘     └─────────────┘     └─────────────┘
```

---

## 🔑 关键发现

### 1. Plan Mode 激活方式

| 方式 | 操作 | 版本 |
|------|------|------|
| **快捷键** | `Shift+Tab` 两次 | 所有版本 |
| **命令** | `/plan` | v2.1.0+ |
| **配置** | `.claude/settings.json` + `"planMode": true` | 所有版本 |
| **系统提示** | `--append-system-prompt` | v1.0.51+ |

### 2. Plan Mode 期间工具限制

**可用工具** (只读):
- `Read`, `LS`, `Glob`, `Grep`
- `Task` (研究子代理)
- `TodoRead`/`TodoWrite`
- `WebFetch`, `WebSearch`
- `NotebookRead`

**受限工具** (需批准后):
- `Edit`/`MultiEdit`
- `Write`
- `Bash`
- `NotebookEdit`

### 3. exit_plan_mode 工具 (关键发现)

**重要引用**:
> This mechanic is enabled by a combination of the **hidden `exit_plan_mode` tool** and `--append-system-prompt` flag
> 
> — [ClaudeLog - Auto Plan Mode](https://claudelog.com/mechanics/auto-plan-mode/)

**工具行为**:
1. Claude 完成计划后调用 `exit_plan_mode`
2. 等待用户明确批准
3. 用户批准后切换回正常模式

**推测工具格式**:
```json
{
  "type": "tool_use",
  "name": "exit_plan_mode",
  "id": "exit_plan_123",
  "input": {
    "plan_summary": "Implement feature X with 3 steps",
    "files_to_modify": ["src/main.go", "src/utils.go"],
    "commands_to_run": ["go test ./..."],
    "approval_required": true
  }
}
```

### 4. 退出时的 Extra Cautious 确认

**重要引用**:
> When exiting plan mode, Claude is **extra cautious** and will ask for **additional confirmation** about the task he is about to execute.
> 
> — ClaudeLog

**确认提示示例**:
```
我准备执行以下操作：

1. 编辑 src/main.go (3 处修改)
2. 创建 src/utils.go
3. 运行 go test ./...

这些修改将影响 API 接口。确认继续吗？

[✅ 批准并执行]  [📝 修改计划]  [❌ 取消]
```

---

## 📊 stdout 事件映射

| 阶段 | 事件类型 | subtype | 描述 |
|------|---------|---------|------|
| 激活 | `system` | - | Plan Mode 已激活 |
| 研究 | `thinking` | `plan_generation` | 生成计划步骤 |
| 研究 | `tool_use` | - | 只读工具调用 |
| 计划 | `assistant` | - | 展示计划内容 |
| 退出 | `tool_use` | - | `exit_plan_mode` 调用 |
| 批准 | `permission_request` | - | 执行批准请求 |
| 执行 | `tool_use` | - | 实际工具调用 |

---

## 🔧 HotPlex 实现建议

### 1. 新增事件类型

```go
// provider/event.go
const (
    // 计划模式 - Claude 生成计划但不执行
    EventTypePlanMode ProviderEventType = "plan_mode"
    
    // 退出计划模式请求
    EventTypeExitPlanMode ProviderEventType = "exit_plan_mode"
)
```

### 2. 识别 exit_plan_mode 工具

```go
// provider/claude_provider.go
case "tool_use":
    event.Type = EventTypeToolUse
    event.ToolName = msg.Name
    
    // 特殊处理 exit_plan_mode
    if msg.Name == "exit_plan_mode" {
        event.Type = EventTypeExitPlanMode
        event.Content = "请求退出 Plan Mode 并执行计划"
    }
```

### 3. Slack UI 块

```go
// chatapps/slack/block_builder.go

// BuildExitPlanModeBlock 退出 Plan Mode 确认
func (b *BlockBuilder) BuildExitPlanModeBlock(planSummary string) []map[string]any {
    return []map[string]any{
        {
            "type": "header",
            "text": plainText("⚠️ 准备退出 Plan Mode"),
        },
        {
            "type": "section",
            "text": mrkdwnText(fmt.Sprintf("*计划摘要:*\n%s", planSummary)),
        },
        {
            "type": "actions",
            "elements": []map[string]any{
                {
                    "type": "button",
                    "text": plainText("✅ 批准并执行"),
                    "action_id": "plan_approve",
                    "style": "primary",
                },
                {
                    "type": "button",
                    "text": plainText("📝 修改计划"),
                    "action_id": "plan_modify",
                },
                {
                    "type": "button",
                    "text": plainText("❌ 取消"),
                    "action_id": "plan_cancel",
                    "style": "danger",
                },
            },
        },
    }
}
```

### 4. 回调处理

```go
// chatapps/slack/adapter.go
switch actionID {
case "plan_approve":
    // 发送允许响应到 stdin
    return a.sendPermissionDecision(sessionID, "allow")
    
case "plan_modify":
    // 打开计划修改 modal
    return a.openPlanModificationModal(channelID, sessionID)
    
case "plan_cancel":
    // 发送拒绝响应
    return a.sendPermissionDecision(sessionID, "deny")
}
```

---

## 📝 Auto Plan Mode (扩展)

### 什么是 Auto Plan Mode?

使用 `--append-system-prompt` 强制 Claude 在执行任何操作前先进入 Plan Mode。

**系统提示词**:
```
CRITICAL WORKFLOW REQUIREMENT

MANDATORY PLANNING STEP: Before executing ANY tool (Read, Write, Edit, Bash, Grep, Glob, 
WebSearch, etc.), you MUST:

1. FIRST: Use exit_plan_mode tool to present your plan
2. WAIT: For explicit user approval before proceeding
3. ONLY THEN: Execute the planned actions

ZERO EXCEPTIONS
```

**使用方式**:
```bash
claude --append-system-prompt "$(cat auto-plan-mode.txt)"
```

---

## 📚 详细文档

- [完整交互流程调研](./docs/verification/claude-plan-mode-interaction-flow.md)
- [stdin 支持验证](./docs/verification/claude-stdin-support-verification.md)
- [功能实现报告](./docs/verification/claude-features-verification-report.md)

---

## ✅ 验证清单

### 已完成
- [x] Plan Mode 激活方式调研
- [x] 可用工具 vs 受限工具列表
- [x] `exit_plan_mode` 隐藏工具发现
- [x] Extra Cautious 确认机制确认
- [x] Opus Plan Mode 增强功能调研
- [x] HotPlex 实现建议提供

### 待实验
- [ ] `exit_plan_mode` 工具的确切 JSON 格式
- [ ] stdin 响应流程验证
- [ ] Slack 交互式回调集成测试

---

**维护者**: HotPlex Team  
**最后更新**: 2026-02-26  
**调研状态**: ✅ 文献调研完成 | ⚠️ 实际测试待进行
