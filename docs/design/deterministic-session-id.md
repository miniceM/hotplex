# 确定性 Session ID 设计方案

**版本**: 1.0  
**日期**: 2026-02-25  
**状态**: 设计中  
**作者**: AI Agent  

---

## 1. 需求背景

### 1.1 当前问题

当前 Session ID 基于时间戳生成，导致无法保证同一用户总是映射到同一个 ProviderSessionID。

### 1.2 用户需求

```
场景 1: 用户张三 + BotApp 单聊 → 固定的 ProviderSessionID = "aaa111-..."
场景 2: 用户张三 + BotApp + Channel1 → 固定的 ProviderSessionID = "bbb222-..." (与单聊不同)
场景 3: 用户李四 + BotApp + Channel1 → 固定的 ProviderSessionID = "ccc333-..." (与张三不同)
```

### 1.3 设计目标

| 目标 | 描述 | 优先级 |
|------|------|--------|
| **确定性** | 相同输入 → 相同 SessionID | P0 |
| **隔离性** | 不同用户/Bot/Channel → 不同 SessionID | P0 |
| **可恢复** | 进程 GC 后能恢复会话 | P0 |
| **可扩展** | 支持平台自定义生成规则 | P1 |

---

## 2. 架构设计

### 2.1 双层 Session 映射

```
第一层：base.Adapter (平台层)
  key = "platform:user_id:bot_user_id:channel_id"
    ↓
  SessionID = UUID5("hotplex:session:" + key)
    ↓
  存储：a.sessions[key] = &base.Session{SessionID, ...}
  
第二层：engine.SessionPool (引擎层)
  uniqueStr = "namespace:session:" + SessionID
    ↓
  ProviderSessionID = UUID5(uniqueStr)
    ↓
  存储：sm.sessions[SessionID] = &engine.Session{...}
  marker: ~/.claude/projects/{ProviderSessionID}/marker
```

### 2.2 Session ID 生成流程

```
用户发送消息
    ↓
提取四元组：platform, user_id, bot_user_id, channel_id
    ↓
构建 key: "slack:U0AHCF4DPK2:bot123:C12345"
    ↓
计算哈希：UUID5("hotplex:session:slack:U0AHCF4DPK2:bot123:C12345")
    ↓
SessionID: "aaa111-222-333-444-555" (确定性，永远固定)
    ↓
ProviderSessionID: UUID5("hotplex:session:" + SessionID)
    ↓
Claude Code 启动：claude --session-id {ProviderSessionID}
```

---

## 3. 详细设计

### 3.1 SessionIDGenerator 接口

```go
type SessionIDGenerator interface {
    Generate(platform, userID, botUserID, channelID string) string
}

// UUID5Generator - 默认实现
type UUID5Generator struct {
    namespace string
}

func (g *UUID5Generator) Generate(platform, userID, botUserID, channelID string) string {
    key := fmt.Sprintf("%s:%s:%s:%s", platform, userID, botUserID, channelID)
    input := g.namespace + ":session:" + key
    return uuid.NewSHA1(uuid.NameSpaceURL, []byte(input)).String()
}
```

### 3.2 Adapter 结构体修改

```go
type Adapter struct {
    // ... 现有字段 ...
    
    // ✅ 新增：
    sessionIDGenerator SessionIDGenerator
}

func NewAdapter(...) *Adapter {
    a := &Adapter{
        // ... 现有初始化 ...
        sessionIDGenerator: NewUUID5Generator("hotplex"),
    }
    return a
}
```

### 3.3 GetOrCreateSession 方法修改

```go
func (a *Adapter) GetOrCreateSession(userID, botUserID, channelID string) string {
    a.mu.Lock()
    defer a.mu.Unlock()

    key := fmt.Sprintf("%s:%s:%s:%s", a.platformName, userID, botUserID, channelID)

    if session, ok := a.sessions[key]; ok {
        session.LastActive = time.Now()
        return session.SessionID
    }

    sessionID := a.sessionIDGenerator.Generate(a.platformName, userID, botUserID, channelID)

    session := &Session{
        SessionID:  sessionID,
        UserID:     userID,
        Platform:   a.platformName,
        LastActive: time.Now(),
    }
    a.sessions[key] = session

    return sessionID
}
```

---

## 4. 实现计划

### Phase 1: 核心基础设施 (2 小时)
- [ ] 1.1: 完成 `session_id_generator.go`
- [ ] 1.2: 修改 `Adapter` 结构体
- [ ] 1.3: 修改 `NewAdapter` 构造函数
- [ ] 1.4: 修改 `GetOrCreateSession` 签名

### Phase 2: 平台适配层更新 (3 小时)
- [ ] 2.1: 更新 Slack adapter 调用
- [ ] 2.2: 实现 Slash Command session 查找
- [ ] 2.3: 更新 Telegram adapter
- [ ] 2.4: 更新其他平台 (可选)

### Phase 3: 测试与验证 (2 小时)
- [ ] 3.1: 单元测试
- [ ] 3.2: 集成测试
- [ ] 3.3: 验证进程恢复机制
- [ ] 3.4: 回归测试

### Phase 4: 文档与清理 (1 小时)
- [ ] 4.1: 更新架构文档
- [ ] 4.2: 添加迁移指南

**总预计时间**: 8 小时

---

## 5. 验收标准

### 功能验收
- [ ] 相同用户+Bot+Channel 总是获得相同 SessionID
- [ ] 不同用户 OR 不同 Bot OR 不同 Channel 获得不同 SessionID
- [ ] 进程 GC 后能正确恢复会话
- [ ] Slash Command 能正确查找会话

### 质量验收
- [ ] 单元测试覆盖率 > 80%
- [ ] 集成测试全部通过
- [ ] 无回归问题

---

## 6. Session ID 示例

```
输入："slack" + "U0AHCF4DPK2" + "bot123" + "C12345"
输出："aaa111-222-333-444-555-666777888999"

输入："slack" + "U0AHCF4DPK3" + "bot123" + "C12345"
输出："bbb222-333-444-555-666-777888999aaa"
```
