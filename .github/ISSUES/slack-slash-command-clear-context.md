# Feature Request: Slack Slash Command `/clear` - Clear Session Context

## 📋 Summary

Add Slack slash command `/clear` to allow users to clear the conversation context without restarting the Claude Code process.

**Key Insight**: Claude Code's built-in `/clear` command already handles this - we just need to forward the command via stdin.

---

## 🎯 Motivation

### Problem

```
User in #general channel:
  14:00 - "Help me debug this Python issue..."  [50 messages exchanged]
  14:30 - "Now help me write a Go HTTP server"  ← AI still thinks about Python!
  
User wants: "Start fresh without Python context"
Current solution: Wait 30 minutes for GC, or manually restart service
```

### Solution

```
User types: /clear
Bot response: "✅ Context cleared. Ready for fresh start!"
Next message: "Help me write a Go HTTP server"  ← AI has no Python context
```

---

## 🏗️ Technical Design

### Why This Works

**Claude Code's `/clear` command behavior** (per official docs):

| Benefit | Description |
|---------|-------------|
| ✅ Clears context window | Removes all previous context and starts fresh |
| ✅ Preserves session | Keeps Claude Code running without reinitialization costs |
| ✅ Faster than restart | No startup overhead |
| ✅ Retains `CLAUDE.md` | Project instructions remain active without re-reading |

**No need to**:
- ❌ Delete marker file
- ❌ Terminate the CLI process
- ❌ Force cold start

**Just**:
- ✅ Send `/clear` to Claude Code via stdin
- ✅ Session continues with empty context

---

### Implementation

#### 1. Slash Command Handler

**File: `chatapps/slack/adapter.go`**

```go
// handleSlashCommand processes incoming slash commands
func (a *Adapter) handleSlashCommand(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusMethodNotAllowed)
        return
    }

    if err := r.ParseForm(); err != nil {
        a.Logger().Error("Parse slash command form failed", "error", err)
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    cmd := SlashCommand{
        Command:     r.FormValue("command"),
        Text:        r.FormValue("text"),
        UserID:      r.FormValue("user_id"),
        ChannelID:   r.FormValue("channel_id"),
        ResponseURL: r.FormValue("response_url"),
    }

    a.Logger().Debug("Slash command received", 
        "command", cmd.Command, 
        "text", cmd.Text, 
        "user", cmd.UserID)

    // Acknowledge immediately (Slack requires response within 3 seconds)
    w.WriteHeader(http.StatusOK)

    // Process command in background
    go a.processSlashCommand(cmd)
}

// processSlashCommand handles the slash command logic
func (a *Adapter) processSlashCommand(cmd SlashCommand) {
    ctx := context.Background()
    
    switch cmd.Command {
    case "/clear":
        a.handleClearCommand(ctx, cmd)
    case "/reset":
        a.handleResetCommand(ctx, cmd)  // Optional future extension
    default:
        a.handleUnknownCommand(ctx, cmd)
    }
}
```

#### 2. Clear Command Implementation

**File: `chatapps/slack/adapter.go`**

```go
// handleClearCommand processes /clear command
func (a *Adapter) handleClearCommand(ctx context.Context, cmd SlashCommand) {
    sessionID := cmd.ChannelID + ":" + cmd.UserID
    
    // Get handler from adapter
    handler := a.Handler()
    if handler == nil {
        a.Logger().Error("Handler is nil")
        a.sendEphemeralMessage(cmd.ResponseURL, "❌ Internal error: Handler not initialized")
        return
    }
    
    // Get the engine session
    emh, ok := handler.(*chatapps.EngineMessageHandler)
    if !ok {
        a.Logger().Error("Handler is not EngineMessageHandler")
        a.sendEphemeralMessage(cmd.ResponseURL, "❌ Internal error: Unexpected handler type")
        return
    }
    
    // Get session from engine
    sess, exists := emh.GetEngine().GetSession(sessionID)
    if !exists {
        a.sendEphemeralMessage(cmd.ResponseURL, "ℹ️ No active session found")
        return
    }
    
    // Send /clear command to Claude Code via stdin
    // This clears the conversation context
    clearCmd := map[string]any{
        "type": "user",
        "message": map[string]any{
            "role":    "user",
            "content": []map[string]any{
                {"type": "text", "text": "/clear"},
            },
        },
    }
    
    if err := sess.WriteInput(clearCmd); err != nil {
        a.Logger().Error("Failed to send /clear command", 
            "session_id", sessionID, "error", err)
        a.sendEphemeralMessage(cmd.ResponseURL, 
            fmt.Sprintf("❌ Failed to clear context: %v", err))
        return
    }
    
    a.Logger().Info("Sent /clear command to Claude Code", 
        "session_id", sessionID)
    
    // Send success response
    a.sendEphemeralMessage(cmd.ResponseURL, 
        "✅ Context cleared. Ready for fresh start!")
}

// sendEphemeralMessage sends a message visible only to the user who issued the command
func (a *Adapter) sendEphemeralMessage(responseURL, text string) {
    payload := map[string]any{
        "response_type": "ephemeral",
        "text":          text,
    }
    
    body, err := json.Marshal(payload)
    if err != nil {
        a.Logger().Error("Failed to marshal ephemeral message", "error", err)
        return
    }
    
    resp, err := http.Post(responseURL, "application/json", bytes.NewReader(body))
    if err != nil {
        a.Logger().Error("Failed to send ephemeral message", "error", err)
        return
    }
    defer resp.Body.Close()
}
```

#### 3. Engine GetSession Method (if not exists)

**File: `engine/runner.go`**

```go
// GetSession retrieves an active session by sessionID.
// Returns the session and true if found, or nil and false if not found.
func (r *Engine) GetSession(sessionID string) (*intengine.Session, bool) {
    if r.manager == nil {
        return nil, false
    }
    return r.manager.GetSession(sessionID)
}
```

---

## 📝 Implementation Tasks

### Phase 1: Core Implementation

- [ ] **Task 1.1**: Add `GetSession(sessionID string) (*Session, bool)` to `engine/runner.go` (if not exists)
- [ ] **Task 1.2**: Update `handleSlashCommand()` in `chatapps/slack/adapter.go`
- [ ] **Task 1.3**: Implement `handleClearCommand()` in `chatapps/slack/adapter.go`
- [ ] **Task 1.4**: Implement `sendEphemeralMessage()` helper

### Phase 2: Optional Extensions

- [ ] **Task 2.1**: Implement `/reset [instructions]` - Clear + update task instructions
- [ ] **Task 2.2**: Implement `/status` - Show current session info
- [ ] **Task 2.3**: Add rate limiting (5 clears/minute/user)

### Phase 3: Testing

- [ ] **Task 3.1**: Unit test: `handleClearCommand()` sends correct stdin
- [ ] **Task 3.2**: Integration test: After `/clear`, next message has no context
- [ ] **Task 3.3**: E2E test: Slash command flow via Slack mock

### Phase 4: Documentation

- [ ] **Task 4.1**: Update `chatapps/configs/slack.yaml` with slash command config
- [ ] **Task 4.2**: Update `docs/chatapps/chatapps-slack.md` with `/clear` docs

---

## 🔒 Security Considerations

### 1. Authorization

**Session isolation**: `sessionID = channel_id + ":" + user_id` ensures users can only clear their own sessions.

### 2. Rate Limiting (Optional)

```go
type rateLimitEntry struct {
    lastClear time.Time
    count     int
}

func (a *Adapter) checkClearRateLimit(userID string) bool {
    // Max 5 clears per minute per user
}
```

### 3. Ephemeral Responses

All command responses use `"response_type": "ephemeral"` to avoid channel spam.

---

## 📊 Metrics (Optional)

```go
// telemetry/metrics.go
type Metrics struct {
    SessionClearsTotal  prometheus.Counter  // Count of /clear commands
}

func (m *Metrics) IncSessionClears() {
    m.SessionClearsTotal.Inc()
}
```

---

## 🧪 Test Plan

### Unit Test

```go
// slack/adapter_test.go
func TestHandleClearCommand(t *testing.T) {
    // 1. Create mock session
    // 2. Call handleClearCommand()
    // 3. Verify WriteInput() was called with /clear
    // 4. Verify ephemeral response was sent
}
```

### Integration Test

```go
// integration/slack_clear_context_test.go
func TestClearContextEndToEnd(t *testing.T) {
    // 1. Send messages to build context
    //    User: "Remember this secret: ABC123"
    //    AI:   "I've noted your secret."
    // 2. Send /clear command
    // 3. Send new message
    //    User: "What secret did I tell you?"
    // 4. Verify AI has NO memory of "ABC123"
}
```

---

## 📚 Documentation

### Update `chatapps-slack.md`

Add new section:

```markdown
## 11. Slash Commands

### /clear

Clear the current conversation context and start fresh.

**Usage**: `/clear`

**What it does**:
- Sends `/clear` command to Claude Code
- Clears all conversation context
- Session continues (no restart needed)
- `CLAUDE.md` project instructions are preserved

**Response**: Ephemeral (only visible to you)
```

---

## 🎯 Success Criteria

- [ ] `/clear` sends `/clear` to Claude Code via stdin
- [ ] After `/clear`, next message has no previous context
- [ ] Session is NOT terminated (process continues)
- [ ] Marker file is NOT deleted (session persists)
- [ ] Ephemeral responses (only visible to issuing user)
- [ ] Unit tests pass (`go test -race ./chatapps/slack/...`)
- [ ] Documentation updated

---

## 🔗 Related

- [Claude Code Interactive Mode Docs](https://code.claude.com/docs/en/interactive-mode)
- [Managing Claude Code's Context](https://www.claudelog.com/faqs/restarting-claude-code/)

---

**Priority**: High  
**Estimated Effort**: 1 day  
**Risk Level**: Low (simple implementation, no session state changes)  
**Labels**: `enhancement`, `slack`, `chatapps`, `session-management`
