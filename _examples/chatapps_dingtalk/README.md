# ChatApps DingTalk 适配器示例

本示例展示如何使用 HotPlex ChatApps 接入层将钉钉机器人接入为用户交互渠道。

## 架构

```
用户 ──► 钉钉 ──► DingTalkAdapter ──► HotPlex Engine
                              │
                              ▼
                        响应消息 ◄──
```

## 配置步骤

### 1. 钉钉应用配置

1. 登录[钉钉开放平台](https://open.dingtalk.com)
2. 创建企业内部应用
3. 添加机器人能力
4. 获取 `AppID` 和 `AppSecret`
5. 配置回调地址

### 2. 环境变量

```bash
# 服务地址
HOTPLEX_CHATAPPS_ADDR=:8080

# 钉钉应用凭证
HOTPLEX_DINGTALK_APP_ID=your_app_id
HOTPLEX_DINGTALK_APP_SECRET=your_app_secret
HOTPLEX_DINGTALK_CALLBACK_TOKEN=your_callback_token
```

### 3. 运行示例

```bash
# 运行服务
go run _examples/chatapps_dingtalk/main.go

# 或者指定地址
HOTPLEX_CHATAPPS_ADDR=:9000 go run _examples/chatapps_dingtalk/main.go
```

## 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/webhook` | GET/POST | 钉钉回调接口 |
| `/health` | GET | 健康检查 |

## 消息流程

1. 用户在钉钉群发送消息
2. 钉钉服务器发送回调到 `/webhook`
3. DingTalkAdapter 解析消息并创建会话
4. 消息处理器处理消息（可接入 HotPlex Engine）
5. 响应发送回钉钉

## 进阶

- 接入 HotPlex Engine 实现 AI 对话
- 支持 Markdown 消息渲染
- 多会话管理
- 消息持久化
