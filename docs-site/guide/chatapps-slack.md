# Slack Mastery Guide

## Architecting the Primary Receptor

This guide is authored for the discerning engineer who demands the highest level of integration. The **HotPlex Slack Adapter** is not just a bot; it is a high-performance cognitive bridge that leverages Slack's **Block Kit** to manifest AI agency with unprecedented clarity.

---

### ⚡ Rapid Manifest Deployment

The most refined path to integration is via the **App Manifest**. This allows you to orchestrate dozens of permissions and features in a single, atomic declaration.

1.  Navigate to the [Slack App Dashboard](https://api.slack.com/apps).
2.  **Create New App** -> **From an app manifest**.
3.  Choose your workspace and paste the following YAML:

```yaml
display_information:
  name: HotPlex
  description: High-Performance AI Agent Receptor
  background_color: "#000000"
features:
  bot_user:
    display_name: HotPlex
    always_online: true
  slash_commands:
    - command: /reset
      description: Re-initialize the agent context (Cold Start)
    - command: /dc
      description: Terminate background long-running processes
oauth_config:
  scopes:
    bot:
      - app_mentions:read
      - chat:write
      - chat:write.public
      - reactions:write
      - im:history
      - channels:history
      - groups:history
      - files:write
      - commands
settings:
  event_subscriptions:
    bot_events:
      - app_mention
      - message.channels
      - message.groups
      - message.im
  interactivity:
    is_enabled: true
  socket_mode_enabled: true
```

---

### 🗝️ The Sovereignty of Secrets

To establish a secure link to the Bridge, you must secure the following cryptographic keys from your Slack Dashboard:

| Key                | Recommended Path      | Purpose                                                         |
| :----------------- | :-------------------- | :-------------------------------------------------------------- |
| **Bot Token**      | `OAuth & Permissions` | **The Primary Key**: For message orchestration and UI updates.  |
| **App Token**      | `Basic Information`   | **The Socket Key**: Enables high-performance Socket Mode.       |
| **Signing Secret** | `Basic Information`   | **The Verifier**: Ensures the integrity of all incoming pulses. |

---

### 📡 Communication Modalities

HotPlex supports two modes of existence. Define your preference in the `.env` configuration:

#### Modality A: Socket Mode (The Developer Choice)
- **Nature**: An outbound WebSocket connection. Ideal for restricted networks or internal developer environments.
- **Config**: `SLACK_MODE=socket`, `SLACK_APP_TOKEN=xapp-...`

#### Modality B: HTTP Mode (The Production Choice)
- **Nature**: A high-availability webhook entry point. Ideal for scalable, production-grade load balancers.
- **Config**: `SLACK_MODE=http`, `SLACK_SIGNING_SECRET=...`
- **End-point**: Register `https://your-domain.com/webhook/slack/events` in the Slack Console.

---

### ✨ The Visual Language of Agency

#### 1. The Pulse of Progress (Reactions)
The agent communicates its internal state via a subtle language of emojis:
- 📥 (`:inbox:`): The request has been queued in the session pool.
- 🧠 (`:brain:`): The engine is performing deep cognitive reasoning or codebase scanning.
- ✅ (`:white_check_mark:`): The interaction loop has reached a successful resolution.
- ⚠️ (`:warning:`): An unexpected friction was encountered. Check the Error Zone.

#### 2. Structural Interaction (6 Zones Mapping)
Every agent message sequence is divided into atomic **Zones** to ensure clarity and avoid history bloat:
- **Zone 0 (Status):** Ephemeral indicator (`Initializing...`) destroyed upon further action.
- **Zone 1 (Thinking):** Real-time context limited to a scrolling window of the latest 64-characters with a 1-second cadence throttle to maintain UI sanity.
- **Zone 2 (Action):** High-visibility `tool_use` / `tool_result` cards strictly capped to a sliding window of **2 messages**. Older actions are automatically purged.
- **Zone 3 (Display):** The final intellectual artifact, auto-paginated using divider blocks every ~3500 bytes for unlimited fluid streaming.
- **Zone 4 (Interaction):** Blocking wait state containing interactive prompt elements (HITL approvals, questions).
- **Zone 5 (Summary):** Session-end statistics indicating token and duration metrics.

---

### 🛠️ Deterministic Configuration

Fine-tune the behavior of your receptor in `chatapps/configs/slack.yaml`:

- **`bot_user_id`**: **Mandatory.** Ensures the receptor recognizes its own identity.
- **`dm_policy`**: Choose between `allow` or `pairing` (restricts DMs to known users).
- **`group_policy`**: Control whether the agent listens to all chatter or only explicit @mentions.

---

> "Integrate not just for function, but for the experience of collaboration." — The HotPlex Team
