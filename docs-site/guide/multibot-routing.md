# MultiBot @ Routing

## Intelligent Multi-Bot Message Routing

HotPlex supports running multiple bots in a single Slack channel with intelligent @mention-based routing. This document explains how to configure and use the MultiBot routing feature.

---

## Overview

In a multi-bot environment, you may have multiple HotPlex instances (bots) running in the same Slack channel. Without proper routing, this leads to:

- **Duplicate responses**: All bots respond to every message
- **Confusion**: Users don't know which bot they're talking to
- **Resource waste**: Unnecessary processing by unmentioned bots

The **MultiBot @ Routing** feature solves this by:
- Responding only when the bot is explicitly @mentioned
- Broadcasting a polite message when no @mention is present (letting users know multiple bots are available)

---

## Configuration

### Enable MultiBot Mode

In your bot's YAML configuration (`chatapps/configs/slack.yaml`):

```yaml
# Group policy: Controls how the bot handles messages in channels/groups
group_policy: "multibot"  # Options: allow, mention, multibot, block
```

### Policy Options

| Policy | Behavior |
| :------ | :-------- |
| `allow` | Process all messages (default) |
| `mention` | Only process when @mentioned |
| `multibot` | Intelligent routing for multi-bot channels |
| `block` | Ignore all channel messages |

### Bot Identity

Each bot must have a unique `bot_user_id` to distinguish it in @mentions:

```yaml
# Bot 1
bot_user_id: "U01BOT1"

# Bot 2
bot_user_id: "U02BOT2"
```

---

## How It Works

### Message Flow

```
User posts message in #channel
         │
         ▼
┌─────────────────────────┐
│ Extract @mentions       │
│ from message text      │
└─────────────────────────┘
         │
         ▼
┌─────────────────────────┐
│ Check GroupPolicy      │
└─────────────────────────┘
         │
    ┌────┴────┐
    ▼         ▼
@mentioned  No @mention
    │         │
    ▼         ▼
Bot processes  Check if broadcast
message       mode is enabled
              (multibot policy)
                    │
              ┌─────┴─────┐
              ▼           ▼
        Respond     Send polite broadcast
        normally    ("I see multiple bots available...")
```

### Decision Logic

```
Message: "Hello @hotplex-bot-01"
├── Bot 01: @mentioned → ✅ Process
├── Bot 02: not mentioned → ❌ Skip
└── Bot 03: not mentioned → ❌ Skip

Message: "Hello team"
├── Bot 01: multibot mode + no mention → ✅ Send broadcast response
├── Bot 02: multibot mode + no mention → ✅ Send broadcast response
└── Bot 03: multibot mode + no mention → ✅ Send broadcast response
```

---

## Broadcast Response

When no bot is @mentioned in a multibot environment, bots can optionally send a polite broadcast response to inform users:

### Default Behavior

Each bot sends an identical broadcast message:

```
🤖 Multiple HotPlex bots are available in this channel.
• @hotplex-bot-01 - Primary assistant
• @hotplex-bot-02 - Code review specialist
Please @mention the bot you'd like to use.
```

### Custom Response

Implement `BroadcastResponder` interface for custom broadcast messages:

```go
type CustomBroadcaster struct{}

func (b *CustomBroadcaster) Respond(userID, message string) string {
    // Custom logic to generate response based on user or message
    return "Hi! Which bot would you like to use? @bot-a or @bot-b?"
}
```

---

## Thread Ownership

MultiBot routing includes **Thread Ownership** to prevent multiple bots from responding to the same thread:

```yaml
# Enable thread ownership
thread_ownership:
  enabled: true
```

### Behavior

1. When a bot starts processing a thread, it "owns" that thread
2. Other bots will not process messages in that thread
3. Thread ownership is released when:
   - The thread receives no new messages for the timeout period
   - Another bot is @mentioned in the thread

---

## Configuration Example

### Two-Bot Setup

**Bot 1 Configuration** (`slack-01.yaml`):
```yaml
mode: socketmode
server:
  port: 8080
slack:
  app_token: xapp-xxx-01
  bot_token: xoxb-xxx-01
  bot_user_id: U01HOTPLEX01
group_policy: "multibot"
thread_ownership:
  enabled: true
  timeout: 30m
```

**Bot 2 Configuration** (`slack-02.yaml`):
```yaml
mode: socketmode
server:
  port: 8081
slack:
  app_token: xapp-xxx-02
  bot_token: xoxb-xxx-02
  bot_user_id: U01HOTPLEX02
group_policy: "multibot"
thread_ownership:
  enabled: true
  timeout: 30m
```

### Environment Setup

```bash
# Bot 1
cp .env .env-01
# Edit .env-01: HOTPLEX_CHATAPPS_CONFIG_DIR=chatapps/configs/slack-01.yaml

# Bot 2
cp .env .env-02
# Edit .env-02: HOTPLEX_CHATAPPS_CONFIG_DIR=chatapps/configs/slack-02.yaml
```

---

## Best Practices

1. **Unique bot_user_id**: Each bot must have a unique `bot_user_id` to prevent session ID collisions
2. **Consistent policies**: Use the same `group_policy` across all bots in a channel
3. **Thread ownership**: Enable thread ownership to prevent duplicate processing
4. **Broadcast responses**: Configure custom broadcast messages to help users understand the multi-bot setup
5. **Clear naming**: Give bots clear, distinguishable names in Slack

---

## Related Documentation

- [Slack Integration](/guide/chatapps-slack) - General Slack setup
- [Docker Matrix](/guide/deployment) - Multi-container deployment
- [Docker Security](/guide/docker-security) - Container isolation
