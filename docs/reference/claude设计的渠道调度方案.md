# 渠道调度模块重新设计方案（claude 设计）

> 状态：待评审。撰写时间：2026-07-30。
> 本方案为 clean-slate 重新设计，仅以问题清单（[`当前渠道调度存在的问题.md`](./当前渠道调度存在的问题.md)）与业务需求为输入，不受现有实现结构约束。
> 目标读者：实现者。本文档包含可直接落地的模块结构、Go 接口、Redis key 与 Lua 脚本设计、数据库迁移、配置 Schema 与分阶段实施计划。

---

## 目录

1. [设计目标](#1-设计目标)
2. [核心思想：统一评分模型 + 加权一致性哈希](#2-核心思想统一评分模型--加权一致性哈希)
3. [会话键解析（会话级亲和）](#3-会话键解析会话级亲和)
4. [权重合成函数](#4-权重合成函数)
5. [选择算法与绑定守卫](#5-选择算法与绑定守卫)
6. [错误分类与双预算重试](#6-错误分类与双预算重试)
7. [熔断器](#7-熔断器)
8. [容量与溢出](#8-容量与溢出)
9. [模块结构与 Go 接口](#9-模块结构与-go-接口)
10. [Redis 状态设计（key 布局 + Lua 脚本）](#10-redis-状态设计key-布局--lua-脚本)
11. [数据库变更](#11-数据库变更)
12. [路由策略配置（routing policy）](#12-路由策略配置routing-policy)
13. [多副本与降级](#13-多副本与降级)
14. [可观测性](#14-可观测性)
15. [请求流全景](#15-请求流全景)
16. [分阶段实施计划](#16-分阶段实施计划)
17. [测试策略](#17-测试策略)
18. [问题覆盖对照表](#18-问题覆盖对照表)
19. [风险与待验证项](#19-风险与待验证项)

---

## 1. 设计目标

从问题清单与需求反推出的五条硬目标：

1. **粘性（亲和）、健康度、层级、负载四者不互相打架。** 现状的核心病灶是四套独立机制串行叠加、互相短路（亲和绕过健康 A1、层级杀死亲和 A2、失败清亲和 A3）。
2. **高层级渠道饱和时流量自动溢出到备用渠道**（L2），备用渠道不再是永远闲置的冷备。
3. **失败处理分类化**：瞬时错误（502/504/连接抖动）原地重试，致命错误（401/404/欠费）立即切渠道并熔断，不再一刀切（R1/R2）。
4. **核心逻辑纯函数化**：调度决策可脱离 DB/Redis 单测；单一代码路径，无 legacy/v2 双轨（O1）。
5. **多副本部署安全**：无分布式锁、无选主、无读-改-写竞态（H1）；调度热路径不查数据库（P1、H2）。

### 非目标

- 不做对话内容指纹（对 body 哈希推断会话）：热路径成本高、误聚合风险大，只认显式会话信号。
- 不做跨请求的全局最优分配（如最少连接、全局队列）：有状态算法在多副本下需要协调，收益不抵复杂度。
- 不引入合成探测请求作为常规探活手段（管理后台手动"测试渠道"保留，与调度无关）。

---

## 2. 核心思想：统一评分模型 + 加权一致性哈希

不再让「优先级分组 → 亲和覆盖 → 健康过滤 → 权重随机」串行执行（每层都可能推翻上一层语义），而是把所有信号收敛为**单一的有效权重函数** `W(c)`，再用**加权一致性哈希（Weighted Rendezvous Hashing，下称 HRW）**做选择：

```
W(c) = baseWeight(c)      // 运营配置的组内权重（沿用 chn_channels.weight）
     × tierFactor(c)      // 层级偏置：primary=1.0，secondary=0.15，reserve=0.02（可配）
     × healthFactor(c)    // 健康因子 ∈ (0,1]，连续函数，非硬阈值
     × headroom(c)^γ      // 负载余量 (1 - inflight/softLimit)^γ，饱和时趋近 0
     × costFactor(c)      // 成本因子：同模型更便宜的上游微幅占优
     × rampFactor(c)      // 爬坡因子：新渠道 / 熔断刚恢复的渠道从小流量爬坡

选择：pick = argmax_c [ -W(c) / ln(u_c) ]，u_c = hash(sessionKey, channelID) → (0,1)
```

这个结构一次性解决三组矛盾：

| 矛盾 | 如何消解 |
|---|---|
| 亲和 vs 健康（A1） | 粘性是 HRW 对相同 sessionKey 的**涌现属性**，不是硬覆盖。渠道健康下降 → healthFactor 缩小 → W 缩小 → HRW 自然绕开。不存在"命中亲和就跳过一切检查"的短路路径。 |
| 层级 vs 溢出（L2） | 溢出是**连续过程**：primary 饱和 → headroom 下降 → W 坍缩 → 新会话自然落到 secondary。无阈值切换、无抖动边界。 |
| 多副本一致性 | HRW 是纯确定性计算：任何副本对相同 sessionKey + 相同候选快照得出相同结果，无需锁、无需选主。 |

---

## 3. 会话键解析（会话级亲和）

### 3.1 解析链（按序取第一个命中）

```
1. 显式头：X-Session-Id
   —— 写进平台接入文档，鼓励客户端主动携带；平台自有 SDK 默认携带。

2. 协议内信号（按入站格式）：
   a) Anthropic 格式（/v1/messages）：
      metadata.user_id —— Claude Code 每次请求都携带，格式形如
      user_<hash>_account_<uuid>_session_<uuid>
      用正则提取 session 段：`session_([0-9a-fA-F-]{36})`
      提取失败（格式不符）则整个 user_id 作为会话键（仍优于身份级）。
      ⚠️ 实现前需抓包验证格式稳定性（见 §19 风险）。
   b) OpenAI Responses（/v1/responses）：previous_response_id、conversation_id。
   c) OpenAI Chat Completions：无会话信号（`user` 字段是身份不是会话），落到第 3 层。
   d) Gemini：无会话信号，落到第 3 层。

3. 身份级回退：tenant:user:apiKey:model 四元组。
```

会话键最终形态：`sk:<来源标记>:<原始值哈希>`，来源标记 ∈ {hdr, anthropic, openai, ident}，用于观测各来源占比。

### 3.2 为什么"一个 key 多会话集中到一个渠道"不需要在身份级修

亲和的目的是命中上游 KV / prompt 缓存，缓存以会话为单位，所以**会话级键天然优于身份级键**：同一用户的会话 A / 会话 B 各自独立做 HRW，散布到不同渠道，同时各自的多轮对话仍稳定粘在各自渠道——缓存收益与负载分散兼得。

回退到身份级时，一个用户的所有会话确实集中到一个渠道，但这不是 bug：单用户负载相对渠道容量很小，负载均衡靠"大量用户被 HRW 散开"实现。极端情况（单身份流量压垮渠道）由 headroom 因子兜底：渠道趋饱和 → 权重坍缩 → 新绑定自动迁走。

---

## 4. 权重合成函数

全部为纯函数，输入是候选快照中的渠道状态字段，输出 float64。

### 4.1 tierFactor

```
tier ∈ {primary, secondary, reserve}（chn_channels.tier，见 §11）
tierFactor = policy.tierFactors[tier]     // 默认 {1.0, 0.15, 0.02}
```

- secondary 在正常态接到约 1/8 以下的零星流量，这点流量同时完成**天然保温探活**——无需合成探测即可持续验证备用渠道可用性。
- 运营想要纯冷备时，把 secondary/reserve 的 tierFactor 配为 0；此时仅在上层全熔断时由硬性扩组规则启用（§8.3）。

### 4.2 healthFactor

```
healthFactor = clamp(succEwma, 0.01, 1.0)^α × latencyPenalty
α 默认 2（succEwma=0.9 → 0.81；succEwma=0.5 → 0.25）
latencyPenalty = clamp(latRef / max(latEwma, latRef), 0.5, 1.0)
latRef 默认 3000ms（延迟不超过 3s 不惩罚，超过按比例最多减半）
```

succEwma / latEwma 来自 Redis 原子 EWMA（§10.2），读取的是候选快照里的最近值（快照刷新间隔内的滞后可接受）。

### 4.3 headroom

```
headroom = max(0, 1 - inflight / softLimit)
headroomFactor = headroom^γ        // γ 默认 2，饱和时权重加速坍缩
softLimit：手动 max_concurrency 优先；未配置用 429 起始点自动估计（§8.2）
inflight 为 0 或无容量信息时 headroomFactor = 1
```

### 4.4 costFactor

```
costRatio = chn_abilities.cost_ratio   // 上游实际价 / 平台基准价，默认 1.0
costFactor = clamp((1 / costRatio)^β, 0.5, 2.0)    // β 默认 0.5
```

同模型等价渠道中更便宜的上游获得温和优势（β=0.5 使 8 折渠道获得约 1.12 倍权重），不会因价格差碾压健康度信号。cost_ratio 挂在 chn_abilities（渠道×模型粒度），支持批量导入。

> 币种说明：cost_ratio 是**无量纲比例**，不是金额，Go 层用 float64，不适用 decimal 强约束。

### 4.5 rampFactor

```
新启用渠道 / 熔断恢复（OPEN→CLOSED）后 rampWindow（默认 120s）内：
rampFactor = clamp(elapsed / rampWindow, 0.05, 1.0)
其余时间恒为 1.0
```

避免恢复瞬间被 HRW 一次性灌满流量、立刻再次打挂。

---

## 5. 选择算法与绑定守卫

### 5.1 HRW 选择（纯函数）

```go
// relay/dispatch/hrw.go
func PickHRW(candidates []ScoredChannel, sessionKey string) *ScoredChannel {
    best, bestScore := (*ScoredChannel)(nil), math.Inf(1)
    for i := range candidates {
        c := &candidates[i]
        if c.Weight <= 0 { continue }
        h := sha256.Sum256([]byte(sessionKey + "\x00" + strconv.FormatInt(c.ID, 10)))
        raw := binary.BigEndian.Uint64(h[:8])
        u := (float64(raw>>11) + 1) / (float64(uint64(1)<<53) + 1)   // (0,1)，53 位精度
        score := -math.Log(u) / c.Weight
        if score < bestScore || (score == bestScore && c.ID < best.ID) {
            best, bestScore = c, score
        }
    }
    return best
}
```

### 5.2 绑定守卫（防抖层）

纯 HRW 的弱点：W(c) 随负载连续变化，临界状态下同一会话可能在两个渠道间反复横跳，每跳一次损失一次上游缓存。因此在 HRW 之上加**绑定守卫**：

```
首次选择    → 写入绑定 sessionKey → channelID（Redis，TTL = policy.bindTTL 默认 30min，成功请求滑动续期）
后续请求    → 读绑定 → 检查被绑渠道当前是否「合格」：
    合格 = 熔断器非 OPEN
        且 healthFactor ≥ policy.keepHealthMin   （默认 0.5）
        且 headroom     ≥ policy.keepHeadroomMin （默认 0.1）
        且 渠道仍在本请求的候选集内（未被本请求排除、未被禁用）
    合格   → 直接复用（不重跑 HRW，绝对稳定）
    不合格 → 重跑 HRW，CAS 重绑
```

与现状的本质区别（修复 A2 / A3）：

- 绑定失效判据是**被绑渠道自身状态**（病了 / 满了 / 熔断了），不是"是否在最高优先级组"（A2 消除——tier 只是权重因子，不存在组边界）。
- **单次请求失败不删除绑定**：failover 只把渠道加入本请求的排除列表，绑定留存，下一个请求重新做合格性检查（A3 消除）。绑定的自然消亡途径只有两条：TTL 过期、合格性检查失败触发重绑。
- 渠道被禁用（手动或熔断长期 OPEN）时通过反向索引批量清理绑定（§10.3），与现状 `InvalidateChannelAffinities` 语义一致。

---

## 6. 错误分类与双预算重试

### 6.1 错误分类器（纯函数）

```go
// relay/dispatch/classify.go
type ErrorClass int
const (
    ErrClassClient        ErrorClass = iota // 客户端错误，换渠道无意义
    ErrClassTransient                       // 瞬时错误，原地重试
    ErrClassRateLimit                       // 限流
    ErrClassChannelFatal                    // 渠道致命错误
    ErrClassTimeout                         // 超时
    ErrClassPartialStream                   // 流已写出后中断，不可重试
)

func Classify(statusCode int, err error, streamStarted bool, retryAfter time.Duration) ErrorClass
```

| 类别 | 判定 | 处理 |
|---|---|---|
| `CLIENT` | 400 参数错、413 过大、422、内容策略拒绝 | **不重试**，原样返回（换渠道结果相同） |
| `TRANSIENT` | **502、504**、520–527、请求未发出时连接被重置 | **原地重试**：同渠道，退避 100ms→300ms + jitter，预算 k |
| `RATE_LIMIT` | 429 | Retry-After ≤ policy.rateLimitWaitMax（默认 2s）→ 原地等待重试一次；否则立即 failover。429 事件喂给 softLimit 估计器（§8.2） |
| `CHANNEL_FATAL` | 401/403（上游 key 失效）、404（模型不存在）、上游余额耗尽 | **零原地重试**，立即 failover + 熔断计数直接达阈值 + 运营告警 |
| `TIMEOUT` | 连接超时 / 首 token 超时 | 立即 failover（等下去大概率仍超时） |
| `PARTIAL_STREAM` | SSE 已开始写出后上游中断 | **不可重试**（客户端响应已污染），记失败，结束 |

> 502/504 是最应该原地重试的错误：通常是上游网关/LB 瞬时抖动，短暂退避后大概率恢复；换渠道反而付出丢上游缓存的代价。而 401/404/欠费换一百次也不会好，必须直接熔断。

### 6.2 双预算结构

```
原地预算 k = policy.inPlaceBudget    默认 2（每渠道独立计）
failover 预算 m = policy.failoverBudget 默认 2（最多再尝试 2 个不同渠道）
总时限 = policy.totalDeadline        默认 30s（流式请求首 token 前生效）
```

三个预算独立扣减：

- TRANSIENT 原地重试成功只消耗原地预算，不动 failover 预算。
- CHANNEL_FATAL 跳过原地预算，直接扣 failover 预算。
- 容量租约获取失败（渠道满）**不扣任何预算**，只把渠道加入本请求排除列表重新选择。
- 退避曲线：`min(100ms × 2^attempt, 1s) + jitter(0~50%)`；有 Retry-After 时优先遵从。

### 6.3 重试决策 FSM（纯函数）

```go
// relay/dispatch/policy.go
type RetryDecision int
const (
    DecisionInPlaceRetry RetryDecision = iota // 同渠道退避重试
    DecisionFailover                          // 排除当前渠道重新选择
    DecisionAbort                             // 终止，返回错误
)

type AttemptState struct {
    InPlaceUsed  int           // 当前渠道已用原地次数
    FailoverUsed int           // 已切换渠道次数
    Elapsed      time.Duration // 已耗时
    StreamStarted bool
}

func Decide(cls ErrorClass, s AttemptState, p RetryPolicy) (RetryDecision, time.Duration /*backoff*/)
```

---

## 7. 熔断器

粒度：**渠道级** + **渠道×模型级**两层。渠道级管连接性/认证类故障（影响整个渠道），渠道×模型级管单模型故障（如某模型上游已下线而其它模型正常）。

```
状态机：CLOSED → OPEN → HALF_OPEN → CLOSED
                  ↑________________|（探测失败回 OPEN）

CLOSED → OPEN：滑动窗口（默认 60s）内失败 ≥ failThreshold（默认 8）
              或 CHANNEL_FATAL 一次直达。
OPEN 持续 cooldown（默认 30s，指数递增至 5min 上限）后进入 HALF_OPEN。
HALF_OPEN：Redis 原子令牌，每探测窗口（默认 10s）只放行 1 个真实请求：
    成功 → CLOSED（rampFactor 开始爬坡）
    失败 → 回 OPEN，cooldown 翻倍。
```

- **自然探活**：HALF_OPEN 放行的是真实业务请求，不发合成请求。多副本下由 Redis 原子令牌保证每窗口全局只放行一个（§10.4）。
- OPEN 渠道从候选集硬排除（这是权重体系之外唯一的硬规则之一）。
- 状态转移逻辑（纯函数）在 `relay/dispatch/breaker.go`，计数与令牌存取通过 StatePort。
- 渠道级 OPEN 持续超过 policy.autoDisableAfter（默认 10min）→ 落库置 `chn_channels.status='disabled', auto_disabled=1` + 清理该渠道全部绑定 + 通知，替代现状的 consecutive_failures 自动禁用。

---

## 8. 容量与溢出

### 8.1 inflight 租约

沿用 ZSET 租约模式（多副本安全、实例崩溃自愈）：

```
member = requestID，score = 过期时间戳（毫秒）
获取：Lua 原子【清过期 → ZCARD < softLimit ? ZADD : 拒绝】
长流式请求每 30s 续租；结束 ZREM 释放；实例崩溃后租约到期自动消失。
Redis 故障 → fail-open（放行）。
```

### 8.2 softLimit 双来源

```
1. 手动：chn_channels.max_concurrency > 0 时直接使用。
2. 自动估计（未配置时）：记录每次收到 429 时的 inflight 水位，
   onset_ewma = onset_ewma×0.8 + 当前水位×0.2（Lua 原子），
   softLimit = max(4, floor(onset_ewma × 0.9))。
   无 429 历史时视为无限容量（headroomFactor = 1）。
```

### 8.3 溢出行为（三层递进）

1. **常态软溢出**：primary headroom 下降 → headroom^γ 坍缩 → W_primary 低于 W_secondary → 新会话自然落到 secondary。primary 上的既有绑定只要仍"合格"（headroom ≥ 0.1）继续留存——**老会话不迁移，保上游缓存；新会话去分流**。
2. **租约硬拒绝**：单渠道 inflight 触顶时租约获取失败 → 该渠道本请求内排除、不扣预算、立刻重选。
3. **硬性扩组兜底**：若某 tier 的全部渠道均被熔断排除或 tierFactor 配置为 0，候选集自动扩到下一 tier（primary → secondary → reserve）。此规则独立于权重计算，保证参数误配时可用性不丢。

---

## 9. 模块结构与 Go 接口

### 9.1 目录结构

```
relay/dispatch/                  ← 新模块。纯库：不 import gf/gdb/dao/redis
    types.go        Channel、Tier、RequestProfile、Decision、Outcome、ScoredChannel
    session.go      会话键解析链（输入 headers + 已解析的 body 字段，纯函数）
    classify.go     错误分类器（纯函数）
    score.go        W(c) 权重合成（纯函数）
    hrw.go          加权 rendezvous 选择（纯函数）
    policy.go       重试决策 FSM + RetryPolicy/RoutingPolicy 结构体定义
    breaker.go      熔断状态机（转移逻辑纯函数，存取走端口）
    coordinator.go  编排：解析会话键 → 快照 → 绑定守卫 → HRW → 租约 → 结果回写
    ports.go        端口接口（见 9.2）

internal/logic/dispatchadapter/  ← 适配层，全部 I/O 在这
    catalog.go      DAO 三表 JOIN → 内存快照，定时刷新 + pub/sub 失效
    state_redis.go  StatePort 的 Redis 实现（全部 Lua 原子脚本）
    state_local.go  Redis 故障时的本地降级镜像
    config.go       routing policy 加载 + 热更新
    wire.go         组装 coordinator 供 handler 使用
```

依赖方向：`relay/handler → relay/dispatch ← internal/logic/dispatchadapter`。核心不知道适配层存在。

### 9.2 端口接口

```go
// relay/dispatch/ports.go
package dispatch

// CatalogPort 渠道目录：返回某模型的候选渠道快照（含最近的健康/负载读值）。
// 实现方保证 O(1) 内存读取，禁止在调用路径中查库。
type CatalogPort interface {
    Snapshot(ctx context.Context, tenantID int64, model string, scope []int64) []Channel
}

// StatePort 运行时状态：绑定 / 健康 / 熔断 / 容量。实现方保证多副本原子性。
type StatePort interface {
    // 绑定
    GetBinding(ctx context.Context, sessionKey string) (channelID int64, ok bool)
    SetBinding(ctx context.Context, sessionKey string, channelID int64, ttl time.Duration)
    TouchBinding(ctx context.Context, sessionKey string, ttl time.Duration)
    InvalidateChannelBindings(ctx context.Context, channelID int64)

    // 健康（fire-and-forget，实现方内部异步化）
    ReportOutcome(o Outcome) // {ChannelID, Model, Success, LatencyMs, ErrClass}

    // 熔断
    BreakerState(ctx context.Context, channelID int64, model string) BreakerState
    TryProbeToken(ctx context.Context, channelID int64) bool // HALF_OPEN 探测令牌

    // 容量
    AcquireLease(ctx context.Context, channelID int64, softLimit int, requestID string) bool
    RefreshLease(ctx context.Context, channelID int64, requestID string)
    ReleaseLease(ctx context.Context, channelID int64, requestID string)
}

// Clock / Entropy 便于测试注入
type Clock interface{ Now() time.Time }
```

### 9.3 协调器对外 API（handler 唯一入口）

```go
type Coordinator struct { /* catalog CatalogPort; state StatePort; policy atomic.Pointer[RoutingPolicy]; clock Clock */ }

// Route 开启一次请求的调度会话。
func (co *Coordinator) Route(ctx context.Context, req RequestProfile) *RouteSession

type RouteSession struct { /* 内部持有排除列表、预算状态、当前选择 */ }

// Next 返回下一个应尝试的渠道（首次调用 = 初始选择）。返回 nil 表示无渠道可用。
func (s *RouteSession) Next(ctx context.Context) *Decision

// Report 上报本次尝试结果，返回重试决策（原地/换渠道/终止）与退避时长。
func (s *RouteSession) Report(ctx context.Context, statusCode int, err error, streamStarted bool,
    latencyMs float64, retryAfter time.Duration) (RetryDecision, time.Duration)

// Finish 请求结束（成功或最终失败）：释放租约、续期或落定绑定、上报健康。
func (s *RouteSession) Finish(ctx context.Context, success bool)
```

handler 侧的重试循环收敛为：

```go
sess := coordinator.Route(ctx, profile)
for {
    d := sess.Next(ctx)
    if d == nil { return errAllChannelsFailed }
    err := forward(d)                       // 转发（含渠道 Key 获取）
    if err == nil { sess.Finish(ctx, true); return nil }
    decision, backoff := sess.Report(ctx, code(err), err, streamStarted(err), lat, retryAfter(err))
    switch decision {
    case dispatch.DecisionInPlaceRetry: sleep(backoff) // 同一个 d 再 forward
    case dispatch.DecisionFailover:     continue        // 下轮 Next 换渠道
    case dispatch.DecisionAbort:        sess.Finish(ctx, false); return err
    }
}
```

---

## 10. Redis 状态设计（key 布局 + Lua 脚本）

统一前缀 `dispatch:v1:`。所有多步操作一律单条 Lua 原子执行。

### 10.1 key 布局

| key | 类型 | 内容 | TTL |
|---|---|---|---|
| `dispatch:v1:bind:<sha256(sessionKey)>` | STRING | channelID | bindTTL（默认 30min，成功滑动续期） |
| `dispatch:v1:bind:rev:<channelID>` | SET | 指向该渠道的绑定 key 集合 | bindTTL + 60s |
| `dispatch:v1:health:<channelID>:<model>` | HASH | succ_ewma, lat_ewma, fail_window_count, window_start_ms | 24h 滑动 |
| `dispatch:v1:breaker:<channelID>` | HASH | state(0/1/2), opened_at_ms, cooldown_ms, probe_window_start_ms | 24h |
| `dispatch:v1:breaker:<channelID>:<model>` | HASH | 同上（模型级） | 24h |
| `dispatch:v1:inflight:<channelID>` | ZSET | member=requestID, score=过期时间戳 ms | 租约期 + 60s |
| `dispatch:v1:limit429:<channelID>` | HASH | onset_ewma | 24h |
| `dispatch:v1:catalog:ver` | STRING | 目录版本号（配置变更时 INCR） | 无 |

pub/sub 频道：`dispatch:v1:catalog:invalidate`（负载为变更的 channelID 或 `*`）。

### 10.2 Lua：健康 EWMA 原子更新（消除 H1 竞态）

```lua
-- KEYS[1] = health key; ARGV = success(0/1), latencyMs, nowMs, windowMs
local succ = tonumber(redis.call('HGET', KEYS[1], 'succ_ewma') or '1.0')
local lat  = tonumber(redis.call('HGET', KEYS[1], 'lat_ewma') or '0')
if ARGV[1] == '1' then
  succ = succ * 0.9 + 0.1
  lat  = (lat == 0) and tonumber(ARGV[2]) or (lat * 0.7 + tonumber(ARGV[2]) * 0.3)
else
  succ = succ * 0.9
  -- 失败窗口计数（供熔断判定）
  local ws = tonumber(redis.call('HGET', KEYS[1], 'window_start_ms') or '0')
  if tonumber(ARGV[3]) - ws > tonumber(ARGV[4]) then
    redis.call('HSET', KEYS[1], 'window_start_ms', ARGV[3], 'fail_window_count', 1)
  else
    redis.call('HINCRBY', KEYS[1], 'fail_window_count', 1)
  end
end
redis.call('HSET', KEYS[1], 'succ_ewma', succ, 'lat_ewma', lat)
redis.call('PEXPIRE', KEYS[1], 86400000)
return redis.call('HGET', KEYS[1], 'fail_window_count') or '0'
```

调用方（state_redis.go 的 `ReportOutcome`）**fire-and-forget**：投入有界 channel 由后台 worker 批量执行，失败路径零 DB/Redis 往返阻塞（消除 H2）。返回的窗口失败数就地用于熔断转移判定，一次往返完成两件事。

### 10.3 Lua：绑定 CAS 写入 + 反向索引

```lua
-- KEYS[1] = bind key; ARGV = channelID, ttlSec, revPrefix
local old = redis.call('GET', KEYS[1])
if old and old ~= ARGV[1] then
  redis.call('SREM', ARGV[3] .. old, KEYS[1])
end
redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
redis.call('SADD', ARGV[3] .. ARGV[1], KEYS[1])
redis.call('EXPIRE', ARGV[3] .. ARGV[1], tonumber(ARGV[2]) + 60)
return 1
```

渠道禁用/熔断长期 OPEN 时：`SMEMBERS rev → DEL 各绑定 → DEL rev`（单 Lua）。

### 10.4 Lua：HALF_OPEN 探测令牌（多副本每窗口全局放行 1 个）

```lua
-- KEYS[1] = breaker key; ARGV = nowMs, probeWindowMs
local ws = tonumber(redis.call('HGET', KEYS[1], 'probe_window_start_ms') or '0')
if tonumber(ARGV[1]) - ws >= tonumber(ARGV[2]) then
  redis.call('HSET', KEYS[1], 'probe_window_start_ms', ARGV[1])
  return 1  -- 放行本请求作为探测
end
return 0
```

### 10.5 Lua：inflight 租约获取

```lua
-- KEYS[1] = inflight key; ARGV = nowMs, expireAtMs, softLimit, requestID, keyTtlMs
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[3]) then return 0 end
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[4])
redis.call('PEXPIRE', KEYS[1], ARGV[5])
return 1
```

---

## 11. 数据库变更

迁移脚本：`migrations/000014_channel_dispatch_redesign.sql`

```sql
-- +goose Up

-- 1. 渠道层级：三档固定枚举，替代自由整数优先级的调度语义
--    （priority 列保留不删，仅停止参与调度，供回滚与历史查询）
ALTER TABLE chn_channels ADD COLUMN IF NOT EXISTS tier VARCHAR(16) NOT NULL DEFAULT 'primary';
COMMENT ON COLUMN chn_channels.tier IS '调度层级：primary=首选 secondary=备用 reserve=保底';

-- 按现有 priority 自动映射：全局最高优先级值 → primary，次高 → secondary，其余 → reserve
WITH ranked AS (
  SELECT id, DENSE_RANK() OVER (ORDER BY priority DESC) AS rk FROM chn_channels
)
UPDATE chn_channels c SET tier = CASE ranked.rk
    WHEN 1 THEN 'primary'
    WHEN 2 THEN 'secondary'
    ELSE 'reserve' END
FROM ranked WHERE c.id = ranked.id;

-- 2. 渠道×模型成本比例（无量纲，上游实际价/平台基准价）
ALTER TABLE chn_abilities ADD COLUMN IF NOT EXISTS cost_ratio NUMERIC(10,4) NOT NULL DEFAULT 1.0;
COMMENT ON COLUMN chn_abilities.cost_ratio IS '成本比例：该渠道该模型上游实际价/平台基准价，1.0=等价，0.8=八折，参与调度 costFactor 计算';

-- 3. 健康快照表：Redis 为真相源，DB 仅存周期快照供仪表盘
--    （chn_health_scores 保留为快照落点，不再被调度读取；
--     调度切换完成后由 cron 每 5min 把 Redis EWMA 值写入此表）

-- +goose Down
ALTER TABLE chn_abilities DROP COLUMN IF EXISTS cost_ratio;
ALTER TABLE chn_channels DROP COLUMN IF EXISTS tier;
```

执行后运行 `gf gen dao` 重新生成 entity/do/dao。

角色变化汇总：

| 表 | 现状角色 | 新角色 |
|---|---|---|
| `chn_channels.priority` | 调度分组依据 | 冻结（保留列，调度改读 tier） |
| `chn_channels.weight` | 组内权重 | baseWeight（沿用） |
| `chn_channels.max_concurrency` | 容量上限 | softLimit 手动来源（沿用） |
| `chn_health_scores` | 调度实时读取 | 仪表盘快照（cron 从 Redis 落盘） |
| `chn_channel_affinities`（若存在遗留表） | — | 废弃，绑定只在 Redis |

---

## 12. 路由策略配置（routing policy）

单一版本化 JSON 对象，存 `sys_options` key = `channel_routing_policy`，替代现有散落的布尔开关（`channel_scheduler_v2_enabled` / `channel_affinity_enabled` / `channel_capacity_enabled` / `channel_auto_disable_enabled` 全部退役）。支持全局默认 + 租户覆盖（`channel_routing_policy:tenant:<id>`，浅合并），配置中心热加载（沿用现有 Config 监听机制，加载后 `atomic.Pointer` 替换）。

```jsonc
{
  "version": 1,
  "tierFactors": { "primary": 1.0, "secondary": 0.15, "reserve": 0.02 },
  "health":  { "alpha": 2.0, "latRefMs": 3000 },
  "load":    { "gamma": 2.0, "leaseSeconds": 90 },
  "cost":    { "beta": 0.5, "min": 0.5, "max": 2.0 },
  "ramp":    { "windowSeconds": 120, "floor": 0.05 },
  "binding": {
    "ttlSeconds": 1800,
    "keepHealthMin": 0.5,
    "keepHeadroomMin": 0.1
  },
  "retry": {
    "inPlaceBudget": 2,
    "failoverBudget": 2,
    "totalDeadlineSeconds": 30,
    "backoffBaseMs": 100,
    "backoffMaxMs": 1000,
    "rateLimitWaitMaxMs": 2000
  },
  "breaker": {
    "windowSeconds": 60,
    "failThreshold": 8,
    "cooldownSeconds": 30,
    "cooldownMaxSeconds": 300,
    "probeWindowSeconds": 10,
    "autoDisableAfterSeconds": 600
  },
  "session": {
    "headerName": "X-Session-Id",
    "parseAnthropicMetadata": true,
    "parseOpenAIResponses": true
  }
}
```

校验：加载时做 Schema 校验（范围 + 类型），非法配置拒绝生效并告警，继续用上一份有效配置——杜绝 O1 式"误配置静默降级"。

---

## 13. 多副本与降级

| 状态 | 存放 | 多副本策略 |
|---|---|---|
| 渠道目录快照 | 各实例内存 | 各自定时刷（5s）+ pub/sub 失效；秒级不一致无害（HRW 输入短暂不同只影响极少数新绑定） |
| 健康 EWMA / 窗口失败数 | Redis | 单条 Lua 原子读-算-写，N 副本并发失败精确 +N（H1 消除） |
| inflight 租约 | Redis ZSET | 实例崩溃后租约自然过期，无需清理协调 |
| 会话绑定 | Redis | Lua CAS，两副本同时冷启动同一会话时先写者赢、后者跟随，收敛一致 |
| 熔断状态 / 探测令牌 | Redis | Lua 原子转移；HALF_OPEN 每窗口全局只放行 1 个 |
| 选择计算 | 无状态 | HRW 纯确定性，任何副本同输入同输出 |

**Redis 故障降级**（state_local.go）：

- 健康：用最后一次成功读取的快照（内存镜像，随目录快照一起持有）。
- 绑定：不可用 → 退化为纯 HRW。HRW 本身仍提供软粘性（相同 sessionKey 同选择），只是失去防抖层。
- 熔断：退化为实例本地熔断（进程内计数），保护仍在、只是不再全局协同。
- 容量：fail-open 放行。
- 结论：**Redis 完全不可用时调度链路仍可服务**，只损失精度不损失可用性。

---

## 14. 可观测性

### 14.1 决策日志（每次选择一条，DEBUG 级可关）

```
request_id, session_key_source(hdr/anthropic/openai/ident), binding(hit/rebind/new),
chosen_channel, tier, weight_breakdown{base,tier,health,headroom,cost,ramp},
candidates_count, excluded{breaker:n, lease:n, request:n}
```

### 14.2 重试/失败日志（WARN）

```
request_id, attempt_seq, channel, err_class, decision(inplace/failover/abort),
backoff_ms, budgets{inplace_left, failover_left}
```

### 14.3 指标（Prometheus 风格，供 ops 仪表盘）

- `dispatch_selection_total{channel,tier,reason=bind|hrw}`
- `dispatch_retry_total{channel,err_class,decision}`
- `dispatch_breaker_state{channel}`（gauge 0/1/2）
- `dispatch_binding_ratio`（绑定命中率）
- `dispatch_overflow_total{from_tier,to_tier}`（溢出计数——验证 §8.3 是否按预期工作）
- `dispatch_session_source_total{source}`（会话键来源占比——验证 Claude Code 解析有效性）

### 14.4 现有字段衔接

`bil_usage_logs.retry_index` 语义不变（最终成功尝试的序号）；`SelectionReason` 取值改为 `bind` / `hrw` / `overflow`，前端渠道日志展示同步调整。

---

## 15. 请求流全景

```
请求进入
 │
 ├─ 1. 解析会话键（显式头 → 协议信号 → 身份四元组）          [session.go 纯函数]
 ├─ 2. 取候选快照（内存；OPEN 熔断渠道硬排除；tier 硬扩组兜底）[catalog.go]
 ├─ 3. 读绑定 → 被绑渠道合格？ ──是──→ 选它
 │                └─否/无绑定──→ 算 W(c) → HRW → 选中 → CAS 写绑定
 ├─ 4. 取 inflight 租约（满 → 本请求排除该渠道 → 回 3，不扣预算）
 ├─ 5. 获取渠道 Key、转发
 │     ├─ 成功 → Finish：释放租约 / 绑定续期 / 异步上报成功 EWMA
 │     └─ 失败 → Report → 错误分类
 │           ├─ CLIENT / PARTIAL_STREAM → Abort
 │           ├─ TRANSIENT 且原地预算>0 → 退避 → 回 5（同渠道）
 │           ├─ 其余且 failover 预算>0 → 本请求排除该渠道 → 回 3
 │           └─ 预算/时限耗尽 → Abort
 │              （失败路径全程：异步上报失败 EWMA + 熔断计数；绑定不删除）
 └─ 结束
```

---

## 16. 分阶段实施计划

> 原则：每阶段独立可合入、可回滚；**不引入长期存活的新旧切换开关**——灰度用影子模式，切换是一次性的（阶段 4），之后立即删旧码（阶段 5）。

### 阶段 0：纯核心库（无任何接线，零风险）

**交付物**
- `relay/dispatch/` 全部纯逻辑文件：types / session / classify / score / hrw / policy / breaker / ports。
- 完整单元测试：
  - HRW 分布测试（10 万次采样，权重比例误差 < 2%）；
  - 会话键解析链全 case（含 Claude Code metadata.user_id 真实样本）；
  - 错误分类表驱动测试（覆盖 §6.1 全部行）；
  - 重试 FSM 全路径（预算耗尽、流中断、Retry-After 遵从）；
  - 熔断转移全路径（含 cooldown 指数递增）；
  - 权重合成边界（headroom=0、health=0.01、tierFactor=0）。

**验收**：`go test ./relay/dispatch/...` 全绿，覆盖率 ≥ 90%（纯函数无理由达不到）；`relay/dispatch` 的 import 列表中无 gf / dao / redis。

### 阶段 1：适配层 + 数据迁移

**交付物**
- `migrations/000014_channel_dispatch_redesign.sql`（§11）+ `gf gen dao`。
- `internal/logic/dispatchadapter/`：
  - catalog.go（内存快照 + 5s 刷新 + pub/sub 失效；快照内容 = 现三表 JOIN 等价数据 + tier + cost_ratio + Redis 健康/负载读值）；
  - state_redis.go（§10 全部 Lua 脚本 + ReportOutcome 后台批量 worker）；
  - state_local.go（降级镜像）;
  - config.go（routing policy 加载/校验/热更新）。
- miniredis 集成测试：Lua 脚本正确性、并发 EWMA 精确计数（100 并发失败 → fail_window_count 精确 =100）、CAS 绑定竞争收敛。

**验收**：迁移 up/down 幂等；tier 自动映射结果人工抽查确认；集成测试全绿。

### 阶段 2：影子模式（只算不用）

**交付物**
- handler 在现有调度结果返回后，**并行调用新 Coordinator 计算一次决策**（仅初始选择，不含重试），将新旧决策写入对比日志（channel 是否一致、不一致时双方 weight_breakdown）。
- 对比统计脚本/看板：一致率、不一致的分布归因。

**验收**：影子运行 ≥ 3 天或 ≥ 10 万请求；对新旧不一致的每一类原因给出解释（预期差异：新方案的健康降权/溢出行为本就应不同）；影子路径 p99 额外耗时 < 1ms（纯内存计算）。

**回滚**：删掉影子调用即可，无状态残留。

### 阶段 3：切换写路径（一次性）

**交付物**
- handler 重试循环替换为 §9.3 的 `Route/Next/Report/Finish` 模式；
- 健康上报、绑定写入、租约全部走新 StatePort；
- 移除现有 `GetChannelForModel` 内的调度逻辑（保留渠道 Key 获取、租户/成员/Key 模型权限校验——它们不属于调度模块，位置不动）；
- cron 新增：Redis 健康 EWMA → `chn_health_scores` 每 5min 快照落盘（替代原实时写）；熔断长期 OPEN → 自动禁用落库。
- 渠道管理后台联动：禁用/删除渠道时调用 `InvalidateChannelBindings` + pub/sub 目录失效。

**验收**：DoD 全项（手动端到端 + 单测 ≥80% + 错误码文案 + 日志含 request_id + 权限不变）；压测对比切换前后 p50/p99 与 DB QPS（预期 DB 调度查询归零）；故障注入演练：单渠道 502 风暴（应原地重试消化）、单渠道 401（应 <1s 熔断）、primary 全灭（应扩组到 secondary）。

**回滚**：git revert 本阶段合并提交（阶段 0-2 的代码无行为影响可留）；Redis 新 key 前缀独立，回滚后旧逻辑不受污染。

### 阶段 4：溢出与成本完整化

**交付物**
- softLimit 429 自动估计器上线（此前只用手动 max_concurrency）；
- 管理后台：渠道表单加 tier 下拉、abilities 列表加 cost_ratio 编辑 + CSV 批量导入；
- rampFactor 接通熔断恢复与新渠道创建事件；
- `dispatch_overflow_total` 等指标接入 ops 仪表盘。

**验收**：人工压测单 primary 渠道至饱和，观测溢出指标与 secondary 流量占比曲线符合 §8.3 预期。

### 阶段 5：清理与收尾

**交付物**
- 删除：legacy 排序调度、v2 旧调度（`relay/scheduler` 旧实现）、旧亲和（`affinity.go`）、旧健康实时写（`health.go` 的调度耦合部分）、`channel_scheduler_v2_enabled` 等四个布尔开关及其读取点；
- `chn_channel_affinities` 遗留表（若存在）标记废弃；
- 文档：运维手册（policy 参数调优指引、熔断/溢出排障流程）、接入文档补 `X-Session-Id` 说明；
- 影子对比代码移除。

**验收**：全仓 grep 无旧开关引用；`relay/scheduler` 目录仅剩新实现或已并入 `relay/dispatch`。

---

## 17. 测试策略

| 层 | 手段 | 关键用例 |
|---|---|---|
| 纯核心 | 表驱动单测 + 分布采样 | HRW 权重比例；粘性稳定性（同 seed 万次同结果）；权重连续变化下绑定守卫的防抖有效性（模拟 headroom 抖动，断言重绑次数 ≤ 阈值） |
| 熔断/重试 | 假时钟状态机测试 | 全转移路径；cooldown 递增；探测窗口 |
| 适配层 | miniredis | Lua 原子性（并发 100 goroutine）；CAS 收敛；租约过期自愈 |
| 协调器 | 假端口注入 | 端到端场景剧本：「primary 渐进饱和 → 新会话溢出比例曲线」「亲和渠道健康跌破 keepHealthMin → 下一请求重绑」「502×2 原地恢复不换渠道」 |
| 系统级 | 影子模式（阶段 2）+ 故障注入（阶段 3） | 见分阶段验收 |

---

## 18. 问题覆盖对照表

| 问题编号 | 问题 | 本方案对应机制 |
|---|---|---|
| A1 🔴 | 亲和命中绕过健康度降权 | 粘性由权重涌现（§2）；绑定合格性含 healthFactor ≥ 0.5（§5.2） |
| A2 🟡 | 亲和仅在最高优先级组内生效 | 无优先级分组，tier 只是权重因子（§4.1） |
| A3 🔴 | 任何可重试错误清除亲和 | 失败只排除不删绑定（§5.2、§6） |
| R1 🟡 | 重试无退避 | 指数退避 + jitter + Retry-After（§6.2） |
| R2 🟡 | 瞬时错误无原地重试 | 双预算，TRANSIENT 原地重试（§6.1/6.2） |
| H1 🔴 | 健康度读-改-写竞态 | Lua 单脚本原子 EWMA（§10.2） |
| H2 🟡 | 失败路径同步写库阻塞重试 | ReportOutcome fire-and-forget + 后台批量（§10.2） |
| P1 🟡 | 每次重试重查 DB | 内存目录快照，热路径零 DB（§9.1 catalog） |
| O1 🔴 | legacy/v2 双路径 | 单路径 + 影子灰度 + 阶段 5 删旧码（§16）；policy 校验拒绝非法配置（§12） |
| L2 ⚪ | 无优先级溢出分流 | headroom 连续溢出 + 租约硬拒绝 + 扩组兜底（§8.3） |
| L3 ⚪ | 亲和无会话维度 | 会话键解析链（§3.1） |

---

## 19. 风险与待验证项

1. **Claude Code metadata.user_id 格式**：方案假设其内嵌 `session_<uuid>` 段。阶段 0 开始前抓真实流量样本验证；若格式变化或缺失，解析链自动落到身份级，功能不受损、仅粒度退化（已有 `dispatch_session_source_total` 指标持续监控占比）。
2. **tierFactor/γ 等参数的初值合理性**：默认值来自推导而非实测。阶段 2 影子模式的 weight_breakdown 日志用于校准；参数全部热更新，无需发版调整。
3. **429 自动估计的冷启动**：无 429 历史的渠道视为无限容量，首次过载会真实吃到一轮 429 才收敛。可接受（429 本身会触发 failover，用户无感）；不可接受时运营手动配 max_concurrency。
4. **绑定守卫与 HRW 的一致性漂移**：绑定长期存活期间权重结构可能大变（如新增渠道），绑定会"钉"在旧选择上直到 TTL 过期或不合格。这是有意为之（保缓存优先于再均衡）；如需主动再均衡，未来可加"绑定年龄上限"策略参数，本期不做。
5. **Redis 单点**：本方案把更多状态放进 Redis，Redis 可用性要求提高。降级路径（§13）保证不可用时仍可服务；生产部署建议 Redis 主从/哨兵（部署层事项，不在本方案范围）。
