# Team-API — 项目开发规范

## 项目概述

多租户 大模型 API 网关 SaaS 平台。租户注册后获取 API Key，通过统一接口调用 OpenAI/Claude/Gemini 等大模型，平台负责计费、限流、监控。

双控制台架构：管理后台（平台运营）+ 租户控制台（组织使用），两套完全独立的用户体系。

## 技术栈

- **后端**：Go + GoFrame (v2) + goose（数据库迁移）
- **数据库**：PostgreSQL（唯一，不用 MySQL/SQLite）
- **缓存**：Redis + 内存缓存（双层缓存）
- **前端**：Vue 3 + Vite + TailwindCSS + Naive UI（管理后台）
- **包管理**：bun
- **对象存储**：S3 / OSS / COS / MinIO（禁止纯本地磁盘作为生产存储）

## 项目结构

项目基于 GoFrame v2 标准项目结构（`gf init` 生成），在此基础上扩展业务模块。

```
team-api/
├── api/                        # API 定义（请求/响应结构体 + 路由注解）
│   ├── admin/v1/               #   管理后台 API（gf gen ctrl 源）
│   ├── tenant/v1/              #   租户控制台 API（gf gen ctrl 源）
│   ├── captcha/v1/             #   验证码 API（管理后台 + 租户控制台共用）
│   ├── docs/v1/                #   OpenAPI 文档 API
│   ├── open/v1/                #   开放平台 API
│   ├── payment/v1/             #   支付回调 API
├── hack/
│   └── config.yaml             # gf CLI 脚手架配置（gen dao 等工具的数据库连接和生成规则）
├── internal/
│   ├── cmd/                    # 入口命令、服务启动、路由注册
│   │   ├── cmd.go              #   main 包注册，路由分组定义
│   │   └── cmd_reset_pwd.go    #   密码重置命令
│   ├── consts/                 # 常量定义（业务状态码、枚举等）
│   ├── controller/             # 控制器（由 gf gen ctrl 从 api/ 自动生成，禁止手动修改）
│   │   ├── admin/              #   管理后台控制器
│   │   ├── tenant/             #   租户控制台控制器
│   │   ├── captcha/            #   验证码控制器
│   │   ├── docs/               #   文档控制器
│   │   ├── open/               #   开放平台控制器
│   ├── handler/                # 特殊端点处理器（不走 Controller 链路，见下方说明）
│   │   ├── relay/              #   AI 代理端点处理器
│   │   ├── public/             #   支付回调端点处理器
│   │   └── setup/              #   系统初始化向导处理器
│   ├── logic/                  # 业务逻辑实现（核心代码所在）
│   │   ├── admin/              #   管理后台业务逻辑
│   │   ├── tenant/             #   租户控制台业务逻辑
│   │   ├── billing/            #   计费引擎
│   │   ├── common/             #   公共逻辑（缓存、配置、安全、邮件、验证码、JWT 等）
│   │   ├── docs/               #   OpenAPI 文档生成
│   │   ├── monitor/            #   监控告警引擎
│   │   ├── notification/       #   通知服务
│   │   ├── open/               #   开放平台业务逻辑
│   │   ├── payment/            #   支付逻辑（回调处理、订单履约）
│   │   ├── relay/              #   Relay 业务逻辑（亲和性、健康检查、缓存）
│   │   └── task/               #   异步任务框架
│   ├── model/                  # 数据模型（由 gf gen dao 自动生成，禁止手动修改）
│   │   ├── do/                 #   Domain Objects（查询条件、输入结构体）
│   │   └── entity/             #   Entity（与数据库表 1:1 映射的数据实体）
│   ├── packed/                 # 打包资源（内嵌静态文件、编译时生成）
│   ├── dao/                    # 数据访问对象（由 gf gen dao 自动生成，禁止手动修改）
│   ├── response/               # 统一响应工具包（Success/Error 封装）
│   ├── service/                # 业务接口定义（由 gf gen service 从 logic/ 自动生成，禁止手动修改）
│   ├── middleware/             # 中间件
│   └── utility/                # 工具函数
│       ├── crypto/             #   加密工具
│       ├── export/             #   数据导出
│       ├── totp/               #   TOTP 两步验证
│       └── turnstile/          #   Cloudflare Turnstile 验证
├── relay/                      # AI 模型代理层（顶级模块，独立于 GoFrame 脚手架生成目录）
│   ├── channel/                #   供应商适配器（registry.go + 24 个子目录，每个供应商一个）
│   │   └── openai/ claude/ gemini/ ...
│   ├── common/                 #   Relay 层共享类型和工具
│   ├── constant/               #   Relay 层常量（供应商类型、模式、错误码）
│   ├── dto/                    #   数据传输对象（OpenAI/Claude/Gemini/Realtime/Task 等格式定义）
│   ├── handler/                #   Relay 请求处理器（chat/claude/gemini/audio/rerank/task/realtime 等）
│   ├── helper/                 #   Relay 辅助函数（流式处理、状态码映射、系统提示词、thinking 处理）
│   ├── override/               #   请求/响应覆盖（Header 改写、参数覆盖）
│   ├── scheduler/              #   渠道调度器（调度、亲和性、重试）
│   └── taskchannel/            #   异步任务渠道适配器
│       ├── registry.go         #     任务渠道注册表
│       ├── kling/              #     可灵视频生成
│       ├── midjourney/         #     Midjourney 图像生成
│       ├── sora/               #     Sora 视频生成
│       └── suno/               #     Suno 音乐生成
├── manifest/
│   ├── config/                 # 应用运行时配置（GoFrame 标准路径）
│   ├── deploy/                 # 部署配置（K8s/Compose 等）
│   ├── docker/                 # Docker 构建配置
│   ├── i18n/                   # 国际化资源文件
│   └── protobuf/               # Protobuf 定义文件
├── migrations/                 # goose 数据库迁移脚本（六位序号编号：000001_xxx.sql）
├── web/                        # 前端
│   ├── admin/                  # 管理后台
│   └── tenant/                 # 租户控制台
├── docs/                       # 项目文档
│   ├── 开发计划-v2/            #   分周期开发计划（周期一～六）
│   └── 协议文档/               #   协议相关文档
├── new-api/                    # 参考项目（只读，不修改）
├── sub2api/                    # 参考项目（只读，不修改）
├── main.go                     # 应用入口
├── go.mod
└── Makefile
```

### Relay 层设计说明

Relay 层作为顶级 `relay/` 模块独立于 GoFrame 脚手架生成目录（`internal/` 下的 `controller/service/dao/model`），原因：
- **文件量大**：24 个供应商适配器 + 4 个异步任务适配器 + 共享模块，放在 `internal/logic/` 下层级过深
- **职责独立**：Relay 是纯粹的代理转发层，不依赖 GoFrame ORM/DAO，与 `internal/` 下的业务逻辑解耦
- **参考对齐**：目录结构与 new-api 的 `relay/` 保持一致，降低移植和理解成本

`internal/logic/` 中的业务逻辑（如计费、渠道调度）通过 Go import 调用 `relay/` 包，而非将 relay 嵌入 logic 内部。

### GoFrame 代码生成与手动编写分界

| 目录 | 生成方式 | 能否手动修改 |
|------|---------|------------|
| `api/` | 手动编写 | 是（结构体定义和路由注解） |
| `hack/config.yaml` | 手动编写 | 是（CLI 工具配置） |
| `internal/controller/` | `gf gen ctrl` | 否（自动生成） |
| `internal/service/` | `gf gen service` | 否（自动生成接口） |
| `internal/model/do/` | `gf gen dao` | 否（自动生成） |
| `internal/model/entity/` | `gf gen dao` | 否（自动生成） |
| `internal/dao/` | `gf gen dao` | 否（自动生成） |
| `internal/logic/` | 手动编写 | 是（核心业务逻辑） |
| `internal/cmd/` | 手动编写 | 是（路由注册、服务启动） |
| `internal/consts/` | 手动编写 | 是（常量定义） |
| `internal/handler/` | 手动编写 | 是（特殊端点处理器，见下方说明） |
| `internal/response/` | 手动编写 | 是（统一响应工具包） |
| `internal/middleware/` | 手动编写 | 是（中间件） |
| `internal/utility/` | 手动编写 | 是（工具函数：加密、导出、TOTP、Turnstile） |
| `manifest/config/` | 手动编写 | 是（运行时配置） |
| `relay/` | 手动编写 | 是（AI 模型代理层，独立于 GoFrame 脚手架） |

### internal/handler/ 目录说明

`internal/handler/` 目录包含**特殊端点的处理器**，这些端点不适合走 GoFrame 的标准 Controller 链路：

| 子目录 | 用途 |
|--------|------|
| `handler/relay/` | `/v1/*` AI 代理端点（OpenAI/Claude/Gemini 兼容格式）- 需要直接操作 `http.ResponseWriter` 进行流式转发 |
| `handler/public/` | `/api/payment/*` 支付回调端点 - 需要返回非 JSON 格式的响应（如支付宝的表单重定向） |
| `handler/setup/` | `/api/setup/*` 系统初始化向导端点 - 首次部署时的系统配置页面和初始化接口 |

**为什么不用 Controller**：
- **流式响应**：AI 代理端点需要实时转发上游的 SSE 流，Controller 的响应包装会破坏流式传输
- **原生格式透传**：必须保持与 OpenAI/Claude/Gemini API 完全兼容的请求/响应格式
- **自定义响应**：支付回调需要返回 HTML 表单或文本响应，而非统一的 JSON 格式

**与 Controller 的区别**：
- Controller：走 GoFrame 标准链路（`api → controller → service → logic → dao`），返回统一 JSON 响应
- Handler：直接处理 HTTP 请求/响应，不走 GoFrame 的 `ghttp.Request` 包装，用于特殊场景

### GoFrame 脚手架生成顺序（重要）

项目 `hack/config.yaml` 中 `gfcli.gen.ctrl` 已配置 `withService: true`，意味着 `gf gen ctrl` 会尝试自动将控制器连接到 service 方法。但前提是 **service 接口中已存在对应方法**，否则生成 `CodeNotImplemented` 桩代码。

**新增接口时必须按以下顺序执行，不可颠倒：**

1. **编写 API 定义**（`api/xxx/v1/xxx.go`）— 添加 Req/Res 结构体
2. **编写 Logic 实现**（`internal/logic/xxx/xxx.go`）— 实现业务逻辑
3. **`gf gen service`** — 从 Logic 方法签名自动生成 Service 接口
4. **`gf gen ctrl`** — 从 API 定义生成 Controller，此时 `withService: true` 会自动生成 `return service.Xxx().Method(ctx, req)` 调用

如果先执行 `gf gen ctrl` 再执行 `gf gen service`，控制器会是桩代码（返回 501 Not Implemented），需要手动修改。始终遵循 "logic → gen service → gen ctrl" 的顺序可避免此问题。

**重要：`gf gen ctrl` 执行后必须验证**。即使顺序正确，`withService: true` 的自动接线对新方法也可能失败，生成 `CodeNotImplemented` 桩代码。执行后务必检查：
```bash
grep -r "CodeNotImplemented" internal/controller/
```
如果有匹配，手动将桩代码替换为 `return service.Xxx().Method(ctx, req)` 调用。切勿假设 `gf gen ctrl` 的输出一定正确。

### Controller 采用 merge 模式（合并生成，强约束）

**本项目所有 controller 按 `merge` 模式生成**：`gf gen ctrl` 默认每个接口方法生成一个独立 `.go` 文件（一个模块动辄两三百个文件），项目已统一改为**按 `api/` 源文件合并**——每个 `api/xxx/v1/*.go` 对应一个 controller 文件（如 `api/admin/v1/ticket.go` 的所有方法合并进 `internal/controller/admin/admin_v1_ticket.go`）。因此：

- **生成命令必须带 `-m` 参数**：`gf gen ctrl -m`（不要用裸 `gf gen ctrl`，否则会按方法拆分，与现有合并文件产生重复方法定义、编译报错）。
  > ⚠️ `hack/config.yaml` 被 gitignore（含数据库密码），其中的 `merge: true` 不入库、不随仓库同步。**不能依赖 config，必须显式带 `-m`**。
- **`gf gen ctrl` 永不覆盖已存在的 controller 文件**：日常新增接口时，新方法会以 `CodeNotImplemented` 桩代码**追加**进对应的合并文件，手动接线一次即长期保留。仍按上文「`grep -r CodeNotImplemented` → 手动替换为 `return service.Xxx().Method(ctx, req)`」处理新桩代码。
- **切勿删除整个合并文件再重新生成**：合并文件里的手写 service 接线（含 controller 方法名 ≠ service 方法名的自定义映射，如 `TicketList → service.Admin().ListAllTickets`）一旦删文件重生成就会全部退化为桩代码，需要逐方法重接。只在确实新增/删除了 api 方法时让 `-m` 增量更新对应文件。

### GoFrame 请求处理链路

```
cmd（路由注册）
  → api（请求/响应结构体定义）
    → controller（解析请求、校验参数、调用 service）
      → service（接口定义，自动生成）
        → logic（业务逻辑实现，使用 DO 构造查询）
          → dao（ORM 操作，返回 Entity）
        ← 返回 Entity
      ← controller 格式化响应（Res 结构体）
```

## 参考项目（只读，禁止修改）

项目根目录下有 `new-api/` 和 `sub2api/` 两个参考项目，它们是大模型网关领域的成熟实现。
大多数情况下不需要查看这2个目录下的代码，除非我主动提及或者涉及到大模型相关的实现逻辑。

### new-api 重点参考

- **Relay 代理层架构**：`controller/relay/`，适配器模式、请求转发、响应解析
- **供应商适配器**：`relay/channel/` 下 40+ 供应商的适配器实现，优先复用
- **协议转换**：OpenAI ↔ Claude ↔ Gemini 消息格式互转
- **渠道调度**：优先级/权重选择、自动重试、Key 轮询
- **计费模型**：预扣/结算/退款、梯度定价、Token 计量

### sub2api 重点参考

- **渠道亲和性**：用户+模型→渠道映射缓存策略
- **运维监控**：仪表盘设计、健康监控页面
- **用户体系**：前端布局和组件设计

### 参考原则

参考不等于照搬。new-api 使用 Gin + GORM，我们需要适配到 GoFrame 框架。接口设计和核心算法可以参考，但代码需要重新组织以符合 GoFrame 标准结构（`api → controller → service → logic → dao` 分层）和项目的多租户架构。注意 GoFrame 的 `model/do/`、`model/entity/`、`dao/`、`service/`、`controller/` 均由脚手架自动生成，业务代码只写在 `api/`、`logic/`、`cmd/`、`handler/`、`middleware/`、`response/`、`utility/`、`consts/` 和顶级 `relay/` 中。

## 核心架构决策

### 多租户隔离

行级隔离：所有业务表包含 `tenant_id` 字段，GoFrame ORM 中间件全局注入 `WHERE tenant_id = ?`。请求链路通过 Context 传递 tenant_id。

### 路由前缀

```
/api/admin/     管理后台（JWT 认证，admin_users 表）
/api/tenant/    租户控制台（JWT 认证，tenant_users 表，RAM 格式 用户名@租户代码）
/api/payment/   支付回调（签名验证，无用户认证）
/api/captcha/   验证码服务（无需认证，管理后台 + 租户控制台共用）
/api/docs/      OpenAPI 文档（无需认证）
/api/open/      开放平台 API（HMAC-SHA256 认证）
/api/setup/     系统初始化向导（首次部署，完成后禁用）
/v1/            AI 代理端点 — OpenAI 兼容格式（API Key 认证，Bearer Token）
/v1beta/        AI 代理端点 — Gemini 兼容格式（API Key 认证，Bearer Token）
/suno/          Suno 音乐生成端点（API Key 认证）
```

### RBAC 权限模型

三层：角色 + 权限点 + 数据范围。公式：`is_allowed = has_permission_point AND within_data_scope AND resource_state_allows`。

管理后台角色：`super_admin`（全权限）、`admin`（可配置权限集）。
租户控制台角色：`owner`（全权限）、`admin`（管理权限）、`member`（个人数据）。

### 五层额度模型

```
租户钱包（货币池）→ 套餐额度（资源池）→ 成员额度（控制线）→ 项目预算（控制线）→ Key 额度（控制线）
```

扣费优先级：先扣套餐资源量，不足部分扣钱包余额。免费版额度耗尽停止服务，钱包余额不兜底。

### 计费流程

```
预扣（Redis 原子操作）→ 转发请求 → 结算（实际用量）→ 退款/补扣差额
```

价格查询优先级：租户独立价 > 套餐价 > 模型基础价 > 硬编码默认。

最终价格公式：`最终价格 = 基础价格 × 模型乘数 × 租户乘数`。模型乘数根据模型稀有度/成本动态调整；租户乘数根据租户等级/用量等级设置（VIP 折扣等）。双层乘数替代单纯的"租户独立定价"，提供更灵活的定价能力。

### 钱包冻结余额

wallets 表包含 `frozen_balance` 字段，用于支付中/退款中的金额冻结，防止并发超扣。可用余额 = `balance - frozen_balance`。预扣和支付操作需检查可用余额而非总余额。

### 系统货币规则

平台采用**内部统一 USD + 充值入口 CNY**策略，系统内除充值/支付外的所有金额均为 USD：

| 业务场景 | 货币 | 说明 |
|----------|------|------|
| 大模型价格展示和计费 | **USD（美元）** | 模型定价、Token 单价、计费扣费统一使用美元，与上游供应商（OpenAI/Anthropic/Google）报价币种一致 |
| 用户钱包余额 | **USD（美元）** | 钱包存储 USD 余额，充值时 CNY 按汇率转换为 USD 入账 |
| 计费记录 / 交易流水 | **USD（美元）** | 所有 bil_records、bil_transactions 金额均为 USD |
| 套餐价格 | **CNY（人民币）** | 管理后台创建套餐时设置的价格为人民币，用户购买套餐使用人民币支付 |
| 充值/支付 | **CNY（人民币）** | 用户通过支付渠道（支付宝/微信/Stripe）充值为人民币，履约时转换为 USD 入账钱包 |

#### 三层币种固定规则（强约束，写新代码必须遵守）

币种**按层固定**，不允许在层内混用。判断一个金额用什么币种，只看它属于哪一层，不需要逐字段记忆：

| 层 | 表前缀 | 币种 | 说明 |
|----|--------|------|------|
| **订单 / 支付层** | `ord_` | **永远 CNY** | 充值订单、套餐订单、退款，金额一律人民币。`ord_orders.currency` 默认值必须为 `CNY` |
| **钱包 / 计费 / 账本层** | `bil_` | **永远 USD** | 钱包余额、冻结余额、累计充值、交易流水、计费记录、用量、预扣/结算/退款，一律美元 |
| **套餐定价** | `pln_` | **CNY** | 套餐卖的是 token 额度（`monthly_quota_tokens`），价格用人民币，不与 USD 钱包直接比较 |
| **租户等级阈值** | `tnt_tenant_level_configs.cumulative_recharge_threshold` | **USD** | 与 `bil_wallets.cumulative_recharge`（USD）同币种比较 |

**唯一换汇点**：CNY → USD 的转换**只允许发生在充值履约**（`payment.FulfillOrder` 的 `recharge` 分支，调用 `billing.ConvertCNYToUSD`）。除此之外任何地方都不得做币种换算——套餐购买不入钱包不换算，计费引擎全程 USD 不换算。

**新增带金额字段的代码前，先确认它属于哪一层**，按上表取币种；跨层传递金额（如订单金额入钱包）必须经过唯一换汇点。

#### 汇率规则

- 只维护**单一方向**汇率 `payment_exchange_rate_cny_to_usd`（管理后台支付设置配置，默认 0.14）。需要反向（USD→CNY）时**取倒数**，禁止再配置第二个独立汇率，避免两个汇率漂移导致往返不闭合。
- **充值时必须快照汇率**：履约入账时除了写入换算后的 USD，还要持久化「原始 CNY 金额 + 当时汇率 + 入账 USD」，使历史换算可重建。汇率配置变更不得影响已完成订单的对账。
- **退款认原始 CNY 金额**：退款按订单原始人民币金额退回支付渠道，**禁止**用钱包当前 USD 余额按当前汇率反向折算（防止汇率波动产生套利/亏损）。

#### 前端展示规则

- 钱包余额 / 交易流水 / 计费 / 用量 / Token 单价 → `$`（USD）
- 套餐价格 / 充值金额 / 订单金额 / 退款金额 → `¥`（CNY）
- ⚠️ 营收类指标（`total_cost`、`revenue` 等来自 `bil_usage_logs` 的统计）是 **USD**，用 `$`，不要因为面向中国区运营就标成 `¥`。需要人民币营收报表时，在**出报表时**按快照汇率折算，账本本身仍存 USD。

#### 人民币对账 / 导出规则

判断要不要存 CNY，只看**这笔钱有没有真的以人民币发生过**，两种对账答案相反：

| 对账类型 | CNY 是否真实发生 | 做法 |
|----------|------------------|------|
| **现金对账**（充值 / 退款，对支付宝/微信/银行流水） | ✅ 真实发生 | 在**充值订单/充值交易**上快照「原始 CNY 金额 + 当时汇率 + 入账 USD」，CNY 是真相、USD 是派生 |
| **消费对账**（AI 用量 / 计费扣费） | ❌ 从未发生（逐笔均为 USD 计算） | **禁止**在 `bil_usage_logs` 等高频写入表加 CNY 快照字段，导出时折算 |

**为什么消费记录不加 CNY 快照字段**：① 没有真实人民币事件可对，存进去是虚构事实；② `bil_usage_logs` 是全系统写入量最大的表，加派生列存储代价大；③ 逐行存 CNY 再 SUM 会累积舍入误差；④ 制造与 USD 列冲突的第二真相源，反而新增对账问题。

**消费需要展示/导出人民币时**：先按 USD 聚合（`SUM(total_cost)`），**最后乘一次汇率**得到 CNY；报表头注明「所用汇率 + 折算口径 + 仅供参考，非实际支付/开票金额」。汇率用统一的报表汇率（或 `payment_exchange_rate_cny_to_usd` 取倒数），不得逐行各取各的。

**发票 / 税务口径**：合法可开票的人民币金额是用户**实际支付的充值额**（真实 CNY），绝不是「消费额 × 汇率」。租户人民币对账单/发票一律从充值记录出，精确到分、与支付渠道流水一一对应。

> 决策规则：**钱真的以人民币动过 → 存真实 CNY 快照（充值/退款）；没动过、只是想看人民币 → 导出时对聚合 USD 折算一次，不落库。**

## 数据库表前缀定义

按功能域划分，新建表时必须归入对应前缀，除非加入了新的功能模型，否则不得自行新增前缀。

| 前缀 | 功能域 | 周期 | 涉及表（示例） |
|------|--------|------|--------------|
| `sys_` | 平台系统 + 系统配置 | 一 | `sys_admin_users` `sys_admin_role_perms` `sys_admin_data_scopes` `sys_sessions` `sys_email_verify_codes` `sys_idempotency_records` `sys_options` |
| `tnt_` | 租户核心 + 项目管理 | 一 | `tnt_tenants` `tnt_users` `tnt_invitations` `tnt_member_imports` `tnt_member_model_scopes` `tnt_projects` |
| `mdl_` | AI 模型 | 一 | `mdl_models` `mdl_pricing_tiers` `mdl_tenant_models` |
| `chn_` | 渠道管理 | 一 | `chn_channels` `chn_abilities` `chn_health_scores` `chn_channel_keys` `chn_channel_affinities` |
| `api_` | API 密钥 | 一 | `api_keys` `api_key_model_scopes` |
| `bil_` | 计费账单 | 一 | `bil_records` `bil_wallets` `bil_transactions` `bil_usage_logs` `bil_daily_usage_summary` `bil_daily_revenue_summary` `bil_monthly_usage_summary` `bil_monthly_revenue_summary` |
| `ord_` | 订单支付 + 优惠券 | 二 | `ord_orders` `ord_refunds` `ord_payment_channels` `ord_promo_codes` `ord_promo_code_usages` `ord_redemptions` `ord_redemption_usages` |
| `pln_` | 套餐订阅 | 二 | `pln_plans` `pln_tenant_plans` `pln_feature_flags` |
| `ntf_` | 通知消息 | 三 | `ntf_templates` `ntf_send_log` `ntf_messages` `ntf_read_status` `ntf_preferences` `ntf_announcements` |
| `aud_` | 审计日志 | 三 | `aud_request_logs` `aud_operation_logs` `aud_sensitive_access_logs` `aud_login_history` |
| `spt_` | 客户支持（工单+帮助+反馈） | 三/六 | `spt_tickets` `spt_replies` `spt_attachments` `spt_categories` `spt_articles` `spt_feedbacks` |
| `ops_` | 运维监控 | 四 | `ops_alert_rules` `ops_alert_events` `ops_system_metrics` |
| `tsk_` | 异步任务 | 四 | `tsk_tasks` `tsk_task_logs` `tsk_model_tasks` |
| `fil_` | 文件存储 | 四 | `fil_files` |
| `opn_` | 开放平台 + Webhook | 五 | `opn_apps` `opn_webhook_configs` `opn_webhook_events` `opn_webhook_delivery_logs` |
| `clg_` | 更新日志 | 六 | `clg_changelogs` |

### 前缀使用规则

- 新建表必须从上表选择前缀，不允许自行新增前缀
- 如果一个表明显属于两个域，选择核心归属域的前缀
- 通用/跨域表（如 `sys_options` 系统配置）放在最相关的域下
- 迁移脚本文件名按现有文件顺序递增编号，格式：`{序号}_{变更内容描述}.sql`，例如 `000002_alter_mdl_pricing_cache_fields.sql`

## 开发规范

### Go 后端

- **缩进统一使用 tab**：所有 Go 代码的缩进必须使用 tab，与 `go fmt` 自动格式化保持一致，禁止混用空格和 tab 缩进
- GoFrame 标准项目结构，不要偏离。框架使用规范（时间处理、错误处理、ORM 模式、日志等）详见 [`docs/reference/goframe-conventions.md`](docs/reference/goframe-conventions.md)，遇到不确定的 GoFrame 用法时善用 `/goframe-v2` skill 查询框架规范
- **修复 GoFrame 框架使用 bug 时**，必须在 `docs/reference/goframe-conventions.md` 末尾的「已修复的框架使用错误记录」章节追加记录（问题描述、原因、修复方式），防止同类问题重现
- 数据库迁移用 goose，脚本按六位序号递增编号（`migrations/000001_xxx.sql`）
- 所有业务表必须有 `id`, `created_at`, `updated_at` 字段
- 审计相关表按月分区（audit_logs）
- 使用量大的写入操作（usage_logs）异步批量写入
- 错误响应统一格式：`{"error": {"type": "...", "message": "...", "code": "...", "request_id": "..."}}`
- 每个 API 请求生成唯一 Request ID，贯穿全链路
- 敏感字段（密码、API Key 原值）AES-256 加密存储
- 缓存策略：API Key 60s、用户信息 300s、模型定价 600s、渠道信息 300s
- 代码注释统一使用中文编写

### 前端

- Vue 3 Composition API + `<script setup>` 语法
- **管理后台**：使用 **Naive UI** 组件库，快速构建数据密集型页面（表格、表单、图表等），可参考 [Soybean Admin](https://github.com/soybeanjs/soybean-admin) 的架构设计（路由、权限、布局）
- **租户控制台**：TailwindCSS 原子类 + 自建组件，面向客户，注重品牌调性和视觉差异化
- TailwindCSS 作为基础样式层，两个控制台共用（Naive UI 的 CSS-in-JS 方案与 TailwindCSS 无冲突）
- 状态管理用 Pinia
- HTTP 请求封装 Axios，JWT 自动刷新，统一错误处理
- 按钮级权限控制：无权限的按钮不渲染（`v-if="hasPermission('xxx')"`）
- 前端不用 React，不用 JSX

### 数据库

- 只用 PostgreSQL，写 SQL 迁移脚本时使用 PostgreSQL 语法
- **数据库是远程连接**，本地没有 psql 等命令可用，数据库操作只能通过 goose 迁移脚本执行
- **表名带模块前缀**：同一模块/功能/域的表使用统一前缀（见下方《数据库表前缀定义》），提升可维护性
- **全部小写蛇形命名**：表名和字段名一律 `snake_case`，禁止驼峰或大写
- **禁止数据库级外键**：所有关联都是逻辑关联，不使用 `REFERENCES`/`FOREIGN KEY`，在代码层面处理关联关系和级联操作
- **所有字段必须带注释**：建表时每个字段都加 `COMMENT '说明'`，便于后续维护
- JSONB 字段用于灵活配置（功能权限、通知偏好等）
- 字符串字段注意长度限制，不要用 `VARCHAR(255)` 一刀切
- 金额字段统一用 `NUMERIC(20,10)`（PostgreSQL 中 NUMERIC 与 DECIMAL 等价），避免累计计算精度丢失，不用 FLOAT
- **资金显示精度**：系统中所有资金相关数值（钱包余额、交易金额、计费金额、单价等）在 API 响应和前端展示时一律精确到小数点后 6 位（特殊说明除外），与 `NUMERIC(20,10)` 存储精度对齐，保证前端显示与后端计算结果一致
- 索引按实际查询场景创建，不过度索引
- **追加写表使用 BRIN 索引**：`usage_logs`、`audit_logs`、`billing_records` 等按时间追加写入的表，在 `created_at` 字段上使用 BRIN 索引（而非 B-tree），大幅节省存储空间

### API 接口

- **URL 路径统一小写蛇形**：`/api/admin/tenant-plans` 而非 `/api/admin/tenantPlans`
- GoFrame 有脚手架命令（`gf gen`），能使用脚手架生成的代码（model/dao/api）就用脚手架生成，不要手写
- 完整的 JSON 示例、SSE 流式格式、各供应商响应格式、错误类型映射、中间件差异等详见 [`docs/reference/api-format-reference.md`](docs/reference/api-format-reference.md)

#### 统一响应格式（`/api/*` 管理类接口）

所有管理类接口（`/api/admin/*`、`/api/tenant/*`、`/api/payment/*`、`/api/open/*`）使用统一 JSON 结构：`{"code": 0, "message": "ok", "data": ..., "request_id": "..."}`。`code=0` 成功，非 0 错误。HTTP 状态码照常设置，前端以 `code` 为主。

**data 字段规范**：分页列表 `{list, total, page, page_size}`，不分页 `{list}`，创建 `{id}`，详情直接返回对象，更新/删除 `null`。数组字段名统一用 `list`，list 内部可为空但不能有 null。

**后端**：统一使用 `internal/response` 包（`response.Success` / `response.Error`），禁止直接拼 `g.Map`。业务错误码 >= 10000 定义在 `internal/consts/consts.go`，映射为 HTTP 422。`message` 必须中文，禁止暴露技术细节。

**前端**：Axios 拦截器以 `code === 0` 为唯一成功判据，统一处理错误提示。所有 API 调用 `try/catch` 包裹。

#### 大模型代理接口（`/v1/*`、`/v1beta/*`、`/suno/*`）

代理端点是 OpenAI/Claude/Gemini 等 AI 服务的反向代理，必须与上游 API 格式完全兼容。核心规则：

- **禁止使用统一响应格式和 `internal/response` 包**，直接操作 `http.ResponseWriter`
- **原生透传**：上游响应原样转发，错误也用供应商原生格式
- 错误处理：`WriteRelayError`（OpenAI 格式）、`WriteClaudeRelayError`（Claude 格式）

### Definition of Done（功能完成标准）

每个功能模块交付时需满足以下标准：

| # | 标准 | 说明 |
|---|------|------|
| 1 | 功能通过手动测试 | 核心业务流程端到端可跑通 |
| 2 | 单元测试覆盖率 ≥ 80% | 核心计费/调度/权限逻辑必须有单元测试 |
| 3 | 接口文档同步更新 | API 端点变更时同步更新 OpenAPI 文档 |
| 4 | 数据库迁移可重复执行 | goose up/down 幂等，不破坏已有数据 |
| 5 | 错误码与提示文案完整 | 所有错误场景有明确错误码和用户可读消息 |
| 6 | 日志记录完整（含 trace_id） | 关键业务操作日志贯穿 request_id |
| 7 | 权限校验覆盖所有端点 | 新增端点必须配置权限点，无权限返回 403 |
| 8 | 代码审查通过 | 核心模块需 Code Review 后合入 |


## 文档索引

关键文档在 `docs/` 下，需要时直接阅读：

| 文档 | 路径 | 用途 |
|------|------|------|
| 产品需求文档 | `docs/team-api产品需求文档.md` | 功能需求的最终依据 |
| 原始开发计划 | `docs/team-api开发计划.md` | S01-S26 迭代拆分 |
| new-api 调研 | `docs/new-api调研报告.md` | 参考项目能力盘点 |
| sub2api 调研 | `docs/sub2api调研报告.md` | 参考项目能力盘点 |
| 对比分析 | `docs/new-api-vs-sub2api对比分析报告.md` | 两者差异 |
| 大模型实施方案 | `docs/大模型完整实施方案-v2.1.md` | 大模型代理层设计 |
| API 格式参考 | `docs/reference/api-format-reference.md` | JSON 示例、SSE 格式、错误映射、中间件差异 |
| GoFrame 使用规范 | `docs/reference/goframe-conventions.md` | 框架使用规范 + 已修复的框架 bug 记录 |


## 开发环境

- **操作系统**：Windows
- **GoFrame CLI**：已安装 `gf` 脚手架，优先使用 CLI 完成以下工作：

| 命令 | 用途 | 何时使用 |
|------|------|---------|
| `gf init` | 初始化项目骨架 | 项目创建时 |
| `gf gen dao` | 从数据库生成 DAO/DO/Entity 代码 | 表结构变更后 |
| `gf gen service` | 从 Logic 层生成 Service 接口 | 业务接口变更后 |
| `gf gen ctrl -m` | 从 API 定义生成 Controller（**merge 合并模式，必带 `-m`**） | API 定义变更后 |
| `gf gen pb` | 生成 protobuf 文件 | 需要时 |
| `gf run` | 热编译运行开发服务器 | 开发调试 |
| `gf build` | 编译生产二进制 | 构建部署 |
