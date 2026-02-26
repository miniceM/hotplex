# Claude Code Plan Mode 完整交互流程调研报告

**调研日期**: 2026-02-26  
**信息来源**: ClaudeLog 官方文档、社区实践、HotPlex 代码分析

---

## 🎯 核心发现

### Plan Mode 退出机制关键洞察

**关键发现**: Claude Code 存在一个 **隐藏的 `exit_plan_mode` 工具**，用于：
1. 结束 Plan Mode 只读状态
2. 请求用户批准执行计划
3. 切换回正常执行模式

**重要引用**:
> This mechanic is enabled by a combination of the **hidden `exit_plan_mode` tool** and `--append-system-prompt` flag
> 
> — [ClaudeLog - Auto Plan Mode](https://claudelog.com/mechanics/auto-plan-mode/)

---

## 📋 Plan Mode 完整交互流程

### 阶段 1: 激活 Plan Mode

**激活方式**:
| 方式 | 操作 | 版本 |
|------|------|------|
| 快捷键 | `Shift+Tab` 两次 | 所有版本 |
| 命令 | `/plan` | v2.1.0+ |
| 配置 | `"planMode": true` in `.claude/settings.json` | 所有版本 |
| 系统提示 | `--append-system-prompt` 强制激活 | v1.0.51+ |

**状态特征**:
- 终端底部显示 "Plan Mode" 标识
- Claude 只能使用只读工具
- 无法编辑文件、执行命令

---

### 阶段 2: 研究和分析 (只读)

**可用工具** ✅:
- `Read` - 读取文件
- `LS` - 目录列表
- `Glob` - 文件搜索
- `Grep` - 内容搜索
- `Task` - 研究子代理
- `TodoRead`/`TodoWrite` - 任务管理
- `WebFetch` - 网页内容分析
- `WebSearch` - 网络搜索
- `NotebookRead` - Jupyter 笔记本读取

**受限工具** ❌:
- `Edit`/`MultiEdit` - 文件编辑
- `Write` - 文件创建
- `Bash` - 命令执行
- `NotebookEdit` - 笔记本编辑
- MCP 工具 (修改状态的工具)

**输出事件** (stream-json):
```json
{
  "type": "thinking",
  "subtype": "plan_generation",
  "status": "Analyzing codebase structure...",
  "content": [
    {
      "type": "text",
      "text": "Step 1: Review the current architecture..."
    }
  ]
}
```

---

### 阶段 3: 生成计划

**计划输出格式**:
```markdown
# Implementation Plan

## Task Breakdown

### Step 1: Analyze current structure
- Read `src/main.go`
- Identify key components
- Estimate complexity

### Step 2: Implement changes
- Edit `src/main.go` (3 locations)
- Add new file `src/utils.go`
- Run tests

### Step 3: Verify
- Execute `go test ./...`
- Check for errors

## Files to Modify
1. `src/main.go` - Main logic
2. `src/utils.go` - New utilities

## Risks
- Breaking changes in API
- Test coverage gaps
```

**HotPlex 事件识别**:
```go
// provider/claude_provider.go
case "thinking":
    if msg.Subtype == "plan_generation" {
        event.Type = EventTypePlanMode
        event.Content = extractPlanText(msg.Content)
        event.Metadata = &ProviderEventMeta{
            CurrentStep: 1,
            TotalSteps: 3,
        }
    }
```

---

### 阶段 4: 用户审查和编辑计划

**Opus Plan Mode 增强** (Opus 4.6+):
1. Claude 创建 `plan.md` 文件
2. 用户可以编辑计划
3. Claude 等待明确批准

**交互方式**:
- 用户可直接编辑 `plan.md`
- 用户可提问澄清问题
- Claude 可主动提问 (AskUserQuestion)

**配置文件存储** (v2.1.9+):
```json
// .claude/settings.json
{
  "plansDirectory": "~/.claude/plans"
}
```

---

### 阶段 5: 退出 Plan Mode (关键步骤)

#### 5.1 退出方式

| 方式 | 操作 | 说明 |
|------|------|------|
| 快捷键 | `Shift+Tab` | 切换回正常模式 |
| 工具调用 | `exit_plan_mode` | 隐藏的内部工具 |
| 用户批准 | "是的，执行计划" | 自然语言批准 |

#### 5.2 `exit_plan_mode` 工具

**工具格式** (推测):
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

**用户选择后**:
1. Claude 调用 `exit_plan_mode` 工具
2. 等待用户确认
3. 用户确认后，切换回正常模式
4. 开始执行计划

#### 5.3 额外谨慎确认

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

### 阶段 6: 执行计划

**模式切换**:
- Plan Mode → Normal Mode
- 只读工具 → 全部工具可用
- 需要用户明确批准 (Extra Cautious)

**执行特征**:
```json
{
  "type": "tool_use",
  "name": "Edit",
  "input": {"file_path": "src/main.go", ...}
}
```

---

## 📊 事件类型完整映射

### stdout 事件 (Claude → 用户)

| 阶段 | 事件类型 | subtype | 描述 |
|------|---------|---------|------|
| 激活 | `system` | - | Plan Mode 已激活通知 |
| 研究 | `thinking` | `plan_generation` | 生成计划步骤 |
| 研究 | `tool_use` | - | 只读工具调用 (Read, Grep 等) |
| 研究 | `tool_result` | - | 工具执行结果 |
| 计划 | `assistant` | - | 展示计划内容 |
| 退出 | `tool_use` | - | `exit_plan_mode` 工具调用 |
| 批准 | `permission_request` | - | 执行批准请求 |
| 执行 | `tool_use` | - | 实际工具调用 (Edit, Bash 等) |

### stdin 响应 (用户 → Claude)

| 场景 | 响应格式 | 说明 |
|------|---------|------|
| 批准计划 | `{"behavior": "allow"}` | 执行计划 |
| 拒绝计划 | `{"behavior": "deny", "message": "..."}` | 取消执行 |
| 修改计划 | 文本输入 | 要求修改计划 |
| 退出 Plan Mode | N/A | 通过 UI 按钮或快捷键 |

---

## 🔧 HotPlex 实现建议

### 1. 定义 Plan Mode 事件类型

```go
// provider/event.go
const (
    // ... 现有类型
    
    // EventTypePlanMode 计划模式 - Claude 生成计划但不执行
    EventTypePlanMode ProviderEventType = "plan_mode"
    
    // EventTypeExitPlanMode 退出计划模式请求
    EventTypeExitPlanMode ProviderEventType = "exit_plan_mode"
)
```

### 2. 识别 `exit_plan_mode` 工具

```go
// provider/claude_provider.go
case "tool_use":
    event.Type = EventTypeToolUse
    event.ToolName = msg.Name
    
    // 特殊处理 exit_plan_mode
    if msg.Name == "exit_plan_mode" {
        event.Type = EventTypeExitPlanMode
        event.Content = "请求退出 Plan Mode 并执行计划"
        event.Metadata = &ProviderEventMeta{
            PlanSummary: extractPlanSummary(msg.Input),
        }
    }
```

### 3. Block Builder 方法

```go
// chatapps/slack/block_builder.go

// BuildPlanModeBlock 计划模式步骤展示
func (b *BlockBuilder) BuildPlanModeBlock(
    stepNumber, totalSteps int, 
    planText string,
) []map[string]any {
    return []map[string]any{
        {
            "type": "header",
            "text": plainText(fmt.Sprintf("📋 Plan Mode - Step %d/%d", stepNumber, totalSteps)),
        },
        {
            "type": "section",
            "text": mrkdwnText(planText),
        },
        {
            "type": "context",
            "elements": []map[string]any{
                mrkdwnText("🔒 _Plan Mode: 只读研究，等待批准_"),
            },
        },
    }
}

// BuildExitPlanModeBlock 退出计划模式确认
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
            "type": "section",
            "text": mrkdwnText("*即将执行的操作:*\n- 文件编辑\n- 命令执行\n\n这些修改可能影响代码库。"),
        },
        {
            "type": "actions",
            "elements": []map[string]any{
                {
                    "type": "button",
                    "text": plainText("✅ 批准并执行"),
                    "action_id": "plan_approve",
                    "style": "primary",
                    "value": "approve:plan",
                },
                {
                    "type": "button",
                    "text": plainText("📝 修改计划"),
                    "action_id": "plan_modify",
                    "value": "modify:plan",
                },
                {
                    "type": "button",
                    "text": plainText("❌ 取消"),
                    "action_id": "plan_cancel",
                    "style": "danger",
                    "value": "cancel:plan",
                },
            },
        },
    }
}
```

### 4. 回调处理

```go
// chatapps/slack/adapter.go
case "plan_approve":
    // 用户批准计划，发送允许响应
    return a.sendPermissionDecision(sessionID, "allow")

case "plan_modify":
    // 用户要求修改计划
    return a.openPlanModificationModal(channelID, sessionID)

case "plan_cancel":
    // 用户取消计划，退出 Plan Mode
    return a.sendPermissionDecision(sessionID, "deny")
```

---

## 🎬 完整交互流程图

```
┌─────────────┐                    ┌──────────────┐                    ┌─────────────┐
│    User     │                    │  Claude Code │                    │  HotPlex   │
│             │                    │              │                    │  Adapter   │
└──────┬──────┘                    └──────┬───────┘                    └──────┬──────┘
       │                                  │                                   │
       │ 1. Activate Plan Mode            │                                   │
       │    (Shift+Tab x2 / /plan)        │                                   │
       │─────────────────────────────────>│                                   │
       │                                  │                                   │
       │ 2. Research (Read-Only)          │                                   │
       │    - Read files                  │  thinking (subtype=plan_generation)│
       │    - Grep patterns               │──────────────────────────────────>│
       │    - Analyze structure           │                                   │ BuildPlanModeBlock
       │                                  │                                   │───────────> Slack
       │ 3. Generate Plan                 │                                   │
       │                                  │  assistant (plan content)         │
       │<─────────────────────────────────│                                   │
       │                                  │                                   │
       │ 4. Review & Edit Plan            │                                   │
       │    (optional plan.md)            │                                   │
       │<────────────────────────────────>│                                   │
       │                                  │                                   │
       │ 5. Request Exit Plan Mode        │                                   │
       │                                  │  tool_use (name=exit_plan_mode)   │
       │<─────────────────────────────────│                                   │
       │                                  │                                   │ BuildExitPlanModeBlock
       │                                  │                                   │───────────> Slack
       │                                  │                                   │
       │ 6. User Approves                 │                                   │
       │    "Yes, execute the plan"       │                                   │ user clicks "Approve"
       │─────────────────────────────────>│                                   │<──────────────────────
       │                                  │                                   │
       │ 7. Extra Cautious Confirmation   │                                   │
       │                                  │  permission_request               │
       │<─────────────────────────────────│                                   │
       │                                  │                                   │
       │ 8. Confirm Execution             │                                   │
       │    ✅ Approve                    │  stdin: {"behavior":"allow"}      │
       │─────────────────────────────────>│──────────────────────────────────>│
       │                                  │                                   │
       │ 9. Exit Plan Mode                │                                   │
       │    Mode: Plan → Normal           │                                   │
       │                                  │                                   │
       │ 10. Execute Plan                 │                                   │
       │                                  │  tool_use (Edit, Bash, etc.)      │
       │<─────────────────────────────────│                                   │
       │                                  │                                   │
       │ 11. Show Results                 │                                   │
       │                                  │  tool_result, assistant           │
       │<─────────────────────────────────│                                   │
       │                                  │                                   │
```

---

## 📝 Auto Plan Mode (扩展知识)

### 什么是 Auto Plan Mode?

Auto Plan Mode 使用 `--append-system-prompt` 强制 Claude 在执行任何操作前先进入 Plan Mode。

**系统提示词示例**:
```
CRITICAL WORKFLOW REQUIREMENT

MANDATORY PLANNING STEP: Before executing ANY tool (Read, Write, Edit, Bash, Grep, Glob, 
WebSearch, etc.), you MUST:

1. FIRST: Use exit_plan_mode tool to present your plan
2. WAIT: For explicit user approval before proceeding
3. ONLY THEN: Execute the planned actions

ZERO EXCEPTIONS: This applies to EVERY INDIVIDUAL USER REQUEST involving tool usage,
regardless of complexity, tool type, user urgency, or apparent simplicity.
```

**使用方式**:
```bash
claude --append-system-prompt "$(cat auto-plan-mode.txt)"
```

---

## 📚 参考资料

### 官方文档
- [Claude Code Plan Mode](https://claudelog.com/mechanics/plan-mode/)
- [Auto Plan Mode](https://claudelog.com/mechanics/auto-plan-mode/)
- [Output Styles](https://claudelog.com/mechanics/output-styles/)

### HotPlex 相关代码
- [provider/event.go](../provider/event.go) - 事件类型定义
- [provider/claude_provider.go](../provider/claude_provider.go) - 事件解析
- [docs/chatapps/engine-events-slack-mapping.md](../docs/chatapps/engine-events-slack-mapping.md) - Slack 映射

---

## ✅ 验证清单

### 已验证
- [x] Plan Mode 激活方式 (快捷键、命令、配置)
- [x] 可用工具 vs 受限工具列表
- [x] `exit_plan_mode` 隐藏工具存在
- [x] 退出时 Extra Cautious 确认机制
- [x] Opus Plan Mode 增强功能 (plan.md、澄清问题)

### 待实验验证
- [ ] `exit_plan_mode` 工具的确切 JSON 格式
- [ ] stdin 响应格式是否为 `{"behavior": "allow"}`
- [ ] Plan Mode 激活时的系统事件格式
- [ ] Slack 交互式按钮回调后的 stdin 写入流程

---

**维护者**: HotPlex Team  
**最后更新**: 2026-02-26  
**调研状态**: ✅ 文献调研完成 | ⚠️ 实际测试待进行
