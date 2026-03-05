# 🔀 环境变量迁移手册 (Environment Variables Migration Guide)

随着 HotPlex 迈向更加系统化与模块化的 v0.19.0 版本，为了彻底解决在多服务部署环境中可能出现的变量命名冲突（例如与常见框架或系统的 `PORT`、`LOG_LEVEL` 等冲突），我们对所有核心环境变量实施了统一的命名空间隔离。

**自现在起，几乎所有 HotPlex 依赖的环境变量都必须以 `HOTPLEX_` 作为前缀。**

本手册旨在帮助您将现有的 `.env` 文件或服务部署配置（如 Docker Compose、Systemd、Kubernetes ConfigMap 等）平滑迁移至新规范。

---

## 🎯 一分钟快速行动指南

如果您只想快速完成升级，**请直接在您现有的 `.env` 项前加上 `HOTPLEX_`（原本已有 `HOTPLEX_` 前缀的项保持不变）。**

**示例：**
- ❌ 之前: `PORT=8080`
- ✅ 现在: `HOTPLEX_PORT=8080`

- ❌ 之前: `SLACK_BOT_TOKEN=xoxb-xxx`
- ✅ 现在: `HOTPLEX_SLACK_BOT_TOKEN=xoxb-xxx`

*如果您的环境尚未包含这批变量，可以直接将最新的 `.env.example` 复制为 `.env` 并重新填入凭据。*

---

## 📋 详细对照表：变化项一览

以下列出了本次迁移中**必定受到了影响**（需要被重命名）的变量集合。通过检查此表，可以确保您的所有生产配置都不会在启动时因找不到参数而使用默认值（甚至报错）。

### 1. 核心引擎与架构参数
| 旧变量名 (已废弃 ⚠️) | 新变量名 (需更新 ✅)             | 作用说明                                |
| :------------------ | :------------------------------ | :-------------------------------------- |
| `PORT`              | **`HOTPLEX_PORT`**              | HotPlex 代理服务器的监听端口            |
| `EXECUTION_TIMEOUT` | **`HOTPLEX_EXECUTION_TIMEOUT`** | AI 进程单次执行的最大超时时间           |
| `IDLE_TIMEOUT`      | **`HOTPLEX_IDLE_TIMEOUT`**      | 后端 CLI 会话闲置后自动退出的超时时间   |
| `LOG_LEVEL`         | **`HOTPLEX_LOG_LEVEL`**         | 日志打印级别 (DEBUG, INFO, WARN, ERROR) |
| `LOG_FORMAT`        | **`HOTPLEX_LOG_FORMAT`**        | 日志输出格式 (json, text 等)            |
| `STRESS_SESSIONS`   | **`HOTPLEX_STRESS_SESSIONS`**   | 仅内部测试使用的会话压测并发数          |

### 2. 连通层网关 (ChatApps)
| 旧变量名 (已废弃 ⚠️)   | 新变量名 (需更新 ✅)               | 作用说明                           |
| :-------------------- | :-------------------------------- | :--------------------------------- |
| `CHATAPPS_ENABLED`    | **`HOTPLEX_CHATAPPS_ENABLED`**    | 机器人层级的全局总开关             |
| `CHATAPPS_CONFIG_DIR` | **`HOTPLEX_CHATAPPS_CONFIG_DIR`** | 存放各平台配置 `*.yaml` 的文件目录 |

### 3. 具体机器人平台集成
这些变更影响到相应的平台认证接口，未及时更新将导致收不到消息或接口鉴权失败。

#### Slack
| 旧变量名 (已废弃 ⚠️)    | 新变量名 (需更新 ✅)                |
| :--------------------- | :--------------------------------- |
| `SLACK_MODE`           | **`HOTPLEX_SLACK_MODE`**           |
| `SLACK_BOT_TOKEN`      | **`HOTPLEX_SLACK_BOT_TOKEN`**      |
| `SLACK_APP_TOKEN`      | **`HOTPLEX_SLACK_APP_TOKEN`**      |
| `SLACK_SIGNING_SECRET` | **`HOTPLEX_SLACK_SIGNING_SECRET`** |
| `SLACK_SERVER_ADDR`    | **`HOTPLEX_SLACK_SERVER_ADDR`**    |

#### DingTalk (钉钉)
| 旧变量名 (已废弃 ⚠️)       | 新变量名 (需更新 ✅)                   |
| :------------------------ | :------------------------------------ |
| `DINGTALK_APP_ID`         | **`HOTPLEX_DINGTALK_APP_ID`**         |
| `DINGTALK_APP_SECRET`     | **`HOTPLEX_DINGTALK_APP_SECRET`**     |
| `DINGTALK_CALLBACK_TOKEN` | **`HOTPLEX_DINGTALK_CALLBACK_TOKEN`** |
| `DINGTALK_CALLBACK_KEY`   | **`HOTPLEX_DINGTALK_CALLBACK_KEY`**   |

#### Feishu (飞书)
| 旧变量名 (已废弃 ⚠️)         | 新变量名 (需更新 ✅)                     |
| :-------------------------- | :-------------------------------------- |
| `FEISHU_APP_ID`             | **`HOTPLEX_FEISHU_APP_ID`**             |
| `FEISHU_APP_SECRET`         | **`HOTPLEX_FEISHU_APP_SECRET`**         |
| `FEISHU_VERIFICATION_TOKEN` | **`HOTPLEX_FEISHU_VERIFICATION_TOKEN`** |
| `FEISHU_ENCRYPT_KEY`        | **`HOTPLEX_FEISHU_ENCRYPT_KEY`**        |
| `FEISHU_SERVER_ADDR`        | **`HOTPLEX_FEISHU_SERVER_ADDR`**        |
| `FEISHU_TEST_CHAT_ID`       | **`HOTPLEX_FEISHU_TEST_CHAT_ID`**       |

#### 其他平台 (Telegram, Discord, WhatsApp)
*涉及 `TELEGRAM_BOT_TOKEN`, `DISCORD_PUBLIC_KEY`, `WHATSAPP_ACCESS_TOKEN` 等，均需要增加 `HOTPLEX_` 前缀。此处不再赘述，规律如上所述。*

---

## 🚫 不受影响的变量
**原本已经带有 `HOTPLEX_` 前缀的配置项无需进行任何修改**，包括但不限于：
- `HOTPLEX_API_KEY`
- `HOTPLEX_PROVIDER_TYPE`
- `HOTPLEX_BRAIN_PROVIDER`
- `HOTPLEX_ALLOWED_ORIGINS`
- `HOTPLEX_DINGTALK_ENABLED` 等。

---

## 🔧 常见环境迁移指引 (FAQ)

### Q: 我是 Docker Composer 用户应该怎么迁移？
打开 `docker-compose.yml`，在 `environment:` 列表或引用的 `.env` 中按上述表格批量重命名。然后运行 `docker compose down && docker compose up -d` 即可。

### Q: 我在 Makefile / alias 里写死了临时变量怎么办？
如果您的启动命令曾经是 `PORT=9090 make run`，请将其更新为：
`HOTPLEX_PORT=9090 make run`

### Q: 遗漏了会怎样？
由于 Go 代码中使用 `os.Getenv` 获取配置时，如果没有取到就会回退到（默认值 / 空值）。常见症状包括：
*   **启动失败/静默重置**：机器人无法连上平台（因为获取不到 Token）。
*   **端口未生效**：比如曾经监听 9090 但启动后发现依然跑在 8080 (默认)。
当您发现这些症状时，请第一时间检查前缀。

---
最后，对于从本地仓库进行开发更新的使用者，**我们建议直接检查最新的 `.env.example`**，并用作构建 `.env` 的唯一标尺。
