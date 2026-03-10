# Slack Adapter Package

This package implements a high-performance, AI-native Slack adapter for the HotPlex engine. It provides seamless integration with Slack, supporting rich visual components through Block Kit, native streaming via Assistant Threads, and a refined "AI-is-Alive" perception system.

## ⚙️ Required Environment Variables

**Before using this adapter, you MUST configure the following environment variables in your `.env` file:**

```bash
# 1. Bot User ID - Your bot's user ID (REQUIRED)
#    Used to identify the bot's own messages and prevent infinite loops
#    Get it by: curl -X POST "https://slack.com/api/auth.test" \
#                 -H "Authorization: Bearer xoxb-YOUR_BOT_TOKEN" | jq '.user_id'
HOTPLEX_SLACK_BOT_USER_ID=UXXXXXXXXXX

# 2. Primary Owner - Your Slack User ID (REQUIRED)
#    Specifies the bot owner when owner_policy is set to "trusted"
#    If not configured, ALL messages will be rejected (including DMs)
#    Get it from: Slack Profile -> More -> Copy member ID
HOTPLEX_SLACK_PRIMARY_OWNER=UXXXXXXXXXX
```

**Why are these IDs required?**

- **HOTPLEX_SLACK_BOT_USER_ID**:
  - Identifies messages sent by the bot itself
  - Prevents infinite loops (bot responding to its own messages)
  - Required for @mention detection in multibot mode

- **HOTPLEX_SLACK_PRIMARY_OWNER**:
  - Specifies the bot's administrator/owner
  - When `owner_policy: trusted`, only configured users can use the bot
  - If missing, all messages are rejected with "blocked by policy"

**Without these two IDs, the bot will not respond to any messages!**

## 🏗️ Message Data Flow

The following diagram illustrates how signals flow from the Engine through the integration layer to the Slack UI:

```text
       [ HotPlex Engine ]
               |
               v (Event Stream)
      [ engine_handler.go ]
               |
  +------------+------------+---------------------------+
  | (A) Logic Signals       | (B) Content Signals       | (C) Control Signals
  v                         v                           v
[ Status Bar / API ]     [ Processor Chain ]         [ Interaction Mgr ]
  |                         |                           |
  | assistant:write         | Threading / Formatting    | Intercept / Approve
  | (Real-time Feedback)    | Space Folding             | (WAF Closed Loop)
  |                         |                           |
  +------------+------------+---------------------------+
               |
               v (Slack SDK Payload)
   +---------------------------------------+
   |             [ SLACK UI ]              |
   |                                       |
   | 🧠 思考中... [Native Status Bar]       | <--- (A)
   |                                       |
   | +-----------------------------------+ |
   | | 🔧 Tool: bash                     | | <--- (B)
   | | 📋 output in thread reply...      | |
   | +-----------------------------------+ |
   |                                       |
   | ✋ Danger! Confirm Execution? [Warn]  | <--- (C)
   +---------------------------------------+
```

## Core Modules

### 🔌 Connectivity
- **[adapter.go](adapter.go)**: The central entry point. Manages the adapter lifecycle, coordinates logic, and now strictly implements the Native Status API without legacy fallbacks.
- **[socketmode.go](socketmode.go)**: Handles WebSocket-based connections using Slack Socket Mode (preferred).
- **[events.go](events.go)**: Manages HTTP-based Events API callbacks and signature verification.

### 💬 Messaging & UI
- **[messages.go](messages.go)**: Core messaging logic, including standard posts, updates, and **Native Assistant Status** API calls (`assistant.threads.setStatus`).
- **[builder.go](builder.go)**: A sophisticated factory that translates engine `ChatMessage` objects into Slack Block Kit components. It follows an **"Absolute Black Hole"** policy to filter out redundant system noise.
- **[formatting.go](formatting.go)**: Advanced Markdown-to-Mrkdwn converter with support for Slack-specific escapes and blocks.
- **[streaming_writer.go](streaming_writer.go)**: Implements `io.Writer` for character-by-character output via Slack's native streaming UI.

### ⚡ Interactions
- **[slash_commands.go](slash_commands.go)**: Processes slash commands (e.g., `/reset`, `/dc`) with rate limiting and context awareness.
- **[interactive.go](interactive.go)**: Handles interactive callbacks for **WAF Danger Blocks** and permission requests, enabling Human-in-the-loop (HITL) workflows.

### 🛡️ Security & Reliability
- **[security.go](security.go)**: Implements request signature verification, URL sanitization, and PII masking for error messages.
- **[validator.go](validator.go)**: Strict Block Kit schema validation to prevent API errors during complex card rendering.
- **[config.go](config.go)**: Workspace-level configuration including DMPolicy and GroupPolicy enforcement.
- **[rate_limiter.go](rate_limiter.go)**: Per-user token-bucket rate limiting for interactive elements.

## Key Features

- **AI-Native UX (2026 Edition)**: Strictly utilizes the **Assistant Status API** for real-time progress (Thinking, Tool Use, Planning), keeping the chat history clean.
- **Absolute Black Hole Policy**: Silently drops system-level logs and redundant user reflections within the integration layer to minimize UI noise.
- **WAF Closed Loop**: Integrated security interception with interactive confirmation cards that block the execution until human approval is received.
- **Space Folding**: High-volume tool outputs are automatically folded into thread replies, preventing main channel pollution.
- **Clean Architecture**: Removed all deprecated Fallback/Bubble simulate logic, relying exclusively on Slack Native capabilities for a premium experience.
