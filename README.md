<div align="center">

# Team-API

**多租户 AI API 网关 SaaS 平台**

统一接入 OpenAI、Claude、Gemini 等 25+ 大模型供应商，提供计费、限流、监控和多租户管理能力。

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![GoFrame](https://img.shields.io/badge/GoFrame-v2.10-blue?style=flat-square)](https://goframe.org/)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D?style=flat-square&logo=vue.js)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=flat-square&logo=redis)](https://redis.io/)
[![License](https://img.shields.io/badge/License-AGPL_v3-blue?style=flat-square)](LICENSE)

</div>

---

## 功能特性

- **多租户架构** — 行级租户隔离，管理后台与租户控制台双独立用户体系
- **25+ 大模型供应商** — OpenAI、Claude、Gemini、DeepSeek、通义千问、智谱、Ollama 等
- **OpenAI 兼容 API** — 无缝替换 OpenAI API，支持对话补全、向量嵌入、图像生成、语音、实时通信等
- **智能渠道调度** — 优先级/权重路由、自动故障转移、健康监控、渠道亲和性
- **五层额度模型** — 租户钱包 → 套餐额度 → 成员额度 → 项目预算 → Key 额度
- **实时计费引擎** — 预扣 → 转发 → 结算 → 退款，Redis 原子操作保证并发安全
- **双控制台** — 管理后台（Naive UI）用于平台运营 + 租户控制台（TailwindCSS）面向终端用户
- **RBAC 权限控制** — 角色 + 权限点 + 数据范围，前端按钮级权限控制
- **异步任务引擎** — 支持 Midjourney、Suno、可灵、Sora 等异步生成任务
- **全链路可观测** — 请求日志、操作审计、监控告警，Request ID 贯穿全链路

## 系统架构

```
                                ┌─────────────────────────────────┐
                                │          负载均衡器              │
                                └──────────┬──────────────────────┘
                                           │
                    ┌──────────────────────┼──────────────────────┐
                    │                      │                      │
            ┌───────▼──────┐    ┌──────────▼──────┐    ┌─────────▼────────┐
            │  /api/admin  │    │  /api/tenant    │    │   /v1/*          │
            │  管理后台 API │    │  租户控制台 API  │    │   AI 代理转发    │
            └───────┬──────┘    └──────────┬──────┘    └─────────┬────────┘
                    │                      │                      │
            ┌───────▼──────────────────────▼──────┐    ┌─────────▼────────┐
            │        GoFrame 业务服务层            │    │   Relay 代理层   │
            │   (Controller → Service → Logic)    │    │  (25+ 适配器)    │
            └───────┬─────────────────────────────┘    └─────────┬────────┘
                    │                                             │
         ┌──────────┼──────────┐                    ┌────────────┼────────┐
         │          │          │                    │            │        │
    ┌────▼───┐ ┌───▼────┐ ┌───▼───┐          ┌────▼───┐  ┌─────▼──┐  ┌──▼──┐
    │PostgreSQL│ │ Redis  │ │  S3   │          │ OpenAI │  │ Claude │  │ ... │
    └────────┘ └────────┘ └───────┘          └────────┘  └────────┘  └─────┘
```

## 技术栈

| 层级 | 技术选型 |
|------|---------|
| 后端框架 | Go + [GoFrame v2](https://goframe.org/) |
| 数据库 | PostgreSQL 15 |
| 缓存 | Redis 7 + 内存缓存（双层） |
| 数据库迁移 | [Goose](https://github.com/pressly/goose) |
| 管理后台前端 | Vue 3 + Vite + [Naive UI](https://www.naiveui.com/) + TailwindCSS |
| 租户控制台前端 | Vue 3 + Vite + TailwindCSS |
| 对象存储 | S3 / 阿里云 OSS / 腾讯云 COS / MinIO |
| 前端包管理 | pnpm |

## 快速开始

### 环境要求

- Go 1.25+
- PostgreSQL 15+
- Redis 7+
- Node.js 18+ & pnpm（前端开发）
- [GoFrame CLI](https://goframe.org/pages/viewpage.action?pageId=1114260)（`gf` 命令）
- [Goose](https://github.com/pressly/goose)（数据库迁移）

### 1. 克隆仓库

```bash
git clone https://github.com/your-org/team-api.git
cd team-api
```

### 2. 启动基础设施

使用 Docker Compose 启动 PostgreSQL、Redis 和 MinIO：

```bash
docker compose -f manifest/docker/docker-compose.yaml up -d
```

### 3. 修改配置

复制并编辑配置文件：

```bash
cp manifest/config/config.yaml.example manifest/config/config.yaml
```

核心配置项：

```yaml
server:
  address: ":18888"

database:
  default:
    type: "pgsql"
    link: "pgsql:user:password@tcp(127.0.0.1:5432)/team-api?sslmode=disable"

redis:
  default:
    address: "127.0.0.1:6379"
    db: 0

jwt:
  secret: "your-secret-key-change-in-production"
```

### 4. 执行数据库迁移

```bash
make migrate-up
```

### 5. 启动后端服务

```bash
# 开发模式（热编译）
make run

# 或直接运行
gf run main.go
```

API 服务将在 `http://localhost:18888` 启动。

### 6. 启动前端（可选）

```bash
# 管理后台
cd web/admin
pnpm install
pnpm dev

# 租户控制台（另开终端）
cd web/tenant
pnpm install
pnpm dev
```

### 7. 生产构建

```bash
# 构建后端二进制文件
make build

# 构建前端
cd web/admin && pnpm build
cd web/tenant && pnpm build
```

## API 接口

### 管理类接口

| 路径 | 说明 | 认证方式 |
|------|------|---------|
| `/api/admin/*` | 管理后台 API | JWT（admin_users 表） |
| `/api/tenant/*` | 租户控制台 API | JWT（tenant_users 表） |
| `/api/payment/*` | 支付回调 | 签名验证 |
| `/api/open/*` | 开放平台 API | HMAC-SHA256 |
| `/api/status` | 公开状态页 | 无需认证 |
| `/api/captcha/*` | 验证码服务 | 无需认证 |
| `/api/docs/*` | OpenAPI 文档 | 无需认证 |
| `/api/setup/*` | 系统初始化向导 | 仅首次部署 |

### AI 代理接口（OpenAI 兼容）

| 接口 | 说明 |
|------|------|
| `POST /v1/chat/completions` | 对话补全 |
| `POST /v1/embeddings` | 文本向量嵌入 |
| `POST /v1/images/generations` | 图像生成 |
| `POST /v1/audio/transcriptions` | 语音转文字 |
| `POST /v1/audio/translations` | 语音翻译 |
| `POST /v1/audio/speech` | 文字转语音 |
| `GET  /v1/models` | 获取可用模型列表 |
| `POST /v1/moderations` | 内容审核 |
| `POST /v1/rerank` | 重排序 |
| `WS   /v1/realtime` | 实时通信（WebSocket） |

**使用示例：**

```bash
curl http://localhost:18888/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "你好！"}]
  }'
```

## 配置说明

### 环境变量

所有配置项均可通过 `GF_` 前缀的环境变量覆盖（GoFrame 框架约定）：

```bash
# 数据库
GF_DATABASE_DEFAULT_LINK="pgsql:user:pass@tcp(host:5432)/dbname?sslmode=disable"

# Redis
GF_REDIS_DEFAULT_ADDRESS="127.0.0.1:6379"

# JWT
GF_JWT_SECRET="your-production-secret"

# 服务端口
GF_SERVER_ADDRESS=":18888"
```

### 数据库迁移

使用 [Goose](https://github.com/pressly/goose) 管理数据库迁移，按六位序号递增编号：

```bash
# 执行所有待迁移脚本
make migrate-up

# 回滚上一次迁移
make migrate-down

# 查看迁移状态
make migrate-status
```

## Docker 部署

### 构建镜像

```bash
gf build
docker build -t team-api:latest -f manifest/docker/Dockerfile .
```

### 使用 Docker Compose 运行

```bash
docker compose -f manifest/docker/docker-compose.yaml up -d
```

## 开发指南

### 代码生成

GoFrame CLI 从定义文件自动生成样板代码：

```bash
# 从数据库表结构生成 DAO/DO/Entity
gf gen dao

# 从 Logic 层生成 Service 接口
gf gen service

# 从 API 定义生成 Controller
gf gen ctrl
```

**重要：** 新增 API 时必须按以下顺序执行：

1. 在 `api/` 中定义请求/响应结构体
2. 在 `internal/logic/` 中实现业务逻辑
3. 执行 `gf gen service`
4. 执行 `gf gen ctrl`

### Makefile 命令

```bash
make run             # 启动开发服务器（热编译）
make build           # 构建生产二进制文件
make tidy            # 整理 Go 模块依赖
make migrate-up      # 执行数据库迁移
make migrate-down    # 回滚上一次迁移
make migrate-status  # 查看迁移状态
```

## 计费模型

Team-API 实现了五层额度体系：

```
租户钱包（人民币）
  └─ 套餐额度（资源池）
      └─ 成员额度（控制线）
          └─ 项目预算（控制线）
              └─ Key 额度（控制线）
```

- 大模型定价使用**美元**（与上游供应商报价一致）
- 用户钱包与支付使用**人民币**
- 管理后台可配置人民币/美元兑换比例
- 预扣通过 Redis 原子操作保证并发安全
- 价格查询优先级：租户独立价 > 套餐价 > 模型基础价 > 默认价

## 功能模块开发进度

> ✅ 已完成 &nbsp; 🚧 部分完成 &nbsp; ⬜ 未开始

| 模块             | 状态 | 说明                                                                   |
|----------------|:----:|----------------------------------------------------------------------|
| 项目骨架与基础设施      | ✅ | GoFrame v2 项目结构、数据库迁移、双层缓存、异步任务框架、文件存储                               |
| 双控制台认证系统       | ✅ | 管理后台/租户控制台独立认证、JWT 双 Token、会话控制、登录锁定、邀请注册                            |
| RBAC 权限控制      | ✅ | 角色 + 权限点 + 数据范围三层模型                                                  |
| 操作审计日志         | ✅ | 全链路 Request ID、操作日志中间件、敏感访问记录、登录历史                                   |
| OpenAI 适配器     | ✅ | Chat/Embeddings/Images/Completions/Audio/Rerank/Moderations/Realtime |
| Claude 适配器     | ✅ | Messages API、OpenAI ↔ Claude 协议转换                                    |
| Gemini 适配器     | ✅ | GenerateContent API、OpenAI ↔ Gemini 协议转换                             |
| 24 大模型供应商      | ✅ | DeepSeek、通义千问、智谱、Moonshot、Mistral、xAI、Bedrock、Vertex AI、Ollama 等     |
| 渠道调度引擎         | ✅ | 优先级/权重路由、自动故障转移、渠道亲和性、健康评分                                           |
| 计费引擎           | ✅ | 预扣→转发→结算→退款、梯度定价、双层乘数、五层额度校验                                         |
| 限流与并发控制        | ✅ | 四级 QPS 限流 + 三级并发限制（系统→租户→成员→Key）                                     |
| 管理后台前端         | ✅ | Vue 3 + Naive UI、仪表盘、模型/渠道/租户/账单/权限管理                                |
| 租户控制台前端        | ✅ | Vue 3 + TailwindCSS、注册登录、仪表盘、API Key/用量/钱包/成员管理                      |
| 套餐与定价系统        | ✅ | 套餐 CRUD、Feature Flag、升降级差价计算、自动续费、月度额度重置                             |
| 订单与支付框架        | ✅ | 订单状态机、统一支付回调、退款审批、履约逻辑                                               |
| 支付渠道对接         | 🚧 | 框架已完成，支付宝/微信/Stripe 待对接                                              |
| 租户生命周期         | ✅ | 试用→活跃→逾期→冻结→终止、宽限期处理、注销冷却期、数据清理                                      |
| 兑换码/优惠券        | ✅ | 批量生成、使用记录、核销逻辑                                                       |
| 收入报表           | ✅ | 日/月度用量与收入汇总、租户额度水位快照                                                 |
| 成员管理增强         | ✅ | 批量导入、额度分配、模型范围分配、禁用/移除联动                                             |
| 项目管理           | ✅ | 项目预算控制、项目归档/恢复                                                       |
| 通知系统           | ✅ | 35+ 模板、站内信、邮件推送、通知偏好设置、发送失败重试                                        |
| 公告管理           | ✅ | 全局/定向公告发布、已读状态追踪                                                     |
| 工单系统           | ✅ | 提交/回复/分配/关闭工单、附件管理                                                   |
| 帮助中心           | ✅ | 文章/分类管理、富文本内容                                                        |
| 用户反馈           | ✅ | 反馈收集与管理                                                              |
| 更新日志           | ✅ | 版本变更记录发布                                                             |
| 系统监控           | ✅ | CPU/内存/磁盘/网络采集、Go 运行时指标、数据库/Redis 连接池监控                              |
| 应用指标           | ✅ | QPS/TPM/并发数/P95/P99 延迟/错误率                                           |
| 告警引擎           | ✅ | 告警规则 CRUD、检测引擎、抑制策略、通知分发、确认/解决流程                                     |
| 渠道健康可视化        | ✅ | 24h 健康趋势、自动禁用/恢复、健康评分快照                                              |
| 模型生命周期         | ✅ | active→deprecated→sunset→removed 状态机、Deprecation/Sunset 响应头          |
| 敏感词过滤          | ✅ | 4 种策略（关闭/仅记录/替换/拦截）、匹配引擎、中间件集成                                       |
| 维护模式           | ✅ | 控制台维护横幅、API 维护 503 响应、维护期代理继续服务                                      |
| 数据治理           | ✅ | 数据保留策略、自动清理、数据导出权/删除权、租户注销清理                                         |
| 异步任务管理         | ✅ | 任务状态查看、超时处理、Midjourney/Suno/可灵/Sora 适配                               |
| 开放平台           | ✅ | 应用管理、HMAC-SHA256 认证、成员/Key/用量/账单 Open API                            |
| Webhook        | ✅ | 30+ 事件订阅、签名验证、失败重试、投递日志                                              |
| OAuth/SSO      | ✅ | OAuth 集成、SSO 自动建户                                                    |
| API Playground | ✅ | 在线调试（真实调用、计费）                                                        |
| OpenAPI 文档     | ✅ | 3.0 规范自动生成                                                           |
| 图片模型适配验证       | 🚧 | 系统已有部分代码，但未进行验证                                                      |
| 视频模型适配验证       | 🚧 | 系统已有部分代码，但未进行验证                                                      |
| 嵌入模型适配验证       | 🚧 | 系统已有部分代码，但未进行验证                                                      |
| 新手引导流程         | ⬜ | 5 步引导、空状态提示                                                          |
| 在线审计           | ⬜ | 在线升级                                                                 |
| 演示站点           | ⬜ | 在线演示                                                                 |
| 审计日志分库存储       | ⬜ | 审计日志存入独立库中维护                                                         |
| 插件功能           | ⬜ | 计划参考GVA或者Hotgo的实现                                                    |

## 参与贡献

1. Fork 本仓库
2. 创建特性分支（`git checkout -b feature/amazing-feature`）
3. 提交更改（`git commit -m 'feat: 添加某功能'`）
4. 推送分支（`git push origin feature/amazing-feature`）
5. 发起 Pull Request

请遵循项目现有代码风格和 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/) 提交规范。

## 在线交流
欢迎加QQ群聊天吹水：1095286563

**提需求，反馈bug，摸鱼聊天都可以哦~**


## 许可证

本项目采用 [GNU Affero General Public License v3.0](LICENSE) 许可证。

### 核心要求

- **开源义务**：修改并分发本项目的代码，必须以相同协议开源
- **网络条款**：通过网络向用户提供基于本项目的服务（如 SaaS），也必须向用户开放修改后的源代码
- **自由使用**：个人学习、研究、内部使用、商业运营均可，前提是遵守上述开源义务

### 不适用场景

如果你想将本项目代码用于闭源商业产品，需要单独获取商业授权。请联系：**business@team-api.com**

## 致谢

- [GoFrame](https://goframe.org/) — Go 应用开发框架
- [new-api](https://github.com/Calcium-Ion/new-api) — AI 网关参考实现
- [sub2api](https://github.com/sub2api/sub2api) — 监控与亲和性参考实现
- [Naive UI](https://www.naiveui.com/) — Vue 3 组件库
