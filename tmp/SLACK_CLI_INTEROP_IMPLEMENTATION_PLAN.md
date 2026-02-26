# Slack 与 Claude Code CLI 运行时交互能力 - 实现计划

**创建日期**: 2026-02-27
**项目**: HotPlex
**文档版本**: v1.0

---

## 📋 实现概述

本计划基于对 Claude Code CLI 的深入调研，实现以下 Slack 与 CLI 运行时的交互能力：

| 功能 | 优先级 | 实现方式 |
|------|--------|----------|
| **Plan Mode** | P1 | 完整实现（生成 + 批准） |
| **AskUserQuestion** | P2 | 降级处理（文本提示） |
| **Output Styles** | P3 | 后续优化 |

---

## 🎯 功能详细设计

### 1. Plan Mode 完整实现

#### 1.1 背景知识

Plan Mode 是 Claude Code 的只读规划模式：

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐     ┌─────────────┐
│  激活 Plan  │ ──> │  研究和分析  │ ──> │  生成计划    │ ──> │  用户批准   │
│  Mode       │     │  (只读)      │     │              │     │  (stdin)    │
└─────────────┘     └──────────────┘     └─────────────┘     └─────────────┘
```

**关键事件**：
- `thinking` 事件 + `subtype="plan_generation"` → 计划生成阶段
- `tool_use` 事件 + `name="ExitPlanMode"` → 请求用户批准

#### 1.2 事件类型定义

**文件**: `provider/event.go`

```go
const (
    // ... 现有类型

    // EventTypePlanMode 计划模式 - Claude 生成计划但不执行
    // 触发条件: thinking 事件的 subtype="plan_generation"
    EventTypePlanMode ProviderEventType = "plan_mode"

    // EventTypeExitPlanMode 退出计划模式请求 - 等待用户批准
    // 触发条件: tool_use 事件的 name="ExitPlanMode"
    EventTypeExitPlanMode ProviderEventType = "exit_plan_mode"
)
```

#### 1.3 Provider 识别逻辑

**文件**: `provider/claude_provider.go`

```go
// 在 ParseEvent 方法中添加：

case "thinking", "status":
    // 检查是否为 Plan Mode
    if msg.Subtype == "plan_generation" {
        event.Type = EventTypePlanMode
        event.Content = msg.Status // 或从 msg.Content 提取
        // 可选：提取计划步骤信息
        return event, nil
    }
    // ... 现有 thinking 处理逻辑

case "tool_use":
    event.Type = EventTypeToolUse
    event.ToolName = msg.Name

    // 特殊处理 ExitPlanMode
    if msg.Name == "ExitPlanMode" {
        event.Type = EventTypeExitPlanMode
        // 从 input.plan 提取计划内容
        if plan, ok := msg.Input["plan"].(string); ok {
            event.Content = plan
        }
        return event, nil
    }
    // ... 现有 tool_use 处理逻辑
```

#### 1.4 Block Builder 方法

**文件**: `chatapps/slack/block_builder.go`

```go
// BuildPlanModeBlock 计划模式步骤展示
func (b *BlockBuilder) BuildPlanModeBlock(stepNumber, totalSteps int, planText string) []map[string]any {
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
func (b *BlockBuilder) BuildExitPlanModeBlock(sessionID, planSummary string) []map[string]any {
    return []map[string]any{
        {
            "type": "header",
            "text": plainText("⚠️ 准备执行计划"),
        },
        {
            "type": "section",
            "text": mrkdwnText(fmt.Sprintf("*计划摘要:*\n%s", planSummary)),
        },
        {
            "type": "section",
            "text": mrkdwnText("*即将执行的操作可能修改文件和运行命令。*"),
        },
        {
            "type": "actions",
            "block_id": fmt.Sprintf("plan_%s", sessionID),
            "elements": []map[string]any{
                {
                    "type":      "button",
                    "text":      plainText("✅ 批准并执行"),
                    "action_id": "plan_approve",
                    "style":     "primary",
                    "value":     fmt.Sprintf("approve:%s", sessionID),
                },
                {
                    "type":      "button",
                    "text":      plainText("📝 修改计划"),
                    "action_id": "plan_modify",
                    "value":     fmt.Sprintf("modify:%s", sessionID),
                },
                {
                    "type":      "button",
                    "text":      plainText("❌ 取消"),
                    "action_id": "plan_cancel",
                    "style":     "danger",
                    "value":     fmt.Sprintf("cancel:%s", sessionID),
                },
            },
        },
    }
}
```

#### 1.5 事件处理器

**文件**: `chatapps/engine_handler.go`

```go
// 在 Handle 方法的 switch 中添加：
func (c *StreamCallback) Handle(eventType string, data any) error {
    // ... 现有代码

    switch provider.ProviderEventType(eventType) {
    // ... 现有 case

    case provider.EventTypePlanMode:
        return c.handlePlanMode(data)
    case provider.EventTypeExitPlanMode:
        return c.handleExitPlanMode(data)
    }
    // ...
}

func (c *StreamCallback) handlePlanMode(data any) error {
    var planContent string
    var stepNum, totalSteps int

    if m, ok := data.(*event.EventWithMeta); ok {
        planContent = m.EventData
        if m.Meta != nil {
            stepNum = m.Meta.CurrentStep
            totalSteps = m.Meta.TotalSteps
        }
    }

    if planContent == "" {
        return nil
    }

    // 默认值
    if totalSteps == 0 {
        totalSteps = 1
    }
    if stepNum == 0 {
        stepNum = 1
    }

    blocks := c.blockBuilder.BuildPlanModeBlock(stepNum, totalSteps, planContent)
    return c.sendBlockMessage(string(provider.EventTypePlanMode), blocks, false)
}

func (c *StreamCallback) handleExitPlanMode(data any) error {
    var planSummary string

    if m, ok := data.(*event.EventWithMeta); ok {
        planSummary = m.EventData
    }

    if planSummary == "" {
        planSummary = "计划内容将在批准后执行。"
    }

    blocks := c.blockBuilder.BuildExitPlanModeBlock(c.sessionID, planSummary)
    return c.sendBlockMessage(string(provider.EventTypeExitPlanMode), blocks, false)
}
```

#### 1.6 Slack 回调处理

**文件**: `chatapps/slack/adapter.go`

```go
// 在 handleBlockAction 方法中添加：

case "plan_approve":
    // 用户批准计划，发送 stdin 响应
    return a.sendPlanDecision(ctx, channelID, sessionID, "allow")

case "plan_modify":
    // 打开计划修改 modal
    return a.openPlanModifyModal(ctx, triggerID, channelID, sessionID)

case "plan_cancel":
    // 用户取消计划
    return a.sendPlanDecision(ctx, channelID, sessionID, "deny")

// 新增方法：
func (a *Adapter) sendPlanDecision(ctx context.Context, channelID, sessionID, decision string) error {
    // 通过 Engine 发送 stdin 响应
    // 格式待实验确定，初步假设类似 Permission Request：
    // {"behavior": "allow"} 或 {"behavior": "deny"}
    return a.engine.SendStdinResponse(sessionID, map[string]string{
        "behavior": decision,
    })
}

func (a *Adapter) openPlanModifyModal(ctx context.Context, triggerID, channelID, sessionID string) error {
    // 打开 Slack Modal 让用户输入修改建议
    // 用户提交后，将修改建议作为新消息发送给 CLI
    // TODO: 实现 Modal 定义和提交处理
    return nil
}
```

---

### 2. AskUserQuestion 降级处理

#### 2.1 背景知识

AskUserQuestion 在 headless 模式 (`-p`) 下**不支持**，因此采用降级策略：
- 识别 `tool_use` 事件中 `name="AskUserQuestion"`
- 展示为普通文本提示
- 用户通过回复消息来回答（非结构化交互）

#### 2.2 Provider 识别

**文件**: `provider/claude_provider.go`

```go
case "tool_use":
    event.Type = EventTypeToolUse
    event.ToolName = msg.Name

    // 特殊处理 AskUserQuestion (降级为文本提示)
    if msg.Name == "AskUserQuestion" {
        event.Type = EventTypeAskUserQuestion // 新增事件类型
        // 提取问题内容
        if question, ok := msg.Input["question"].(string); ok {
            event.Content = question
        }
        // 提取选项（用于展示）
        if options, ok := msg.Input["options"].([]any); ok {
            event.Metadata = map[string]any{
                "options": options,
            }
        }
        return event, nil
    }
```

#### 2.3 事件类型定义

**文件**: `provider/event.go`

```go
const (
    // ... 现有类型

    // EventTypeAskUserQuestion Claude 请求用户澄清问题
    // 注意: headless 模式下不支持 stdin 响应，降级为文本提示
    EventTypeAskUserQuestion ProviderEventType = "ask_user_question"
)
```

#### 2.4 Block Builder

**文件**: `chatapps/slack/block_builder.go`

```go
// BuildAskUserQuestionPromptBlock 降级处理：展示为文本提示
func (b *BlockBuilder) BuildAskUserQuestionPromptBlock(question string, options []any) []map[string]any {
    blocks := []map[string]any{
        {
            "type": "header",
            "text": plainText("❓ Claude 需要您的输入"),
        },
        {
            "type": "section",
            "text": mrkdwnText(fmt.Sprintf("*问题:* %s", question)),
        },
    }

    // 展示选项（纯文本，无交互按钮）
    if len(options) > 0 {
        var optionTexts []string
        for _, opt := range options {
            if m, ok := opt.(map[string]any); ok {
                if label, ok := m["label"].(string); ok {
                    optionTexts = append(optionTexts, fmt.Sprintf("• %s", label))
                }
            }
        }
        if len(optionTexts) > 0 {
            blocks = append(blocks, map[string]any{
                "type": "section",
                "text": mrkdwnText(fmt.Sprintf("*选项:*\n%s", strings.Join(optionTexts, "\n"))),
            })
        }
    }

    // 提示用户如何回答
    blocks = append(blocks, map[string]any{
        "type": "context",
        "elements": []map[string]any{
            mrkdwnText("💡 _请直接回复此消息来回答问题_"),
        },
    })

    return blocks
}
```

#### 2.5 事件处理器

**文件**: `chatapps/engine_handler.go`

```go
func (c *StreamCallback) Handle(eventType string, data any) error {
    // ... 现有代码

    case provider.EventTypeAskUserQuestion:
        return c.handleAskUserQuestion(data)
}

func (c *StreamCallback) handleAskUserQuestion(data any) error {
    var question string
    var options []any

    if m, ok := data.(*event.EventWithMeta); ok {
        question = m.EventData
        if m.Meta != nil && m.Meta.Metadata != nil {
            if opts, ok := m.Meta.Metadata["options"].([]any); ok {
                options = opts
            }
        }
    }

    if question == "" {
        return nil
    }

    blocks := c.blockBuilder.BuildAskUserQuestionPromptBlock(question, options)
    return c.sendBlockMessage(string(provider.EventTypeAskUserQuestion), blocks, false)
}
```

---

## 🔬 stdin 响应实验计划

### 目标

确定 ExitPlanMode 工具的 stdin 响应格式。

### 实验步骤

1. **准备测试环境**
   ```bash
   mkdir -p /tmp/plan-mode-test/.claude
   cd /tmp/plan-mode-test
   echo '{"planMode": true}' > .claude/settings.json
   ```

2. **触发 Plan Mode**
   ```bash
   claude -p "分析当前目录并制定改进计划" --output-format stream-json 2>&1 | tee plan_output.jsonl
   ```

3. **观察输出**
   - 查找 `type: "tool_use"` 和 `name: "ExitPlanMode"`
   - 记录完整的 JSON 结构

4. **测试 stdin 响应**
   - 假设格式 1: `{"behavior": "allow"}`
   - 假设格式 2: `{"type": "tool_result", "tool_use_id": "...", "content": "approved"}`
   - 验证哪种格式有效

5. **文档化结果**
   - 记录有效的响应格式
   - 更新实现代码

### 预期结果

基于 Permission Request 的相似性，初步假设：
```json
{"behavior": "allow"}
```
或
```json
{"behavior": "deny", "message": "用户取消"}
```

---

## 📊 任务依赖图

```
#1 事件类型定义
 │
 └─▶ #2 Provider 识别逻辑
      │
      └─▶ #3 Block Builder 方法
           │
           └─▶ #4 事件处理器
                │         │
                │         └─▶ #7 AskUserQuestion 降级处理
                │
    #6 stdin 实验 ─└─▶ #5 Slack 回调处理
                            │
                            └─▶ #8 单元测试
```

---

## 📁 文件修改清单

| 文件 | 修改类型 | 说明 |
|------|----------|------|
| `provider/event.go` | 新增 | 添加 EventTypePlanMode, EventTypeExitPlanMode, EventTypeAskUserQuestion |
| `provider/claude_provider.go` | 修改 | 添加 subtype 和特殊工具识别逻辑 |
| `chatapps/slack/block_builder.go` | 新增 | 添加 3 个 Block Builder 方法 |
| `chatapps/engine_handler.go` | 新增 | 添加 3 个事件处理方法 |
| `chatapps/slack/adapter.go` | 修改 | 添加回调处理逻辑 |
| `provider/*_test.go` | 新增 | 单元测试 |

---

## ✅ 验收标准

1. **Plan Mode 生成阶段**
   - [ ] 能识别 `thinking` 事件的 `subtype="plan_generation"`
   - [ ] 能在 Slack 中展示计划步骤

2. **Plan Mode 批准阶段**
   - [ ] 能识别 `ExitPlanMode` 工具调用
   - [ ] 能展示批准/修改/取消按钮
   - [ ] 批准按钮能发送正确的 stdin 响应
   - [ ] 取消按钮能正确终止流程

3. **AskUserQuestion 降级**
   - [ ] 能识别 `AskUserQuestion` 工具调用
   - [ ] 能展示问题和选项
   - [ ] 有明确的用户指引（回复消息来回答）

4. **测试覆盖**
   - [ ] 单元测试覆盖所有新增逻辑
   - [ ] `go test ./...` 通过
   - [ ] `go build ./...` 通过

---

**维护者**: HotPlex Team
**最后更新**: 2026-02-27
