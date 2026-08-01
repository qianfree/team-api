# 渠道调度器重构设计（team-api）

> **状态**：设计定稿（2026-07-30），尚未动工。本文是 clean-slate 重写的实现规格基线。
> **背景**：当前调度实现的问题清单见 [`channel-scheduling-issues.md`](./channel-scheduling-issues.md)。本文不受现状约束，重新设计一套"理想"方案，而非在现有代码上打补丁。
> **范围**：`relay/scheduler`（纯调度逻辑）+ `internal/logic/relay`（DB/Redis adapter）+ `relay/handler`（执行器）的调度相关部分。不涉及计费、协议转换、流式转发等调度以外的能力。

---

## 0. 已敲定的关键决策

| # | 决策点 | 定论 |
|---|---|---|
| 1 | 路由状态真相源 | **Redis 为真相源**：健康分用 Lua 原子 EWMA，并发占用沿用 ZSET；`chn_health_scores` 降级为周期快照。Redis 故障用本地 last-known 快照降级 |
| 2 | 成本感知 | **直接开启**：`costFactor` 进权重，同模型等价渠道优先更便宜的上游；运营维护每渠道实际上游单价 |
| 3 | 对话指纹 | **不做**：会话令牌仅认显式信号（`X-Session-Id` / `previous_response_id` / `conversation_id`），无则回退身份级 |
| 4 | 熔断探活 | **自然探活**：HALF_OPEN 时 Redis 原子计数器每窗口放行 1 个真实请求探测 |

---

## 1. 设计目标与原则

调度器需同时优化多个**互相矛盾**的目标：

| 目标 | 含义 | 主要矛盾对象 |
|---|---|---|
| 正确性 | 只路由到有能力/有权限服务该模型的渠道 | — |
| 可用性 | 渠道故障用户无感（重试/failover） | 粘性 |
| 成本 | 同模型优先更便宜的上游 | 负载均衡 |
| 粘性 | 同一会话命中同一渠道 → 命中上游 KV/prompt 缓存 | 负载均衡 |
| 负载均衡 | 按容量摊开流量，用满备用渠道 | 粘性、优先级独占 |
| 多租户隔离 | 租户级路由范围、配额、不互相挤占 | — |

**六条设计原则**：

1. **策略与机制分离**：调度器只决定"用哪个渠道"；执行器只做 HTTP。重试/退避是数据驱动的策略，不是散落在 handler 里的分支。
2. **选择 = 对候选集打分采样**：优先级、健康、负载、成本、亲和统一成同一个评分公式的因子。
3. **状态事件驱动 + 原子更新**：每个请求结果是一个事件；状态用 Redis Lua/原子操作更新，**从结构上消灭读写竞态**，多实例安全。
4. **粘性是软信号，不是硬覆盖**：亲和由 HRW 种子承担，是涌现属性，永远要和健康/负载竞争，绝不绝对命中。
5. **失败处理是状态机**：请求生命周期 = 显式 FSM，由错误分类器驱动转移。
6. **调度模块是纯函数、零 I/O**：所有 DB/Redis 在接口背后；调度逻辑 100% 可单测、可复现并发 bug。

---

## 2. 顶层架构

```
┌──────────────────────────────────────────────────┐
│  Executor（relay/handler）                        │  机制层：HTTP/SSE、协议适配器
│  只认识 HTTP 和流式，只调 Scheduler 接口           │  负责 buildRelayInfo / 转发 / 计费钩子
└──────────────┬───────────────────────────────────┘
               │ 依赖接口，不依赖实现
┌──────────────▼───────────────────────────────────┐
│  Scheduler（relay/scheduler，纯逻辑）              │  ★ 核心模块，零 dao/redis import
│   ├─ Selector          加权 HRW 打分选择           │
│   ├─ RetryFSM          错误分类 + 状态机           │
│   ├─ AffinityResolver  会话令牌解析 + 绑定守卫      │
│   └─ Policy            版本化路由策略（热加载）     │
└──────────────┬───────────────────────────────────┘
               │ 依赖端口（接口）
┌──────────────▼───────────────────────────────────┐
│  Ports: ChannelCatalog · HealthStore · LoadGauge  │
│         AffinityStore · CostCatalog               │
└──────────────┬───────────────────────────────────┘
               │ 由 adapter 实现（internal/logic/relay）
┌──────────────▼───────────────────────────────────┐
│  PostgresChannelCatalog(内存缓存) · RedisHealthStore│
│  (Lua EWMA + 熔断) · RedisLoadGauge(ZSET)          │
│  RedisAffinityStore · PostgresCostCatalog          │
└──────────────────────────────────────────────────┘
```

**关键性质**：`relay/scheduler` 包 `import` 列表里**不得出现** `dao` / `g.DB()` / `g.Redis()`。所有 I/O 通过端口接口注入。这使得选号、重试、溢出、熔断、亲和衰减全部可用 fake store 单测，包括用并发测试复现历史上的竞态 bug。

---

## 3. 核心选择模型：加权 HRW

### 3.1 单一原语

对每个候选渠道 `c`，给定请求种子 `seed`：

```
pick = argmax_c  weight(c, r) / -ln( μ( hash(seed, c) ) )
```

其中 `μ ∈ (0,1)` 由 `hash(seed, c)` 派生的均匀分布（参考现有 `weightedRendezvousSelect` 的 53 位精度构造，避免 `log(0)`）。

`seed = 会话令牌`（见 §4）。代入后，**这一个公式同时给出**：

- **粘性**：同一 session → seed 相同 → 选出同一渠道（命中上游缓存）。
- **负载均衡**：不同 session → seed 不同 → 大量会话按 `weight` 比例自动摊到各渠道。
- **跨实例一致**：纯函数 (seed, 权重)，多实例无需协调即得出同一结果。
- **自适应**：`weight` 含健康/负载因子，病渠道权重↓ → 新会话自然不再落到它。

> 粘性由此成为 HRW 的**涌现属性**，而非一个要和健康度打架的硬覆盖。**"亲和绕过健康"（现状 A1）在新模型里结构性不存在**：病渠道权重低，HRW 自动绕开。

### 3.2 复合权重

```
weight(c, r) = baseWeight(c)
             × health(c)            ∈ [0,1]    §5 健康分（含熔断器 gating）
             × capacityHeadroom(c)  ∈ (0,1]    §6 = 1 − inFlight/softLimit
             × costFactor(c, r)     ∈ (0,1]    §8 同模型优先便宜上游
             × tierBias(c)          ∈ (0,1]    首选 1.0 / 备用 β₂ / 保底 β₃
             × rampFactor(c)        ∈ (0,1]    新渠道/恢复渠道灰度爬坡
```

任一因子为 0（如熔断 OPEN → `health=0`）即该渠道本轮不参与。

### 3.3 绑定守卫（防抖）

权重会随健康/负载漂移，纯 argmax 可能在两个相近渠道间反复横跳。引入 hysteresis：

1. 会话**首次**请求：加权 HRW 选号 → 把绑定写入 `AffinityStore`（带 TTL）。
2. **后续**请求：若绑定存在且被绑渠道 `weight ≥ keepThreshold` → 直接复用（粘性）。
3. 若被绑渠道 `weight < keepThreshold`（病了/满了）→ 重跑 HRW，重新绑定。

结果：**健康时绝对粘，退化时优雅迁移并重新粘住**，不抖。`keepThreshold` 默认 `0.3 × baseWeight`，可在 routing_policy 配置。

---

## 4. 亲和子系统（决策 3）

### 4.1 会话令牌解析

亲和单位 = **会话**；身份 (tenant:user:key:model) 是**命名空间**。令牌按优先级取第一个非空：

```
sessionToken =
   1) request.Header["X-Session-Id"]            // 自研 SDK 显式指定，完全可控
   2) body.previous_response_id                  // OpenAI Responses API 链式对话
   3) body.conversation_id                       // Coze / Dify / Assistants thread
   4) nil → 身份级回退（不做对话指纹，决策 3）
```

> Claude Code（Anthropic Messages API）、OpenAI Chat Completions 协议无 session id，且不做指纹 → **走身份级粘性**。其"单用户多会话可能先挤一个渠道"的担忧由 §6 负载溢出兜住：渠道打满 → `capacityHeadroom→0` → weight 跌破 `keepThreshold` → HRW 自动溢到下一渠道并重新粘住。

### 4.2 存储（Redis）

沿用现有 `relay:affinity:v2` 的双向结构，重命名到新前缀：

| 用途 | Key | 值 |
|---|---|---|
| 会话 → 渠道绑定 | `sched:aff:{tenantID}:{model}:{sessionTokenHash}` | `channelID`（string，TTL = affinity_ttl，默认 1800s） |
| 渠道 → 绑定集合（失效用） | `sched:affindex:{channelID}` | SET of 绑定 key（TTL = affinity_ttl + 60s） |

- 绑定写入用现有 Lua 脚本模式（先清旧反向索引、再 SETEX + SADD）。
- `InvalidateChannel(channelID)`：读反向集合，逐个 DEL 绑定 key（渠道禁用/下线时调用，已有 `InvalidateChannelAffinities`）。
- 令牌 hash 用 SHA-256 截断，避免长 header/previous_response_id 直接当 key。

### 4.3 接口

```go
type AffinityStore interface {
    Get(ctx context.Context, ns AffinityNamespace, token string) (channelID int64, ok bool)
    Bind(ctx context.Context, ns AffinityNamespace, token string, channelID int64, ttl time.Duration)
    InvalidateChannel(ctx context.Context, channelID int64) int // 返回清除的绑定数
}
type AffinityNamespace struct{ TenantID, UserID, ApiKeyID int64; Model string }
```

---

## 5. 健康子系统（决策 1、4）

### 5.1 模型：熔断器 × 连续评分

每个渠道一个**熔断器** + 一个**连续健康分**，全部 Redis 原子维护。

| 状态 | 含义 | 权重影响 |
|---|---|---|
| `CLOSED` | 健康 | `health(c)` ∈ [0,1] 正常调节权重 |
| `OPEN` | 故障 | `health(c) = 0`，跳过 |
| `HALF_OPEN` | 探活中 | 仅放行探测请求（见 5.4），其余 `health=0` |

转移：
- `CLOSED → OPEN`：滑动窗口内错误率 > `cb_error_rate`（默认 0.5），或连续失败 ≥ `cb_consecutive`（默认 5）。
- `OPEN → HALF_OPEN`：OPEN 持续超过 `cb_cooldown`（默认 30s），惰性判定（下次访问时按 `opened_at` 计算）。
- `HALF_OPEN → CLOSED`：探测请求成功。
- `HALF_OPEN → OPEN`：探测请求失败，重置 `opened_at`。

### 5.2 连续健康分

```
health(c) = 0.40·successScore + 0.25·latencyScore + 0.20·stabilityScore + 0.15·errorDistScore
```

- `successScore`：EWMA 成功率（0–100 → 归一到 0–1）。
- `latencyScore`：<1s=100, 1–3s=80→50, 3–10s=50→20, >10s=20（沿用现有 `calcLatencyScore` 分段）。
- `stabilityScore`：延迟波动 EWMA（沿用现有 `calcStability`）。
- `errorDistScore`：按错误类别加权（auth 错误扣分 > 5xx > 429），近因衰减。

> 该公式在现有 `health.go` 基础上保留权重配比，但**计算移入 Lua 脚本内原子完成**，所有输入因子都在同一 hash 里，score 永远与输入一致——彻底消除现状 `UpdateHealthScore` 的读改写竞态（H1）。

### 5.3 Redis 数据结构

Key：`sched:health:{channelID}`（hash）

| 字段 | 说明 |
|---|---|
| `success_rate` | EWMA 成功率 |
| `latency` | EWMA 延迟(ms) |
| `stability` | 稳定性分 |
| `err_4xx_auth` / `err_5xx` / `err_429` | 错误类别计数（衰减） |
| `consecutive_fail` | 连续失败数 |
| `health_score` | 综合健康分（Lua 内重算） |
| `state` | `CLOSED` / `OPEN` / `HALF_OPEN` |
| `opened_at` | 进入 OPEN 的时间戳 |
| `probe_owner` | 当前 HALF_OPEN 探测请求的 token（自然探活用） |
| `updated_at` | 最近更新时间 |

### 5.4 Lua 脚本（参考实现）

**OBSERVE**（请求结束时调用，原子更新 EWMA + 重算 health + 熔断转移）：

```lua
-- KEYS[1] = sched:health:{channelID}
-- ARGV = [alpha, success(0/1), latencyMs, errClass, now,
--         cb_consecutive, cb_cooldown_ms, affinity_ttl]
local h = KEYS[1]
local alpha    = tonumber(ARGV[1])
local success  = ARGV[2] == '1'
local lat      = tonumber(ARGV[3])
local errClass = ARGV[4]
local now      = tonumber(ARGV[5])
local cbCons   = tonumber(ARGV[6])
local cbCool   = tonumber(ARGV[7])

local state = redis.call('HGET', h, 'state') or 'CLOSED'
local opened = tonumber(redis.call('HGET', h, 'opened_at') or '0')

-- OPEN -> HALF_OPEN 惰性转移
if state == 'OPEN' and (now - opened) >= cbCool then
  state = 'HALF_OPEN'
end

local sr = tonumber(redis.call('HGET', h, 'success_rate')) or 100
local cf = tonumber(redis.call('HGET', h, 'consecutive_fail')) or 0
local latE = tonumber(redis.call('HGET', h, 'latency')) or lat

if success then
  sr = sr * alpha + 100 * (1 - alpha)
  latE = latE * alpha + lat * (1 - alpha)
  cf = 0
  if state == 'HALF_OPEN' then state = 'CLOSED' end           -- 探活成功
else
  sr = sr * alpha                                              -- 失败降成功率
  cf = cf + 1
  if state == 'HALF_OPEN' then
    state = 'OPEN'; opened = now                               -- 探活失败，重新熔断
  elseif cf >= cbCons then
    state = 'OPEN'; opened = now                               -- 连续失败熔断
  end
end

-- 重算 health_score（所有因子已在 hash 中，可一并算出）
local score = ComputeHealth(sr, latE, ...)                     -- 伪码：同 §5.2 公式

redis.call('HMSET', h,
  'success_rate', sr, 'latency', latE, 'consecutive_fail', cf,
  'health_score', score, 'state', state, 'opened_at', opened,
  'updated_at', now)
return redis.call('HMGET', h, 'state', 'health_score')
```

**PROBE_ACQUIRE**（HALF_OPEN 自然探活闸门，每窗口放行 1 个真实请求）：

```lua
-- KEYS[1] = sched:health:{channelID}
-- ARGV = [now, probe_window_ms, probe_token]
local h = KEYS[1]
local state = redis.call('HGET', h, 'state') or 'CLOSED'
if state ~= 'HALF_OPEN' then return 0 end                       -- 非 HALF_OPEN 不放行
local now = tonumber(ARGV[1])
local win = tonumber(ARGV[2])
local owner = redis.call('HGET', h, 'probe_owner_at') or '0'
if (now - tonumber(owner)) < win then return 0 end              -- 窗口内已放过
redis.call('HMSET', h, 'probe_owner', ARGV[3], 'probe_owner_at', now)
return 1                                                        -- 放行该探测请求
```

> 被放行的探测请求走真实业务路径；其结果仍通过 OBSERVE 上报，OBSERVE 内的 HALF_OPEN 分支负责转 CLOSED 或重回 OPEN。

### 5.5 DB 快照

`chn_health_scores` 表保留，但**降级为周期快照**：一个后台任务每 N 秒把 Redis hash 落盘，仅供管理后台仪表盘/审计。路由决策不读此表。

---

## 6. 负载子系统：优雅溢出

### 6.1 实时并发占用

沿用 `internal/logic/relay/capacity.go` 的 Redis ZSET（requestID → 时间戳，30s TTL 续租）。`inFlight(c) = ZCARD sched:load:{channelID}`，跨实例实时。

### 6.2 软上限 softLimit

```
softLimit(c) = manual_max_concurrency  若已显式配置
             否则 = 自动推断值
```

**自动推断**：观察到渠道在 `inFlight ≈ X` 时开始返回 429 → `softLimit` EWMA 收敛到 `X × safety`（safety 默认 0.9）。存于 `sched:loadmeta:{channelID}`（hash: `manual_limit`, `auto_limit`, `updated_at`）。运营不填 `max_concurrency` 时溢出仍可工作。

### 6.3 头余量与溢出

```
capacityHeadroom(c) = clamp(1 − inFlight(c)/softLimit(c), 0, 1)
```

- 正常（首选空闲、`tierBias=1.0`）：首选权重碾压备用（`tierBias=0.1`），流量基本只打首选。
- 首选被打满：`capacityHeadroom→0` → 有效权重下降 → 备用相对权重上升 → 新会话**自动溢到备用**。
- 首选全挂（`health=0`）：全量 failover 到备用。

> **failover（健康驱动）与 spillover（负载驱动）是同一权重机制的两种表现**，替代现状"最高优先级组独占 + 失败才被动触达备用"（L2）。

### 6.4 接口

```go
type LoadGauge interface {
    InFlight(ctx context.Context, channelID int64) (int, error)
    SoftLimit(ctx context.Context, channelID int64) (int, error)
    // Acquire/Release 复用现有 capacity.go 的 slot 机制（ZADD/ZREM + 续租）
    Acquire(ctx context.Context, channelID, max int, requestID string) bool
    Release(ctx context.Context, channelID int64, requestID string)
    // 429 观测，驱动 softLimit 自动推断
    ObserveRejection(ctx context.Context, channelID int64, inFlightAtReject int)
}
```

---

## 7. 失败处理：FSM + 错误分类器

### 7.1 错误分类

| 类别 | 典型 | 语义 |
|---|---|---|
| `TRANSIENT` | 429, 502, 503 | 上游瞬时，大概率 ms 级恢复 |
| `NETWORK` | timeout, EOF, RST, DNS | 网络抖动，可能未送达 |
| `CHANNEL_FATAL` | 401, 403, 404, SSL | 渠道对该请求已不可用 |
| `CLIENT` | 400 | 客户端错误，重试无意义 |
| `PARTIAL_STREAM` | 流已部分写入 | 不可重试（会污染响应） |

分类器输入：HTTP 状态码 + 错误类型 + `IsResponseWritten`/`IsPartialStreamEnd` + relayMode（非幂等生成特判）。复用现有 `constant.IsRetryable` / `IsRetryableForRequest(expensiveGen)` 逻辑，提升为显式分类表。

### 7.2 状态机

```
                       ┌────────────────────────────┐
                       ▼                            │
SELECT ──► ATTEMPT ──► outcome                     │
              │                                    │
              ├ SUCCESS ──► Observe(✓) ──► DONE    │
              │                                    │
              ├ TRANSIENT / NETWORK (可重试)        │
              │   └ 非幂等生成的"可能已送达"网络错误 → DONE(✗)
              │   └ 幂等 → Backoff ──► ATTEMPT(同渠道)   原地预算 k
              │                                    │
              ├ TRANSIENT(原地耗尽) / CHANNEL_FATAL  │
              │   └ Failover ──► SELECT(exclude)        换渠道预算 m
              │                                    │
              └ CLIENT / PARTIAL_STREAM / 全耗尽 ──► DONE(✗)
```

- **原地重试**（同渠道，修 R2）：`TRANSIENT`/`NETWORK` 先在原渠道退避重试，预算 `k`（默认 2）。429 强制 honor `Retry-After`。非幂等生成（图片/视频）的"可能已送达"网络错误**不重试**，避免重复计费。
- **退避**（修 R1）：`backoff(n) = min(cap, base·2ⁿ)·(1 + jitter)`，`base=120ms`、`cap=2s`。
- **failover**（换渠道）：`CHANNEL_FATAL` 立即换；原地预算耗尽才换。换渠道时**不清亲和**，只让被绑渠道健康降权——亲和绑定靠 §3.3 `keepThreshold` 守卫自然失效（修 A3）。
- **预算分离**：`k`（原地）与 `m`（failover）独立计数，原地震荡不会耗光换渠道的机会。

### 7.3 Scheduler 接口（失败决策）

```go
type Scheduler interface {
    // 初次选号
    Select(ctx context.Context, req *SelectRequest) (*Decision, error)
    // 失败后决策：原地重试 / 换渠道 / 终止。同渠道则返回 sameChannel=true
    Next(ctx context.Context, req *SelectRequest, prev *Outcome) (*Decision, error)
    // 异步回报结果，更新健康/亲和/负载
    Observe(ctx context.Context, channelID int64, o Outcome)
}

type Outcome struct {
    Success   bool
    ErrClass  ErrClass
    Err       error
    LatencyMs float64
    RetryAfterMs int64   // 429 的 Retry-After
}
```

Executor（handler）的职责收敛为：`Select → HTTP → Observe`；失败时 `Next` 决定下一步，handler 只执行返回的决策（同渠道重试或 exclude 后重新 Select）。

---

## 8. 成本感知（决策 2）

```go
type CostCatalog interface {
    // 返回渠道对该模型的上游单价（USD per 1M token 或统一折算），decimal 精确
    UnitCost(ctx context.Context, channelID int64, model string) (decimal.Decimal, error)
}
```

`costFactor(c, r)` 在 Selector 内归一化：同模型所有合格渠道按单价升序，最便宜的 `costFactor=1.0`，其余按 `cheapest/this_cost` 递减（夹到 `(0,1]`）。

- 运营在 `chn_channels` 维护每渠道实际上游单价（可批量导入）。
- 未填单价的渠道 `costFactor=1.0`（不参与成本优化，向后兼容）。
- 单价属于计费层（USD），与 [`CLAUDE.md`](../../CLAUDE.md) 三层币种规则一致。

---

## 9. 端口与模块边界

### 9.1 端口汇总

| 端口 | 职责 | adapter | 存储 |
|---|---|---|---|
| `ChannelCatalog` | 合格候选渠道集（带内存缓存 + 失效） | `PostgresChannelCatalog` | PG + 进程内缓存 |
| `HealthStore` | 健康分 + 熔断器状态 | `RedisHealthStore` | Redis hash + Lua |
| `LoadGauge` | 并发占用 + softLimit | `RedisLoadGauge` | Redis ZSET |
| `AffinityStore` | 会话绑定 + 失效 | `RedisAffinityStore` | Redis |
| `CostCatalog` | 渠道单价 | `PostgresCostCatalog` | PG + 缓存 |

### 9.2 ChannelCatalog 缓存（修 P1）

`PostgresChannelCatalog` 维护进程内候选集缓存：
- Key：`(model, tenantChannelScope)`；TTL 默认 10s。
- 失效：渠道/能力/状态变更时主动 invalidate（接入现有 admin 写操作后的失效钩子）。
- 消除现状"单请求最多 4 次 DB 查询"——重试循环复用缓存，不再每次重查。

### 9.3 Selector 主流程

```go
func (s *Scheduler) Select(ctx, req) (*Decision, error) {
    cands, _ := s.catalog.Eligible(ctx, req.Model, req.Scope)   // 缓存命中零 DB
    cands = filterExcluded(cands, req.Exclude)

    // 亲和绑定守卫
    if ch, ok := s.affinity.Get(ns, req.SessionToken); ok {
        if w := s.effectiveWeight(ctx, ch, req); w >= s.policy.KeepThreshold {
            return decide(ch, "affinity-hit"), nil
        }
    }

    // 加权 HRW
    pick := weightedRendezvous(cands, req.SessionToken, func(c) float64 {
        return s.effectiveWeight(ctx, c, req)
    })
    s.affinity.Bind(ns, req.SessionToken, pick.ID, s.policy.AffinityTTL)
    return decide(pick, reason(pick)), nil
}
```

`effectiveWeight` 即 §3.2 公式，读 HealthStore + LoadGauge + CostCatalog 计算。

---

## 10. 数据模型与迁移

### 10.1 `chn_channels` 新增字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `cost_tier` | `NUMERIC(20,10)` NULL | 渠道成本档位/单价（USD），供 costFactor；NULL = 不参与成本优化 |
| `gray_release` | `SMALLINT` DEFAULT 100 | 灰度比例 0–100，驱动 `rampFactor`（0=禁用，100=全量） |
| `soft_limit_override` | `INT` NULL | 手动 softLimit 覆盖；NULL = 自动推断 |

> 现有 `priority`、`weight`、`max_concurrency`、`status` 保留。`max_concurrency` 语义改为"硬上限"（slot 租约仍用它做硬熔断），`soft_limit_override` 作为溢出用的软上限。

### 10.2 `chn_health_scores` 降级

- 字段不变，但**写入方改为后台快照任务**（从 Redis 落盘），路由不再读写。
- 新增 `snapshot_at TIMESTAMPTZ` 标记快照时间。

### 10.3 `routing_policy` 配置

存于 `sys_options`（key = `routing_policy`，JSON）。全局默认 + 租户覆盖（`sys_options` 的 tenant 维度，或 `mdl_tenant_models` 扩展字段，二选一在实现期定）。热加载（监听变更或短 TTL 缓存）。

```yaml
routing_policy:
  affinity_scope: explicit        # 仅显式信号（决策 3）
  affinity_ttl_seconds: 1800
  keep_threshold_ratio: 0.3       # keepThreshold = ratio × baseWeight
  in_place_retry_budget: 2        # k
  failover_budget: 3              # m
  backoff: { base_ms: 120, cap_ms: 2000 }
  spillover: { safety: 0.9 }
  circuit_breaker:
    error_rate: 0.5
    consecutive: 5
    cooldown_ms: 30000
    probe_window_ms: 10000
  tier_bias: { primary: 1.0, secondary: 0.1, fallback: 0.01 }
  cost_aware: true                # 决策 2
```

> 替代现状散落的 `channel_affinity_enabled` / `channel_affinity_ttl_seconds` / `channel_scheduler_v2_enabled` / `channel_auto_disable_*` 等布尔开关。**单一路径、版本化、可回滚、热加载**（修 O1）。

### 10.4 迁移脚本

按现有 `migrations/` 六位序号递增（不在此固定编号）。预计 2 个迁移：
1. `chn_channels` 加字段（`cost_tier` / `gray_release` / `soft_limit_override`）。
2. `chn_health_scores` 加 `snapshot_at`；`sys_options` 写入默认 `routing_policy`。

迁移须可重复执行（`up/down` 幂等，不破坏数据）。

---

## 11. 可观测性

每个 `Decision` 输出得分分解，写入转发 trace（扩展现有 `common.ForwardingTrace`）：

```json
{
  "channel_id": 12,
  "reason": "affinity-hit | weighted | spillover | failover | probe",
  "scores": {
    "12": {"health": 0.92, "headroom": 0.71, "cost": 1.0, "tier": 1.0, "ramp": 1.0, "effective": 0.65},
    "13": {"health": 0.88, "headroom": 0.30, "cost": 0.7, "tier": 0.1, "ramp": 1.0, "effective": 0.018}
  },
  "circuit": {"12": "CLOSED", "13": "CLOSED"},
  "affinity": {"token_source": "header", "bound": 12, "honored": true},
  "hops": [{"attempt": 0, "channel": 12, "outcome": "TRANSIENT", "backoff_ms": 240}, ...]
}
```

熔断状态变化（CLOSED↔OPEN↔HALF_OPEN）发结构化日志 + 可选告警（接入现有通知引擎）。没有这个，权重/阈值全是黑盒，无法调参。

---

## 12. 降级与容错

| 故障 | 降级行为 |
|---|---|
| Redis 不可用（健康/负载读） | 用进程内 last-known 快照（最近一次成功的 HealthStore/LoadGauge 结果，短 TTL）；快照缺失则 `health=1, headroom=1`（退化为基础权重） |
| Redis 不可用（亲和） | 跳过亲和守卫，直接走加权 HRW（seed 仍由会话令牌决定，粘性大体保留） |
| Redis 不可用（熔断） | 退化为"无熔断"，靠连续失败由 failover 预算兜底；不允许把已熔断渠道当健康放行 |
| `ChannelCatalog` 缓存 miss 且 DB 慢 | 复用 last-known 候选集；全空则返回 `ErrChannelUnavailable` |
| softLimit 自动推断缺失 | `capacityHeadroom=1`（不溢出），等同现状的优先级行为，安全降级 |

---

## 13. 分阶段实施计划

每阶段独立可上线、可回滚。建议在 feature flag（routing_policy 内的 `scheduler_engine: v2`）后灰度，验证完毕移除。

| 阶段 | 内容 | 解决的问题 | 行为变化 |
|---|---|---|---|
| **P1 解耦 + 端口地基** | 定义 §9 端口接口；把现有 health/affinity/capacity 逻辑搬到 adapter 后；Scheduler 纯逻辑骨架 + fake store 单测；`ChannelCatalog` 内存缓存 | P1、（为后续铺路） | **无**（纯重构） |
| **P2 Redis 化健康 + 熔断器** | `RedisHealthStore` + Lua OBSERVE/PROBE_ACQUIRE；熔断 FSM；DB 降级快照 | H1、H2 | 有（需灰度观察健康分） |
| **P3 重试 FSM + 退避 + 亲和守卫** | 错误分类器 + RetryFSM；原地/failover 预算；退避；`keepThreshold` 守卫替代无条件清亲和 | A3、R1、R2 | 有 |
| **P4 加权 HRW + 溢出 + 成本 + 灰度** | Selector 上复合权重 + tierBias + capacityHeadroom + costFactor + rampFactor；softLimit 自动推断 | A1、A2、L2 | 有（路由分布变化，需观察） |
| **P5 收尾** | 会话令牌解析（header/previous_response_id/conversation_id）；routing_policy 热加载；删除 legacy 路径与 `channel_scheduler_v2_enabled` | L3、O1 | 无 |

> P1 是纯重构、零行为变化、风险最低，建议作为第一个可评审/可合入的交付。完成 P1 后，P2–P5 都有可测地基。

---

## 14. 与现状问题的对应关系

| 现状问题 | 严重度 | 新设计如何解决 |
|---|---|---|
| **A1** 亲和命中绕过健康度 | 🔴 | 粘性是 HRW 涌现属性，病渠道权重低自动绕开（§3.1） |
| **A2** 亲和仅在最高优先级组内 | 🟡 | 选择在全候选池做，亲和跨优先级生效（§3、§9.3） |
| **A3** 任何可重试错误清亲和 | 🔴 | 失败不清亲和，靠 `keepThreshold` 守卫自然失效（§3.3、§7.2） |
| **R1** 重试无退避 | 🟡 | 指数退避 + jitter + honor Retry-After（§7.2） |
| **R2** 瞬时错误直接换渠道 | 🟡 | 两级重试：TRANSIENT/NETWORK 先原地（§7.2） |
| **H1** 健康度读改写竞态 | 🔴 | Lua 原子 EWMA，结构上无竞态（§5.4） |
| **H2** 失败路径同步写健康 | 🟡 | `Observe` 异步，EWMA 在 Redis 原子完成（§5、§7.3） |
| **P1** 选号无缓存每次查 DB | 🟡 | `ChannelCatalog` 内存缓存 + 失效（§9.2） |
| **O1** 双路径/flag 脚枪 | 🔴 | 单一路径 + 版本化 routing_policy（§10.3、P5） |
| **L2** 无优先级溢出 | ⚪ | capacityHeadroom 软降权实现优雅溢出（§6.3） |
| **L3** 亲和无会话维度 | ⚪ | 会话令牌（显式信号）+ 身份级回退（§4） |

---

## 15. 开放问题（实现期再定）

1. **routing_policy 租户覆盖存储位置**：`sys_options` tenant 维度 vs `mdl_tenant_models` 扩展字段——实现 P5 时定。
2. **对话指纹的二次评估**：决策 3 暂不做。若上线后发现 Claude Code 用户因身份级粘性出现渠道倾斜，可在此文档增补指纹方案（hash(system + 首条 user)）作为可配置开关。
3. **softLimit 自动推断的收敛速度与安全系数**：需用真实 429 数据标定 `safety` 默认值。
4. **灰度 flag 的移除时机**：P2–P4 各自灰度验证通过后再推进下一阶段，全量稳定后 P5 移除 flag。

---

## 参考

- 现状问题清单：[`channel-scheduling-issues.md`](./channel-scheduling-issues.md)
- 现状对比分析：[`channel-scheduling-comparison.md`](./channel-scheduling-comparison.md)
- 决策记忆：`channel-scheduling-redesign-decisions`（项目 memory）
