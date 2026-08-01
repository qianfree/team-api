# GPT 设计的渠道调度方案

> 状态：待审查
>
> 文档类型：独立设计与实施规格
>
> 适用范围：模型渠道选择、会话亲和、容量溢出、失败重试、流量控制、健康管理和多实例一致性
>
> 设计日期：2026-07-30

## 1. 文档目标

本文定义一套可独立实现的渠道调度系统。实现人员不需要从其他设计文档补充核心语义；本文中的默认值、状态转换、接口边界、数据结构、监控口径和分阶段交付条件共同构成第一版实现基线。

本方案解决以下问题：

1. 同一个 API Key 发起多个会话时，如何优先按会话保持渠道亲和，又如何在没有会话 ID 时合理降级。
2. 优先渠道失败时，什么情况原渠道重试，什么情况切换渠道，什么情况必须停止。
3. 如何表达首选、备用、保底渠道，并在首选渠道高负载时逐步把新流量溢出到备用渠道。
4. 如何把渠道选择、亲和、重试和自动调度拆成低耦合、可测试的独立模块。
5. 如何保证多实例部署下的容量、亲和、健康、熔断和临时流量策略一致。
6. 如何观察每次选择的原因，并临时调整某个渠道的流量比例、安全回滚。

## 2. 范围与非目标

### 2.1 本期范围

- 同步 HTTP 模型请求和 SSE 流式请求的渠道调度。
- 按租户、用户、API Key、模型、会话构造路由上下文。
- 模型能力、租户范围、渠道状态等硬资格过滤。
- 会话级软亲和及身份级降级亲和。
- 首选、备用、保底三级渠道。
- 基于并发、吞吐、排队和延迟的负载溢出。
- 渠道级和凭证级错误归因。
- 原渠道重试、凭证轮换和跨渠道故障转移。
- Redis 分布式容量租约、亲和、冷却和熔断状态。
- 临时流量比例、灰度、排空和暂停控制。
- 指标、日志、Trace、看板、告警和调度解释器。
- 影子运行、灰度切换和版本化回滚。

### 2.2 非目标

- 不在第一版实现跨地域全局最优路由。
- 不根据提示词内容推断业务类型或会话身份。
- 不保证任意短时间窗口内流量比例数学上完全精确。
- 不把调度器拆成独立网络微服务；第一版采用进程内核心模块和分布式状态存储。
- 不负责模型协议转换、计费计算、响应内容转换和审计内容脱敏，这些能力通过执行器边界接入。
- 不通过请求内容相似度生成会话指纹。

## 3. 已确定的核心决策

| 主题 | 决策 |
|---|---|
| 调度单位 | 默认按会话分配渠道；无会话信号时按身份级软亲和降级 |
| 渠道层级 | 固定为 `preferred`、`standby`、`emergency` 三层 |
| 层级语义 | 首选低压时独占新会话；高压时渐进溢出到备用；保底只在前两层不可用时使用 |
| 层内算法 | 加权 Rendezvous Hash，保证会话稳定和多实例确定性 |
| 亲和性质 | 软绑定；健康、熔断、硬容量和高压逃逸条件优先于亲和 |
| 重试模型 | 原渠道重试、同渠道凭证轮换、跨渠道切换分别计数 |
| 504 处理 | 不默认原渠道重试；只对可重放且未输出响应的请求允许受限 failover |
| 流式安全 | 向客户端写出任何有效响应字节后禁止切换渠道 |
| 非幂等安全 | 请求可能已送达上游时禁止自动重放，除非上游支持并已使用幂等键 |
| 配置真相源 | PostgreSQL 保存持久策略和审计记录 |
| 运行状态真相源 | Redis 保存容量、亲和、冷却、熔断和实时健康状态 |
| Redis 降级 | 使用本地最近快照和保守容量；严格容量渠道默认 fail-closed |
| 临时流量控制 | 版本化、带 TTL、支持渐进生效和自动回滚，不直接修改永久基础权重 |
| 可观察性 | 每次选择必须有稳定原因码；指标区分首次分配、全部尝试和成功流量 |

## 4. 术语与不变量

### 4.1 术语

- **逻辑请求**：客户端发起的一次请求，不因内部重试而改变 Request ID。
- **尝试**：逻辑请求向某个渠道、某个凭证发送的一次上游请求。
- **新会话**：没有有效亲和绑定的显式会话或身份级主体。
- **亲和绑定**：一个路由主体在一段 TTL 内优先使用某个渠道的记录。
- **逃逸**：亲和渠道不再满足复用条件，当前请求临时或永久离开该渠道。
- **压力**：渠道并发、Token 吞吐、排队和延迟相对限制的综合负载值。
- **软限制**：开始溢出新会话的压力阈值。
- **硬限制**：不再允许获取新容量租约的上限。
- **基础权重**：渠道的长期容量权重。
- **有效权重**：基础权重经过健康、容量、恢复爬坡和临时策略修正后的权重。
- **分配比例**：调度器对首次分配或新会话做出的选择比例，不等同于最终成功请求比例。

### 4.2 必须始终成立的不变量

1. 禁用、无模型能力、超出租户渠道范围或熔断 OPEN 的渠道不得被选择。
2. 未成功获得硬容量租约的渠道不得发送请求。
3. 同一个逻辑请求在所有尝试中使用同一个全局唯一 Request ID。
4. 向客户端写出有效响应字节后不得跨渠道重试。
5. 非幂等请求在“可能已送达上游”时不得自动重放。
6. 人工流量策略不得绕过模型能力、禁用状态、熔断 OPEN 和硬容量限制。
7. 所有重试都必须受尝试次数、切换次数、总耗时和请求 deadline 限制。
8. 监控指标不得使用用户 ID、API Key ID、会话 ID 或 Request ID 作为标签。
9. 多实例对同一显式会话、同一策略版本和同一候选快照应产生相同的初始排序。
10. 临时策略必须可审计、可过期、可回滚；默认不允许永久生效。

## 5. 总体架构

```text
┌──────────────────────────────────────────────────────────────┐
│ Request Normalizer                                           │
│ 协议解析、会话信号、幂等性、成本类型、deadline、流式状态       │
└───────────────────────────┬──────────────────────────────────┘
                            │ SchedulingRequest
┌───────────────────────────▼──────────────────────────────────┐
│ Scheduling Coordinator                                      │
│ 获取目录快照、运行快照、亲和绑定，调用纯调度核心，申请容量租约 │
└───────────────┬───────────────────────┬──────────────────────┘
                │                       │
┌───────────────▼────────────┐  ┌───────▼──────────────────────┐
│ Scheduler Core             │  │ Runtime Adapters             │
│ 硬过滤、层级溢出、HRW、重试 │  │ PostgreSQL、Redis、本地快照  │
│ 纯函数、无网络和存储 I/O    │  │ Lua、缓存、发布订阅          │
└───────────────┬────────────┘  └──────────────────────────────┘
                │ Decision / RetryDecision
┌───────────────▼──────────────────────────────────────────────┐
│ Executor                                                     │
│ 协议适配、HTTP/SSE、凭证注入、响应写入、结果事件上报           │
└───────────────┬──────────────────────────────────────────────┘
                │ AttemptResult
┌───────────────▼──────────────────────────────────────────────┐
│ Observer                                                     │
│ 健康、熔断、冷却、亲和提交、指标、日志、Trace、审计            │
└──────────────────────────────────────────────────────────────┘
```

### 5.1 控制面与数据面

控制面负责低频变更：

- 渠道角色、基础权重、容量限制。
- 模型能力和租户可用范围。
- 路由策略和临时流量覆盖。
- 策略版本发布、审计和回滚。

数据面负责每个请求：

- 读取本地目录快照和 Redis 运行快照。
- 解析亲和、选择渠道、获取容量租约。
- 执行请求和重试状态机。
- 上报结果并原子更新运行状态。

控制面故障不应中断已经发布策略的数据面。数据面必须始终能够使用最近一个完整策略版本继续工作。

## 6. 标准化请求模型

所有协议进入调度器前转换为统一结构：

```go
type SchedulingRequest struct {
    RequestID       string
    TenantID        int64
    UserID          int64
    APIKeyID        int64
    CanonicalModel  string
    Operation       OperationType
    Stream          bool
    Session         SessionIdentity
    Replayability   Replayability
    EstimatedTokens int64
    Deadline        time.Time
    PolicyVersion   int64
}

type SessionIdentity struct {
    Kind  SessionKind // explicit / protocol / identity / none
    Token string      // 进入存储和日志前必须哈希
}

type Replayability uint8

const (
    ReplayUnknown Replayability = iota
    ReplaySafe
    ReplayCostly
    ReplayUnsafe
)
```

### 6.1 操作类型与默认可重放性

| 操作 | 默认分类 | 说明 |
|---|---|---|
| 模型列表、查询类接口 | `ReplaySafe` | 无副作用 |
| Embedding、Rerank | `ReplaySafe` | 结果可重复计算，但仍有成本 |
| 文本 Chat/Completion | `ReplayCostly` | 通常无外部副作用，但可能重复计费 |
| 工具调用生成 | `ReplayCostly` | 只重放模型请求，不得重复执行客户端工具 |
| 图片、视频生成 | `ReplayUnsafe` | 可能产生重复任务和高额成本 |
| 异步任务提交 | `ReplayUnsafe` | 除非上游支持幂等键 |

当上游支持幂等键且执行器确认已传递同一个逻辑请求幂等键时，可以把对应请求从 `ReplayUnsafe` 提升为 `ReplayCostly`，但不得提升为 `ReplaySafe`。

## 7. 会话识别与亲和

### 7.1 会话信号优先级

按以下顺序选择第一个合法、非空的信号：

1. `X-Session-Id` 或系统统一定义的等价 Header。
2. `previous_response_id`。
3. `conversation_id`。
4. `thread_id`。
5. `tenantID:userID:apiKeyID:canonicalModel` 身份级回退。
6. 若策略关闭身份级回退，则为 `none`，每次按无状态新主体调度。

Header 长度默认不超过 256 字节；协议 ID 不超过 512 字节。超长、包含控制字符或不符合允许字符集的值视为无效，并记录低基数原因码，不记录原值。

Claude Code 有本地会话概念，但 Anthropic Messages API 不提供网关可依赖的标准会话字段。只有实际出现在请求 Header 或 Body 中的值才能用作会话信号。不能根据 User-Agent 推断会话，也不能把 `metadata.user_id` 默认解释为会话 ID。

### 7.2 无会话 ID 的能力边界

服务端没有会话信号时无法同时满足以下两个目标：

- 区分同一 API Key 下的多个独立会话。
- 保证同一会话稳定命中同一渠道。

因此默认采用身份级软亲和，并通过短 TTL 和容量逃逸限制集中风险。不得使用以下不可靠方案：

- HTTP/TCP 连接作为会话标识。
- 请求 ID 作为会话标识。
- 对消息正文做相似度或内容指纹。
- 使用客户端 IP 或 User-Agent 拼接伪会话。

### 7.3 亲和命名空间

```text
namespace = tenantID | userID | apiKeyID | canonicalModel | policyRoutingEpoch
affinityKey = SHA256(namespace | sessionKind | sessionToken)
```

`policyRoutingEpoch` 只在需要主动重分布所有会话的重大策略变更时递增，普通权重调整不得递增。

### 7.4 TTL 默认值

| 会话类型 | TTL | 续期方式 |
|---|---:|---|
| 显式 Header 会话 | 60 分钟 | 成功请求滑动续期 |
| 协议原生会话 | 120 分钟 | 成功请求滑动续期 |
| 身份级回退 | 10 分钟 | 成功请求滑动续期 |
| 临时 failover 路由 | 30 秒 | 不续期 |

### 7.5 亲和复用条件

已有绑定只有同时满足以下条件才可复用：

- 渠道通过所有硬过滤。
- 熔断器状态为 CLOSED，或本请求持有 HALF_OPEN 探测许可。
- 渠道压力小于 `affinity_escape_pressure`，默认 0.90。
- 渠道未被当前请求排除或冷却。
- 绑定对应的策略路由纪元有效。

不满足条件时分两类处理：

- **临时绕开**：429、短时满载、一次 502/503。保留原绑定，当前请求使用临时 failover 路由。
- **永久迁移**：渠道禁用、熔断 OPEN、模型能力移除、持续健康失败。新渠道成功后以 CAS 更新绑定。

## 8. 渠道目录与候选模型

调度核心接收的候选必须是完整快照：

```go
type Candidate struct {
    ChannelID           int64
    Role                ChannelRole
    BaseWeight          float64
    Enabled             bool
    SupportedModels     ModelSet
    TenantScope         Scope
    ConcurrencyLimit    int64
    TokenRateLimit      int64
    QueueLimit          int64
    TargetTTFT          time.Duration
    CredentialAvailable bool
    RecoveryStartedAt   time.Time
}

type RuntimeSnapshot struct {
    ChannelID       int64
    Inflight        int64
    TokenRate       float64
    QueueDepth      int64
    TTFTP95         time.Duration
    HealthFactor    float64
    CircuitState    CircuitState
    CooldownUntil   time.Time
    SnapshotAt      time.Time
}
```

目录快照必须携带版本号。一次逻辑请求的重试可以读取更新后的运行快照，但必须使用相同的持久策略版本，避免同一请求在策略发布边界出现不可解释的规则变化。

## 9. 渠道选择算法

### 9.1 第一步：硬过滤

按稳定顺序过滤，并记录每个候选的第一个排除原因：

1. `channel_disabled`
2. `model_unsupported`
3. `tenant_scope_denied`
4. `credential_unavailable`
5. `circuit_open`
6. `channel_cooldown`
7. `request_excluded`
8. `hard_capacity_full`
9. `runtime_snapshot_expired`

运行快照默认超过 15 秒视为过期。过期时的行为由渠道容量模式决定：

- 严格容量渠道：排除，原因 `runtime_snapshot_expired`。
- 弹性容量渠道：使用本地最近快照并施加 0.5 的容量折减系数。

### 9.2 第二步：尝试亲和

若亲和绑定存在且满足复用条件，调度器返回亲和渠道作为首选，并同时返回按正常算法计算的后备顺序。容量租约申请失败时直接使用后备顺序，不重新访问数据库。

### 9.3 第三步：计算压力

每个渠道计算四个压力分量：

```text
concurrencyPressure = inflight / max(concurrencyLimit, 1)
tokenPressure       = tokenRate / max(tokenRateLimit, 1)
queuePressure       = queueDepth / max(queueLimit, 1)
latencyPressure     = TTFTP95 / max(targetTTFT, 1ms)
```

未配置某项限制时，该分量不参与。综合压力为：

```text
pressure = clamp(max(all configured pressure components), 0, 2)
```

角色压力使用该角色内健康候选的加权平均值，并额外计算是否所有候选达到硬限制。不得使用单个最差渠道代表整个角色。

### 9.4 第四步：选择角色

默认阈值：

```text
overflow_start = 0.70
overflow_full  = 0.90
overflow_exit  = 0.60
```

角色选择规则：

1. 没有可用首选渠道时，进入备用。
2. 首选压力低于 0.70 时，新会话进入首选。
3. 首选压力位于 0.70 到 0.90 时，按线性比例把新会话分配给备用：

   ```text
   overflowRatio = (preferredPressure - 0.70) / (0.90 - 0.70)
   ```

4. 使用 `Hash(sessionToken | policyVersion | "role")` 映射到 `[0,1)`；小于 `overflowRatio` 的新会话进入备用，否则进入首选。
5. 首选压力达到或超过 0.90 时，新会话进入备用。
6. 备用不可用时仍允许首选在硬容量以内接收请求。
7. 首选和备用均不可用时进入保底。
8. 保底层默认不接收常规探活以外的业务流量。

`overflow_exit` 用于状态迟滞：系统已经进入溢出状态后，首选角色压力必须降到 0.60 以下并持续 30 秒，才恢复“首选独占新会话”。溢出状态保存在 Redis，避免各实例各自判断产生抖动。

### 9.5 第五步：应用临时流量覆盖

临时策略可以在角色选择前或角色内生效：

- `global`：参与全局角色分配，能够让备用渠道在低压时接收指定比例。
- `within_role`：只调整已经选中角色内的比例，默认模式。
- `overflow_only`：只有系统处于溢出状态时才生效。

硬过滤始终先于临时策略。任何人工策略都不能强制选择不合格渠道。

### 9.6 第六步：计算有效权重

```text
capacityHeadroom = clamp(1 - pressure, 0.05, 1)
recoveryRamp     = clamp(recoveryAge / recoveryRampDuration, 0.05, 1)

effectiveWeight = baseWeight
                × healthFactor
                × capacityHeadroom
                × recoveryRamp
                × temporaryMultiplier
```

默认 `recoveryRampDuration` 为 10 分钟。刚恢复的渠道至少有 0.05 的爬坡因子，保证能够获得少量验证流量，但不能立即承接满额流量。

### 9.7 第七步：加权 Rendezvous Hash

对角色内每个候选计算：

```text
u = uniformHash(sessionRoutingKey, channelID) in (0, 1)
score = -ln(u) / effectiveWeight
selected = candidate with minimum score
```

排序所有候选而不是只返回一个，得到稳定的后备顺序。分数相同时按 Channel ID 升序，确保所有实例一致。

当 `SessionKind=none` 时，`sessionRoutingKey` 使用 Request ID；这会按请求分散流量，不提供会话亲和。

### 9.8 第八步：获取容量租约

协调器按候选顺序原子获取容量租约：

1. 获取成功：执行请求。
2. 获取失败：记录 `lease_denied`，尝试下一候选；不消耗上游重试次数。
3. 当前角色全部租约失败：重新评估下一角色。
4. 所有角色失败：返回明确的 `capacity_exhausted`，不得伪装为上游 502。

### 9.9 选择伪代码

```go
func Decide(in DecisionInput) Decision {
    eligible := HardFilter(in.Candidates, in.Runtime, in.Request)
    if len(eligible) == 0 {
        return Decision{Reason: ReasonNoEligibleChannel}
    }

    if bound := KeepableAffinity(in.Affinity, eligible, in.Policy); bound != nil {
        ordered := RankFallbacks(eligible, in.Request, in.Policy, in.Overrides)
        return Decision{Primary: *bound, Fallbacks: ordered, Reason: ReasonAffinityHit}
    }

    role := SelectRole(eligible, in.Request, in.Runtime, in.Policy, in.Overrides)
    ranked := WeightedRendezvous(eligible.ForRole(role), in.Request.Session, in.Overrides)
    return Decision{Primary: ranked[0], Fallbacks: ranked[1:], Reason: ReasonRoleSelected}
}
```

## 10. 临时流量比例与操控性

### 10.1 支持的操作

| 模式 | 参数 | 行为 |
|---|---|---|
| `weight_multiplier` | `multiplier` | 在基础有效权重上乘以临时系数 |
| `target_share` | `share` | 指定目标分配比例 |
| `canary` | `share` | `target_share` 的受限形式，默认不超过 5% |
| `drain` | 无 | 不分配新会话，已有亲和会话继续 |
| `force_drain` | 无 | 不分配任何请求，并使已有绑定迁移 |
| `pause` | 无 | 临时从候选集中摘除，不修改永久渠道状态 |
| `ramp` | 起止比例、时间 | 在时间区间内线性或分段调整比例 |

### 10.2 分配单位

```text
allocation_unit = session | first_attempt | request
```

- `session`：默认。调整新会话比例，保留会话亲和。
- `first_attempt`：每个逻辑请求首次尝试按目标比例分配，不统计内部重试。
- `request`：每个请求独立分配，比例收敛最快，但主动破坏亲和。

管理 API 使用 `request` 时必须要求 `acknowledge_affinity_break=true`。

### 10.3 目标比例计算

同一作用域内存在一个或多个固定目标比例时：

```text
fixedTotal = sum(all fixed target shares)
remaining  = 1 - fixedTotal
```

- `fixedTotal > 1`：拒绝发布。
- 指定目标的渠道使用目标比例作为归一化权重。
- 未指定目标的渠道按各自基础有效权重瓜分 `remaining`。
- 目标渠道不可用时不把流量强行发送给它；其份额按剩余可用渠道有效权重重新分配。
- 目标渠道恢复后必须经过恢复爬坡，逐步回到目标比例。

目标比例控制的是统计意义上的分配结果。短窗口、会话请求量差异、重试、渠道满载和健康过滤都会使“全部请求占比”偏离目标。

### 10.4 渐进生效

所有提高流量的操作默认要求 `ramp_up >= 60s`。当前比例 `p0`、目标比例 `p1`、开始时间 `t0`、结束时间 `t1`：

```text
p(t) = p0 + (p1 - p0) × clamp((t - t0) / (t1 - t0), 0, 1)
```

降低流量允许立即生效，但 `force_drain` 必须使用高权限。

### 10.5 过期与回滚

- 临时策略默认最长 24 小时。
- 创建时必须设置 `expires_at`；永久变更必须走基础策略发布流程。
- 到期由 Redis TTL 和控制面定时任务双重保证。
- 每次发布生成不可变版本，回滚通过重新激活上一版本完成。
- 数据面发现策略版本缺失或内容校验失败时继续使用上一完整版本。

### 10.6 比例观察口径

控制面必须同时展示：

- `new_session_share`：新会话分配比例。
- `first_attempt_share`：逻辑请求首次尝试比例。
- `attempt_share`：包括重试在内的全部上游尝试比例。
- `success_share`：成功请求比例。
- `token_share`：处理 Token 比例。
- `cost_share`：估算或实际成本比例。

用户设置的目标必须明确绑定其中一个口径。默认是 `new_session_share`。

## 11. 重试和故障转移状态机

### 11.1 独立预算

默认预算：

```text
max_total_attempts       = 4
max_same_channel_retries = 1
max_credential_rotations = 1
max_channel_switches     = 2
max_retry_elapsed        = min(15s, request_deadline_remaining)
```

图片、视频和异步任务提交默认：

```text
max_total_attempts       = 1
max_same_channel_retries = 0
max_channel_switches     = 0
```

只有明确未送达或已使用上游幂等键时，才允许策略提高这些上限。

### 11.2 送达状态

执行器必须把失败标记为以下之一：

```go
type DeliveryState uint8

const (
    DeliveryNotSent DeliveryState = iota
    DeliveryMaybeSent
    DeliveryResponseReceived
    DeliveryResponseStarted
)
```

- `NotSent`：DNS 失败、连接拒绝、TLS 建连失败、请求体尚未发送。
- `MaybeSent`：写入后连接重置、读取超时、context deadline、无响应 EOF。
- `ResponseReceived`：收到完整 HTTP 状态行和错误响应，尚未写给客户端。
- `ResponseStarted`：已向客户端写出有效响应字节。

### 11.3 错误归因

```go
type FailureScope uint8

const (
    FailureRequest FailureScope = iota
    FailureCredential
    FailureChannel
    FailureProvider
    FailureClient
)
```

- 400、413、422：请求级，不降低渠道健康。
- 401、403：优先归因凭证；同渠道所有凭证均失败后才提升为渠道级。
- 429：根据响应头和供应商语义归因到凭证、渠道或 Provider。
- 500、502、503、504：渠道或 Provider 级。
- 客户端取消：客户端级，不降低渠道健康。

### 11.4 决策矩阵

| 错误 | 原渠道 | 凭证轮换 | 跨渠道 | 亲和处理 |
|---|---|---|---|---|
| 400/404/409/413/422 | 停止 | 否 | 否 | 保留 |
| 401/403 | 不原样重试 | 是，最多一次 | 凭证耗尽后允许 | 连续凭证失败后迁移 |
| 408 | 可重放且未输出时一次 | 否 | 允许 | 临时绕开 |
| 429 | 不立即原地重试 | 凭证级限流时允许 | 允许 | 保留，设置临时路由 |
| 500 | `ReplaySafe/Costly` 一次 | 否 | 允许 | 单次保留，持续失败迁移 |
| 502/503 | 一次快速重试 | 否 | 允许 | 单次保留，持续失败迁移 |
| 504 | 默认不原地重试 | 否 | 仅 `ReplaySafe/Costly` | 临时绕开或持续失败迁移 |
| DNS/连接拒绝/TLS 建连失败 | 一次快速重试 | 否 | 允许 | 临时绕开 |
| 写后 RST/EOF/读取超时 | 仅 `ReplaySafe` | 否 | `ReplaySafe/Costly` 受限 | 不立即改绑 |
| 客户端取消 | 停止 | 否 | 否 | 保留 |
| 已输出流式响应 | 停止 | 否 | 否 | 保留 |

对于 `ReplayUnsafe + DeliveryMaybeSent`，状态码是否可重试不再重要，必须停止。

### 11.5 退避

```text
same-channel retry: 100ms × 2^n + random(0, 100ms)
cross-channel retry: 50ms + random(0, 100ms)
429: honor Retry-After; 当前请求直接切换，不等待超长 Retry-After
```

所有等待使用可取消 Timer，并在等待前检查剩余 deadline。预测下一次尝试无法在 deadline 前完成时立即停止。

### 11.6 重试状态

```go
type AttemptState struct {
    TotalAttempts       int
    SameChannelRetries  map[int64]int
    CredentialRotations map[int64]int
    ChannelSwitches     int
    ExcludedChannels    map[int64]Exclusion
    ExcludedCredentials map[int64]Exclusion
    StartedAt           time.Time
    ResponseStarted     bool
}
```

### 11.7 状态转换

```text
SELECT_CHANNEL
  → ACQUIRE_LEASE
      → lease denied → SELECT_NEXT
      → acquired → EXECUTE
          → success → COMMIT_AFFINITY → COMPLETE
          → credential failure → ROTATE_CREDENTIAL
          → same-channel retry → BACKOFF → EXECUTE
          → channel failover → RELEASE → SELECT_NEXT
          → non-retryable/预算耗尽/响应已开始 → FAIL
```

## 12. 健康度与熔断

### 12.1 健康信号

每个渠道维护：

- 请求成功率 EWMA。
- 500/502/503/504 分类错误率。
- 429 比例及作用域。
- TTFT EWMA 和 P95。
- 总响应延迟 EWMA 和 P95。
- 连续渠道级失败次数。
- 当前并发、Token 速率和队列长度。
- 最近一次成功、失败和探测时间。

请求错误和客户端取消不得计入渠道失败率。凭证错误先进入凭证健康状态，避免一个坏 Key 直接熔断整个渠道。

### 12.2 EWMA

默认 `alpha=0.2`：

```text
newValue = alpha × observation + (1 - alpha) × oldValue
```

健康因子不需要追求一个绝对“健康分”，但选择算法需要 `[0,1]` 因子。默认计算：

```text
successFactor = clamp(successEWMA, 0, 1)
latencyFactor = clamp(targetTTFT / max(ttftEWMA, targetTTFT), 0.2, 1)
stabilityFactor = clamp(1 - latencyCV, 0.5, 1)

healthFactor = clamp(
    0.60 × successFactor +
    0.25 × latencyFactor +
    0.15 × stabilityFactor,
    0.05,
    1,
)
```

熔断状态单独作为硬门控，不依赖健康因子降到零。

### 12.3 熔断规则

默认参数：

```text
window                    = 30s
minimum_observations      = 20
open_error_rate           = 50%
open_consecutive_failures = 5
open_duration             = 30s
half_open_max_probes      = 1
close_successful_probes   = 2
```

状态转换：

- CLOSED → OPEN：窗口内达到最小样本且渠道级错误率不低于 50%，或连续渠道级失败达到 5。
- OPEN → HALF_OPEN：OPEN 持续 30 秒后允许竞争探测许可。
- HALF_OPEN → OPEN：任一探测失败。
- HALF_OPEN → CLOSED：连续两个探测成功。

HALF_OPEN 探测许可通过 Redis 原子获取，同一渠道全局最多一个并发探测。探测成功前普通请求仍不得进入。

### 12.4 冷却与熔断的区别

- **冷却**：短时间不选，通常来自 429、容量提示或维护窗口；不代表渠道故障。
- **熔断**：渠道级故障保护，需要 HALF_OPEN 探测恢复。
- **暂停**：人工控制面指令，优先于冷却和熔断。

## 13. 容量与负载状态

### 13.1 容量租约

每次上游尝试在发送前获取租约：

```go
type Lease struct {
    LeaseID     string
    ChannelID   int64
    RequestID   string
    AcquiredAt  time.Time
    ExpiresAt   time.Time
    FencingToken uint64
}
```

- Lease ID 使用 UUIDv7。
- 默认租约 90 秒，长请求每 30 秒续租。
- 续租必须确认 Lease ID 仍存在，不能复活已经释放的租约。
- 请求结束立即释放；实例崩溃后依靠 TTL 回收。
- Lua 脚本在一个原子操作中清理过期租约、检查硬上限、增加租约并返回 fencing token。

### 13.2 Token 吞吐

并发数无法反映长上下文请求的真实压力。请求进入时使用 `EstimatedTokens` 预留短时 Token 预算，完成后用实际 Token 修正滑动窗口。估算缺失时使用模型和操作类型的保守默认值。

### 13.3 排队策略

第一版默认不在网关内部建立无界等待队列：

- 有其他渠道时立即溢出。
- 所有渠道满载时返回 `capacity_exhausted` 或进入有界短等待。
- 可选短等待默认不超过 200ms，队列有明确长度上限。
- 等待必须计入请求 deadline。

## 14. 分布式状态设计

### 14.1 Redis Key

| Key | 类型 | 用途 |
|---|---|---|
| `sched:v1:affinity:{hash}` | String/Hash | 亲和渠道、版本、绑定时间，带 TTL |
| `sched:v1:affidx:{channelID}` | Set | 渠道到亲和键的反向索引 |
| `sched:v1:leases:{channelID}` | ZSET | 容量租约，score 为过期时间 |
| `sched:v1:load:{channelID}` | Hash | 并发、Token、队列、压力快照 |
| `sched:v1:health:{channelID}` | Hash | EWMA、分类错误、连续失败 |
| `sched:v1:circuit:{channelID}` | Hash | 熔断状态、打开时间、探测计数 |
| `sched:v1:cooldown:{scope}:{id}` | String | 凭证/渠道/Provider 冷却截止时间 |
| `sched:v1:overflow:{policyScope}` | Hash | 当前溢出状态和迟滞时间 |
| `sched:v1:policy:active` | String | 当前策略版本 |
| `sched:v1:policy:{version}` | String | 已发布策略快照 |
| `sched:v1:override:{id}` | String | 临时策略快照，TTL 到期 |

所有 Key 必须有明确 TTL 或清理责任。反向亲和索引允许渠道永久下线时批量失效绑定，但批量操作必须分页，不能在一个 Lua 脚本中遍历无界集合。

### 14.2 原子操作

必须使用 Lua 或等价原子命令实现：

- `AcquireLease`
- `RefreshLease`
- `ReleaseLease`
- `ObserveAttempt`
- `TryHalfOpenProbe`
- `CommitAffinityCAS`
- `SetCooldown`
- `TransitionOverflowState`

Lua 脚本需要返回结构化结果码，不得仅返回布尔值。例如容量失败要区分 `hard_full`、`strict_state_missing` 和 `lease_conflict`。

### 14.3 PostgreSQL 持久模型

建议新增逻辑表；实际命名可按项目数据库规范调整。

```sql
CREATE TABLE sched_policies (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(128) NOT NULL,
    version         BIGINT NOT NULL,
    scope           JSONB NOT NULL,
    policy          JSONB NOT NULL,
    status          VARCHAR(16) NOT NULL,
    routing_epoch   BIGINT NOT NULL DEFAULT 1,
    created_by      BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    activated_at    TIMESTAMPTZ,
    UNIQUE (name, version)
);

CREATE TABLE sched_traffic_overrides (
    id                  BIGSERIAL PRIMARY KEY,
    scope               JSONB NOT NULL,
    channel_id          BIGINT NOT NULL,
    mode                VARCHAR(32) NOT NULL,
    allocation_unit     VARCHAR(16) NOT NULL,
    apply_scope         VARCHAR(16) NOT NULL,
    value               NUMERIC(8,6),
    ramp_up_seconds     INT NOT NULL DEFAULT 60,
    preserve_affinity   BOOLEAN NOT NULL DEFAULT TRUE,
    reason              VARCHAR(512) NOT NULL,
    status              VARCHAR(16) NOT NULL,
    starts_at           TIMESTAMPTZ NOT NULL,
    expires_at          TIMESTAMPTZ NOT NULL,
    version             BIGINT NOT NULL,
    created_by          BIGINT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL
);

CREATE TABLE sched_policy_audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    object_type     VARCHAR(32) NOT NULL,
    object_id       BIGINT NOT NULL,
    action          VARCHAR(32) NOT NULL,
    before_data     JSONB,
    after_data      JSONB,
    operator_id     BIGINT NOT NULL,
    reason          VARCHAR(512) NOT NULL,
    request_id      VARCHAR(64) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL
);
```

数据库约束或服务层验证必须保证：

- `target_share` 位于 `(0,1]`。
- 同一作用域固定目标比例之和不超过 1。
- `expires_at > starts_at`。
- 临时策略最长持续时间符合权限规则。
- 发布版本严格递增且不可修改。

## 15. 多实例一致性与降级

### 15.1 一致性边界

- 持久策略：强一致发布。先写完整版本，再原子切换 active version。
- 容量：Redis 原子强约束。
- 熔断和 HALF_OPEN：Redis 原子状态机。
- 亲和：首次写入使用 `SET NX` 或 CAS；并发首请求读取最终胜者。
- 指标：最终一致，不参与正确性判断。
- 本地目录：版本化最终一致，实例不得组合两个版本的局部配置。

### 15.2 策略发布

1. 控制面校验策略。
2. 写 PostgreSQL 不可变版本。
3. 写 Redis 完整快照。
4. 原子切换 `sched:v1:policy:active`。
5. 发布 Pub/Sub 通知。
6. 实例收到通知后拉取并校验完整快照。
7. 未收到通知的实例通过 5 秒轮询发现新版本。

### 15.3 Redis 故障降级

| 能力 | 降级行为 |
|---|---|
| 策略 | 使用本地最近完整版本 |
| 会话亲和 | 使用会话种子的无状态 HRW，不能提交新绑定 |
| 健康 | 使用本地最近快照，超过 60 秒施加健康折减 |
| 熔断 | 保留本地 OPEN 状态，不自行提前恢复 |
| 临时比例 | 使用本地最后版本直到其本地过期时间 |
| 弹性容量渠道 | 使用实例级保守并发限制，可配置 fail-open |
| 严格容量/昂贵渠道 | 默认 fail-closed |

实例级保守限制必须由部署配置给出，例如：

```text
localFallbackLimit = floor(globalHardLimit / configuredMaxReplicas)
```

宁可在 Redis 故障期间少用容量，也不能让多实例同时无界放行昂贵渠道。

### 15.4 时钟

- 所有实例和 Redis 节点必须启用时间同步。
- TTL 和租约优先使用 Redis `TIME` 或在 Lua 内取服务器时间。
- 策略生效时间允许最多 2 秒时钟偏差。

## 16. Go 模块边界

建议包结构：

```text
relay/scheduler/
  types.go              领域类型和原因码
  filter.go             硬过滤
  affinity.go           亲和复用和逃逸规则
  pressure.go           压力与角色溢出
  rendezvous.go         加权 HRW 排序
  override.go           临时流量策略计算
  retry.go              重试状态机
  circuit.go            纯熔断转换规则
  policy.go             策略校验和默认值

internal/logic/scheduling/
  coordinator.go        组织一次选择和租约申请
  catalog.go            PostgreSQL 目录及本地缓存
  runtime_store.go      Redis 运行状态适配器
  affinity_store.go     Redis 亲和适配器
  lease_store.go        Redis 容量适配器
  policy_store.go       策略发布和订阅
  observer.go           结果事件消费

relay/handler/
  executor.go           HTTP/SSE 执行边界
  retry_executor.go     按 RetryDecision 执行尝试
```

`relay/scheduler` 不允许导入 ORM、Redis Client、HTTP Client 或应用配置全局单例。

### 16.1 核心接口

```go
type Scheduler interface {
    Decide(input DecisionInput) Decision
    NextRetry(input RetryInput) RetryDecision
}

type Catalog interface {
    Snapshot(ctx context.Context, req SchedulingRequest) (CatalogSnapshot, error)
}

type RuntimeStore interface {
    Snapshot(ctx context.Context, channelIDs []int64) (map[int64]RuntimeSnapshot, error)
    Observe(ctx context.Context, event AttemptEvent) error
}

type AffinityStore interface {
    Get(ctx context.Context, key AffinityKey) (AffinityBinding, bool, error)
    CommitCAS(ctx context.Context, key AffinityKey, old, next AffinityBinding) (bool, error)
    InvalidateChannel(ctx context.Context, channelID int64, cursor string, limit int) (next string, error)
}

type LeaseStore interface {
    Acquire(ctx context.Context, req LeaseRequest) (Lease, LeaseResult, error)
    Refresh(ctx context.Context, lease Lease) error
    Release(ctx context.Context, lease Lease) error
}
```

接口定义在消费方。构造函数返回具体类型。所有外部调用接收 `context.Context`，并有小于请求剩余 deadline 的独立超时。

### 16.2 Decision 结果

```go
type Decision struct {
    Selected       *RankedCandidate
    Fallbacks      []RankedCandidate
    Reason         ReasonCode
    PolicyVersion  int64
    RoutingEpoch   int64
    AffinityAction AffinityAction
    Role           ChannelRole
    OverflowRatio  float64
    Exclusions     []CandidateExclusion
}
```

`ReasonCode` 必须是稳定枚举，至少包含：

- `affinity_hit`
- `affinity_escape_pressure`
- `affinity_escape_health`
- `preferred_normal`
- `standby_overflow`
- `standby_primary_unavailable`
- `emergency_failover`
- `temporary_target_share`
- `canary`
- `capacity_fallback`
- `retry_failover`
- `no_eligible_channel`
- `capacity_exhausted`

## 17. 管理 API

### 17.1 创建临时流量策略

```http
POST /admin/scheduler/traffic-overrides
```

```json
{
  "scope": {
    "model": "claude-sonnet",
    "tenant_id": 0,
    "operation": "chat"
  },
  "channel_id": 12,
  "mode": "target_share",
  "allocation_unit": "session",
  "apply_scope": "within_role",
  "value": 0.20,
  "ramp_up_seconds": 300,
  "preserve_affinity": true,
  "starts_at": "2026-07-30T10:00:00+08:00",
  "expires_at": "2026-07-30T10:30:00+08:00",
  "reason": "临时验证新线路"
}
```

返回内容必须包含策略 ID、版本、校验后的实际生效范围、预计影响和回滚版本。

### 17.2 排空渠道

```http
POST /admin/scheduler/channels/{channelID}/drain
```

请求参数：

- `mode=drain|force_drain`
- `expires_at`
- `reason`
- `acknowledge_affinity_break`，强制排空时必填 `true`

### 17.3 回滚

```http
POST /admin/scheduler/traffic-overrides/{id}/rollback
```

回滚是一次新的审计动作，不删除历史记录。

### 17.4 调度解释器

```http
POST /admin/scheduler/explain
```

输入脱敏后的请求上下文或指定测试主体，返回：

- 策略版本。
- 会话信号类型，不返回原始 Token。
- 所有候选及排除原因。
- 压力分量。
- 角色和溢出比例。
- 基础权重、所有修正因子和最终分数。
- 亲和复用或逃逸原因。
- 最终选择和后备顺序。
- 相关临时策略。

解释器默认只读，不申请租约、不写亲和、不更新健康。

### 17.5 调度模拟器

```http
POST /admin/scheduler/simulate
```

输入候选策略和脱敏流量样本，输出：

- 预计新会话分布。
- 首选到备用的溢出比例。
- 各渠道预计并发和 Token 负载。
- 目标比例偏差。
- 因硬容量无法完成的流量。
- 相对当前策略的会话迁移比例。

模拟结果不得自动发布。

## 18. 可观察性

### 18.1 指标

建议使用以下 Prometheus 指标；标签必须保持低基数。

```text
scheduler_decisions_total{
  model, channel, role, reason, affinity_action
}

scheduler_attempts_total{
  model, channel, attempt_kind, outcome, error_class
}

scheduler_retries_total{
  model, from_channel, action, error_class
}

scheduler_affinity_total{
  model, action, session_kind, reason
}

scheduler_channel_inflight{channel}
scheduler_channel_pressure{channel, component}
scheduler_channel_effective_weight{model, channel}
scheduler_channel_target_share{model, channel, allocation_unit}
scheduler_channel_actual_share{model, channel, share_kind}
scheduler_channel_token_rate{channel}
scheduler_channel_queue_depth{channel}
scheduler_channel_health_factor{channel}
scheduler_circuit_state{channel, state}
scheduler_lease_operations_total{channel, operation, result}
scheduler_policy_version{instance}
scheduler_override_active{channel, mode}

scheduler_decision_duration_seconds{result}
scheduler_attempt_duration_seconds{channel, outcome}
scheduler_ttft_seconds{channel, model}
scheduler_retry_elapsed_seconds{model, final_outcome}
```

如果模型数量不可控，指标中的 `model` 标签使用模型族或受控规范名；完整模型名写入结构化日志。
策略版本不得放入 Counter/Histogram 标签，否则版本持续增长会造成时间序列累积；具体版本写入日志和 Trace，实例当前版本只通过 `scheduler_policy_version` Gauge 表示。

### 18.2 结构化日志

每个逻辑请求默认记录一条汇总日志；完整候选分数只在错误、采样或 Explain 模式记录。

```json
{
  "event": "scheduler_decision",
  "request_id": "019...",
  "trace_id": "...",
  "policy_version": 42,
  "model": "claude-sonnet",
  "session_kind": "explicit",
  "session_hash_prefix": "7f3a91c2",
  "selected_channel": 12,
  "role": "standby",
  "reason": "standby_overflow",
  "preferred_pressure": 0.84,
  "overflow_ratio": 0.70,
  "affinity_action": "new_binding",
  "target_share": 0.20,
  "attempts": 2,
  "final_outcome": "success"
}
```

不得记录原始 API Key、原始会话 Token、完整请求正文或上游凭证。

### 18.3 Trace

一个逻辑请求一个根 Span，每次尝试一个子 Span：

```text
relay.request
  scheduler.decide
  scheduler.lease.acquire
  upstream.attempt[0]
  scheduler.retry
  scheduler.lease.acquire
  upstream.attempt[1]
  scheduler.observe
```

Span 属性包含策略版本、渠道、角色、原因码、尝试序号、送达状态和错误分类，不包含高敏感标识。

### 18.4 看板

至少提供四个看板：

1. **渠道总览**：QPS、成功率、TTFT、并发、Token 速率、压力、熔断状态。
2. **调度分布**：目标比例与新会话、首次请求、全部尝试、成功和 Token 实际比例。
3. **重试与故障转移**：原渠道重试、凭证轮换、跨渠道切换、最终失败、重试增加延迟。
4. **策略与操控**：当前版本、实例版本一致性、临时策略、到期时间、实际偏差和操作审计。

### 18.5 告警

默认告警建议：

- 目标新会话比例偏差超过 10 个百分点并持续 10 分钟。
- 设置目标比例的渠道 5 分钟没有首次分配，且渠道未被硬过滤。
- 跨渠道重试率超过 5% 持续 5 分钟。
- 任一首选角色可用渠道数为 0。
- 保底渠道业务流量持续超过 1%。
- 渠道熔断超过 5 分钟未恢复。
- 容量租约拒绝率超过 10%。
- Redis 调度操作错误持续 1 分钟。
- 实例策略版本不一致超过 30 秒。
- 临时策略将在 10 分钟内到期且仍承载超过 10% 流量。
- 实际 Token 比例显著高于请求比例，提示大请求集中。

## 19. 安全与权限

角色建议：

- `scheduler.viewer`：查看状态、Explain 和模拟。
- `scheduler.operator`：创建有 TTL 的权重、目标比例和 canary。
- `scheduler.admin`：暂停、强制排空、修改永久策略和回滚。

安全规则：

- `force_drain`、请求级分配、超过 50% 的全局目标比例需要管理员权限。
- 所有写操作必须包含原因。
- 高风险操作可配置双人审批。
- 审计日志不可被普通管理员删除。
- 会话 Token 只保存 SHA-256；日志最多展示不可反推的短前缀。
- 管理 API 必须防止跨租户作用域误配置。

## 20. 测试策略

### 20.1 单元测试

纯调度包使用表驱动测试覆盖：

- 所有硬过滤原因。
- 显式会话、协议会话、身份回退和无亲和模式。
- 亲和保留、临时绕开和永久迁移。
- 0.70、0.90、0.60 阈值边界。
- 首选缺失、备用缺失、保底启用。
- 权重为零、健康因子极低、恢复爬坡。
- 单个和多个固定目标比例。
- 多个目标比例之和超过 1 的校验。
- drain、force_drain、pause 和过期策略。
- 每个错误和送达状态组合的 RetryDecision。
- deadline 不足、预算耗尽和响应已开始。

### 20.2 属性测试

验证以下属性：

- 候选顺序变化不改变相同输入的选择结果。
- 相同会话、策略版本和快照始终得到相同排序。
- 禁用候选永远不会被返回。
- 增加一个渠道只迁移有限比例会话。
- 7:3 权重在足够多会话样本上落入容差区间。
- 固定目标比例在足够多新会话样本上收敛。
- 任意错误序列都不会超过重试预算。
- `ResponseStarted=true` 时 NextRetry 永远返回 Stop。

### 20.3 Redis 集成测试

- 多实例并发争抢最后一个租约，只允许一个成功。
- 续租和释放不会复活旧租约。
- 实例崩溃后租约按 TTL 回收。
- 并发首次亲和提交得到唯一绑定。
- 亲和 CAS 防止旧请求覆盖新迁移。
- HALF_OPEN 全局只放行一个探测。
- 并发健康事件无丢失更新。
- 临时策略 TTL 到期后所有实例恢复上一策略。

### 20.4 仿真测试

构造至少以下流量：

- 10 万个等长会话，验证基础比例。
- 少量超长会话和大量短会话，比较会话比例与请求比例差异。
- 首选压力从 40% 缓慢升到 100%，验证渐进溢出。
- 压力在阈值附近波动，验证迟滞不抖动。
- 渠道突然 100% 返回 503，验证熔断和 failover。
- 429 携带不同 Retry-After，验证冷却范围。
- 临时把渠道调到 20%，5 分钟爬坡后自动到期。

### 20.5 故障与压测

- Redis 延迟、超时、主从切换和完全不可用。
- PostgreSQL 不可用但本地已有完整目录。
- 单实例和多实例突然退出。
- 策略发布过程中实例网络分区。
- 上游慢响应、半开连接、SSE 中断。
- 最大配置实例数下验证硬容量不被突破。

## 21. 分阶段实施计划

每个阶段都必须可独立发布、可观察、可回滚。阶段编号表示依赖顺序，不表示必须一次完成该阶段所有增强项。

### 阶段 0：冻结语义与建立基线

**目标**：在写调度代码前固定协议和验收口径。

**任务**：

- 审查并确认本文中的角色、比例口径、重试矩阵和 Redis 降级策略。
- 确认哪些请求属于 `ReplaySafe/Costly/Unsafe`。
- 定义统一 `X-Session-Id` 规范和 SDK 接入要求。
- 收集当前请求量、成功率、P95/P99、TTFT、错误分类、重试率和渠道占比基线。
- 定义 SLO 和错误预算。
- 确认生产最大实例数，用于 Redis 故障时的本地保守容量。

**交付物**：

- 已批准的设计版本。
- 错误分类表和操作类型表。
- 基线看板。
- 配置默认值清单。

**退出条件**：

- 所有存在歧义的协议字段都有唯一解释。
- 可重放性分类覆盖所有公开请求入口。
- 能量化后续新旧调度决策差异。

**回滚**：无运行变更。

### 阶段 1：纯调度核心与影子决策

**目标**：实现无 I/O 的调度核心，不接管生产流量。

**任务**：

- 建立 `relay/scheduler` 领域类型、策略默认值和校验。
- 实现硬过滤、压力计算、角色溢出、加权 HRW 和目标比例。
- 实现亲和复用判断和 RetryDecision 纯状态机。
- 实现稳定原因码和 Decision 序列化。
- 在现有请求路径旁路调用新调度器，只记录影子决策，不改变实际渠道。
- 建立新旧决策差异指标，不把用户或会话放入指标标签。

**测试**：

- 完成本章单元测试和属性测试。
- 使用脱敏历史样本离线回放。
- 对相同输入跨进程验证确定性。

**交付物**：

- 纯调度包。
- Shadow Coordinator。
- 决策差异看板。

**退出条件**：

- 调度核心无数据库、Redis 和 HTTP 依赖。
- 单元和属性测试通过。
- 影子计算增加的 P99 延迟低于 1ms。
- 连续观察至少一个完整业务周期，无 Panic 和无不可解释 Decision。

**回滚**：关闭 `scheduler_shadow_enabled`，不影响实际转发。

### 阶段 2：分布式运行状态和容量租约

**目标**：先建立多实例安全的底座，再接管选择。

**任务**：

- 实现 Redis Key、Lua 脚本和结果码。
- 实现容量获取、续租、释放和实例崩溃回收。
- 实现运行快照批量读取，禁止每候选一次 Redis 往返。
- 实现本地最近快照和 Redis 故障降级。
- 实现 Token 速率估算和实际值修正。
- 增加租约、压力、Redis 错误指标和告警。

**测试**：

- Redis 集成测试和多实例竞争测试。
- Race Detector。
- Redis 故障注入。
- 生产流量等比例压测。

**交付物**：

- RuntimeStore 和 LeaseStore。
- Lua 脚本版本管理。
- Redis 降级 Runbook。

**退出条件**：

- 多实例不能突破严格硬容量。
- 租约泄漏能在 TTL 内回收。
- Redis 故障时严格和弹性渠道行为符合策略。
- Redis 批量读取延迟满足数据面预算。

**回滚**：关闭 `scheduler_distributed_capacity_enabled`，保留影子指标。

### 阶段 3：健康、熔断和凭证归因

**目标**：让调度能够安全识别坏渠道，并避免错误归因扩大故障。

**任务**：

- 实现标准 AttemptEvent 和 DeliveryState。
- 实现请求、凭证、渠道、Provider、客户端错误归因。
- 实现 Redis 原子 EWMA、连续失败和滑动窗口。
- 实现 CLOSED/OPEN/HALF_OPEN 和全局探测许可。
- 实现 429 冷却及 `Retry-After` 解析。
- 实现恢复爬坡。
- 将健康状态接入影子选择，暂不强制生产摘除。

**测试**：

- 错误矩阵全组合测试。
- 并发 Observe 无丢失测试。
- HALF_OPEN 多实例竞争测试。
- 凭证 401 不误伤健康凭证测试。

**交付物**：

- Observer、熔断状态机和健康看板。
- 错误归因说明和运维 Runbook。

**退出条件**：

- 所有上游失败都有明确 error class、failure scope 和 delivery state。
- 自动熔断与人工判断样本一致性达到审查目标。
- 不因客户端取消或请求错误降低渠道健康。

**回滚**：关闭 `scheduler_circuit_enforcement_enabled`，继续只观察健康状态。

### 阶段 4：会话亲和和三级溢出接管

**目标**：让新调度器开始控制生产渠道选择。

**任务**：

- 实现会话信号解析、校验和哈希。
- 实现 AffinityStore、CAS 提交、TTL 续期和分页失效。
- 实现首选、备用、保底及溢出迟滞。
- 实现容量拒绝后的同角色后备和跨角色降级。
- 为自研 SDK 增加 `X-Session-Id`。
- 按租户或 API Key 做 1%、5%、20%、50%、100% 灰度。

**测试**：

- 会话稳定性和逃逸测试。
- 10 万会话分布仿真。
- 阈值爬升和抖动仿真。
- 灰度租户端到端测试。

**交付物**：

- 生产 Selector 和 AffinityStore。
- 亲和/溢出看板。
- SDK 会话 Header 规范。

**退出条件**：

- 健康会话渠道稳定率达到目标。
- 首选低压时备用仅有规定的探活流量。
- 首选高压时新会话按公式进入备用。
- 保底使用都有明确原因码和告警。
- 灰度期间成功率和延迟不劣于基线容差。

**回滚**：按租户关闭 `scheduler_selection_enabled`，恢复旧选择；新状态继续写入但不参与决策。

### 阶段 5：分层重试状态机接管

**目标**：统一原渠道重试、凭证轮换和跨渠道 failover。

**任务**：

- 执行器返回标准 AttemptResult 和 DeliveryState。
- 接入独立重试预算和可取消退避。
- 实现凭证轮换但不重复计入渠道切换。
- 实现流式响应开始后的硬停止。
- 实现非幂等请求保护和上游幂等键传递。
- 记录完整尝试链 Trace 和逻辑请求汇总日志。

**测试**：

- 决策矩阵端到端测试。
- SSE 首字节前后故障测试。
- 504、写后 RST、读取超时和客户端取消测试。
- 图片、视频和任务提交不重复执行测试。
- 重试预算和 deadline 属性测试。

**交付物**：

- Retry Executor。
- 重试和故障转移看板。
- 504、429、流式中断 Runbook。

**退出条件**：

- 任意错误序列不突破预算。
- 已输出响应后不存在跨渠道尝试。
- 非幂等模糊送达不存在自动重放。
- 重试带来的成功率收益和延迟/成本增量可量化。

**回滚**：关闭 `scheduler_retry_fsm_enabled`，选择器仍可继续工作。

### 阶段 6：控制面和临时流量比例

**目标**：允许运维安全、可观察地调整流量。

**任务**：

- 创建策略、临时覆盖和审计表。
- 实现发布、版本切换、Pub/Sub、轮询和回滚。
- 实现 target share、multiplier、canary、drain、force drain、pause 和 ramp。
- 实现目标比例冲突校验和过期回收。
- 实现管理 UI：目标与实际比例、预计影响、到期时间、一键回滚。
- 实现 Explain 和离线模拟 API。
- 接入 RBAC 和高风险操作确认。

**测试**：

- 多实例策略原子切换。
- 固定目标比例收敛测试。
- TTL 到期和回滚测试。
- 权限、审计和跨租户作用域测试。
- 渠道满载时目标无法完成的展示测试。

**交付物**：

- 调度控制面 API 和 UI。
- 策略版本与审计记录。
- 比例控制 Runbook。

**退出条件**：

- 可在不改永久权重的情况下把指定渠道的新会话目标比例临时调整到指定值。
- 操作到期自动恢复，所有实例版本一致。
- 管理端明确区分目标比例与五种实际比例。
- 硬过滤导致目标无法完成时有明确原因和告警。

**回滚**：原子激活上一策略版本；紧急情况下关闭 `scheduler_overrides_enabled`。

### 阶段 7：全面切换和收尾

**目标**：将新调度路径设为唯一生产路径，完成运维交接。

**任务**：

- 逐步扩大到所有租户和模型。
- 完成生产故障演练：Redis、数据库、上游、实例退出和错误策略发布。
- 固化 SLO、告警值、值班手册和应急操作权限。
- 观察至少两个完整业务周期。
- 停止旧路径写入冲突状态。
- 在确认无需回滚后移除旧路径和临时兼容开关。

**退出条件**：

- 全量流量达到成功率、延迟、成本和容量 SLO。
- 所有 P1 级故障场景完成演练。
- 值班人员能够通过 Explain、Trace 和看板解释异常选择。
- 临时比例调整、排空、熔断恢复和策略回滚均有演练记录。

**回滚**：在旧路径移除前保留快速切换能力；移除必须单独发布，不与首次全量切换同一版本完成。

## 22. 建议默认配置

```yaml
scheduler:
  affinity:
    explicit_ttl: 60m
    protocol_ttl: 120m
    identity_ttl: 10m
    identity_fallback: true
    escape_pressure: 0.90

  overflow:
    start: 0.70
    full: 0.90
    exit: 0.60
    exit_hold: 30s

  retry:
    max_total_attempts: 4
    max_same_channel_retries: 1
    max_credential_rotations: 1
    max_channel_switches: 2
    max_elapsed: 15s

  circuit:
    window: 30s
    minimum_observations: 20
    open_error_rate: 0.50
    consecutive_failures: 5
    open_duration: 30s
    half_open_max_probes: 1
    close_successful_probes: 2

  capacity:
    lease_ttl: 90s
    lease_refresh: 30s
    max_short_queue_wait: 200ms
    runtime_snapshot_max_age: 15s

  recovery:
    ramp_duration: 10m

  overrides:
    default_ramp_up: 60s
    max_ttl: 24h
    default_allocation_unit: session
    default_apply_scope: within_role
```

这些值是上线初始值，不应在缺少指标的情况下频繁调节。阈值变更必须绑定策略版本，并在变更前后对比目标指标。

## 23. 上线验收标准

### 23.1 正确性

- [ ] 禁用、无能力、越权、熔断和满载渠道不会被执行。
- [ ] 多实例不突破严格硬容量。
- [ ] 已输出流式响应后不会重试。
- [ ] 非幂等模糊送达不会自动重放。
- [ ] 临时策略不能绕过硬过滤。

### 23.2 亲和和分布

- [ ] 同一显式会话在健康低压期间保持渠道稳定。
- [ ] 无会话 ID 时明确采用身份级短 TTL 或无亲和模式。
- [ ] 权重和目标比例在大样本下达到规定容差。
- [ ] 首选高压时只迁移新会话，已有健康会话不发生大规模抖动。

### 23.3 可用性

- [ ] 502/503 等短时故障按预算恢复或 failover。
- [ ] 504 和网络模糊错误遵守可重放性。
- [ ] 渠道故障能够熔断并通过单探测恢复。
- [ ] Redis 故障行为符合严格/弹性容量策略。

### 23.4 操控性

- [ ] 可临时调整渠道的新会话目标比例。
- [ ] 可灰度、排空、暂停和自动过期。
- [ ] 所有操作有版本、原因、操作者和回滚记录。
- [ ] 管理端显示目标比例与实际偏差及无法完成原因。

### 23.5 可观察性

- [ ] 每个逻辑请求可通过 Request ID 还原所有尝试。
- [ ] 每次选择有稳定原因码和策略版本。
- [ ] 看板区分首次分配、全部尝试、成功、Token 和成本比例。
- [ ] 高基数和敏感字段未进入 Prometheus 标签或普通日志。
- [ ] 关键告警完成演练。

## 24. 运维操作参考

### 24.1 临时把渠道调整到 20%

推荐操作：

1. 在模拟器中以 `allocation_unit=session`、`target_share=0.20` 运行最近流量样本。
2. 检查渠道硬容量能否承接预计 Token 和并发。
3. 创建 5 分钟 ramp、30 分钟 TTL 的临时策略。
4. 观察新会话比例、Token 比例、TTFT、错误率和租约拒绝。
5. 偏差较大时先检查亲和存量和硬过滤原因，不要立即反复改权重。
6. 验证结束后手动回滚或等待自动过期。

### 24.2 渠道出现大量 502/503

1. 查看错误是渠道级还是 Provider 级。
2. 查看熔断是否打开、failover 是否成功、备用是否有容量。
3. 不要立即清除全部亲和；当前请求会临时绕开。
4. 确认持续故障后执行 drain 或 pause。
5. 恢复后通过 HALF_OPEN 和 ramp 自动回流。

### 24.3 渠道出现大量 504

1. 区分连接建立超时、上游处理超时和响应读取超时。
2. 查看请求的 Replayability 和 DeliveryState。
3. 对图片、视频和任务提交检查是否存在重复任务风险。
4. 不通过扩大重试次数掩盖上游容量问题。
5. 优先降低新会话比例或排空，再分析 TTFT、总延迟和 Token 分布。

### 24.4 紧急下线渠道

1. 优先使用 `force_drain`，设置明确原因和 TTL。
2. 系统停止新租约并使已有亲和在后续请求迁移。
3. 已经执行中的请求不被强制中断，除非另有安全要求。
4. 观察备用和保底压力，必要时调整容量或目标比例。
5. 恢复时先 canary，再 ramp，不直接恢复满量。

## 25. 审查时需要确认的业务参数

以下参数不影响架构，但必须在阶段 0 由业务和运维确认：

1. 每类模型请求的 Replayability，尤其是工具调用、图片、视频和异步任务。
2. 哪些渠道属于严格容量或昂贵渠道，Redis 故障时必须 fail-closed。
3. 生产最大副本数和每个渠道的并发、Token 速率、目标 TTFT。
4. 默认是否开启身份级回退亲和。
5. 临时策略的最大 TTL、审批规则和操作角色。
6. 目标比例默认按新会话、首次请求还是请求计算；本方案默认新会话。
7. 成功率、P95/P99、TTFT、成本和故障恢复的正式 SLO。

上述参数确认后只需要形成策略配置，不需要修改调度核心算法。
