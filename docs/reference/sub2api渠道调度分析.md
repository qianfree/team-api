# sub2api 渠道调度策略分析

> **用途**：参考项目 `sub2api`（Wei-Shaw/sub2api，Gin + ent ORM + Redis 实现）的渠道调度、亲和性、优先级、失败重试 / failover 机制的实现级分析，供 team-api 的 `relay/scheduler/`、`internal/logic/relay/`、`chn_channel_affinities`、`chn_health_scores` 等模块设计与对照参考。
>
> **重要前提**：sub2api 是「订阅转 API」(subscription-to-API)，它调度的基本单位是**账户(account)**——即一个个上游订阅账户（Claude Pro OAuth、ChatGPT Plus、Gemini OAuth 等），而不是 new-api 意义上的「渠道」。本文按提问习惯把 account 对应为「渠道」。
>
> **路径约定**：本文所有 `file:line` 引用均相对于 **`sub2api/backend/`** 目录。
> **分析时间**：2026-07-30。

---

## 目录

1. [两套调度路径](#一两套调度路径)
2. [多个渠道怎么判断优先级](#二多个渠道怎么判断优先级)
3. [怎么做渠道亲和性](#三怎么做渠道亲和性)
4. [失败重试与 failover（四重有界）](#四优先渠道失败会一直往下面的渠道重试吗不会四重有界)
5. [与 new-api 的对比](#五与-new-api-的对比)
6. [对 team-api 的借鉴提示](#六对-team-api-的借鉴提示)

---

## 一、两套调度路径

sub2api 有**两套并行的调度路径**，优先级机制完全不同，必须分开讲：

| 路径 | 文件 | 适用平台 | 优先级机制 |
|------|------|---------|-----------|
| **通用 gateway 调度** | `internal/service/gateway_scheduling.go` | Claude / Gemini / Bedrock / Antigravity 等 | **严格优先级**（数值小优先）+ LRU 兜底 |
| **OpenAI 高级调度器** | `internal/service/openai_account_scheduler.go` | OpenAI 兼容账户 | **多维加权评分**（优先级只是其中一个软因子） |

入口分别由 `internal/handler/gateway_handler.go` 的主转发循环，和 `selectAccountWithScheduler`（`openai_account_scheduler.go:2038`）调用。

---

## 二、多个渠道怎么判断优先级

### 路径 A：通用调度 —— 严格优先级 + LRU

`selectAccountForModelWithPlatform`（`gateway_scheduling.go:1757`）遍历候选账户，选最优的那个（`gateway_scheduling.go:1863`）：

```go
if acc.Priority < selected.Priority {
    selected = acc                       // 数值越小，优先级越高
} else if acc.Priority == selected.Priority {
    switch {
    case acc.LastUsedAt == nil && selected.LastUsedAt != nil:
        selected = acc                   // 从未用过的优先
    ...
    default:
        if acc.LastUsedAt.Before(*selected.LastUsedAt) {
            selected = acc               // 同优先级 → 最久未使用(LRU)
        }
    }
}
```

**结论**：严格分层（低数值优先），同层用 **LRU（最久未使用）** 轮转，OAuth 类型还有额外偏好。**没有加权随机**——这是和 new-api 最大的差异之一。

> ⚠️ 优先级数值语义和 new-api **相反**：sub2api 是**越小越优先**，new-api 是越大越优先。

### 路径 B：OpenAI 高级调度器 —— 多维加权评分

这是 sub2api 最精华的部分。`buildOpenAIAccountLoadPlan`（`openai_account_scheduler.go:791`）给每个候选算一个 `score`（`openai_account_scheduler.go:955`）：

```go
item.score = weights.Priority * priorityFactor +        // 优先级(归一化)
             weights.Load      * loadFactor +           // 负载率(越空闲越高分)
             weights.Queue     * queueFactor +          // 等待队列长度
             weights.ErrorRate * errorFactor +          // EWMA 错误率(越健康越高分)
             weights.TTFT      * ttftFactor +           // 首 token 延迟(越快越高分)
             weights.Reset     * resetFactor +          // 订阅窗口"不用就作废"(use-it-or-lose-it)
             weights.QuotaHeadroom * quotaHeadroomFactor +  // 配额余量
             weights.UpstreamCost  * (upstreamCostFactor)   // 上游成本
// + 粘性加分(weights.Previous / weights.SessionSticky)
```

其中 `priorityFactor = 1 - (priority - minPriority)/(maxPriority - minPriority)`（`scheduler.go:924`）——**优先级数值越小，因子越大，得分越高**，但它是**和负载 / 错误率 / 延迟平等竞争的一个软因子**，不是绝对分层。`weights.*` 是可配置的权重，运营可以调整哪个维度更重要。

然后取 **Top-K**（配置项）个候选，用 `buildOpenAIWeightedSelectionOrder`（`scheduler.go:736`）做**加权随机排序**——把得分平移到正区间当权重（`(score - minScore) + 1.0`，避免单一高分账户垄断），用**从请求派生的确定性种子**（`deriveOpenAISelectionSeed`）做随机，保证同会话稳定。

**健康度怎么来**：EWMA（`updateEWMAAtomic`，`scheduler.go:214`）——每次请求结果经 `ReportResult`（`scheduler.go:1717`）反馈，平滑更新错误率和 TTFT（首 token 时间），`openAIAccountRuntimeStats` 在内存里维护。

### 候选排序的 tiebreaker

即便在 OpenAI 调度器里，`isOpenAIAccountCandidateBetter`（`scheduler.go:630`）的字典序也是：`score` → `Priority`（小优先）→ `LoadRate`（低优先）→ `WaitingCount`（少优先）→ `ID`。优先级只在 score 打平时才起决定作用。

---

## 三、怎么做渠道亲和性

sub2api 的亲和性是**会话级粘性**，目的是把同一会话钉在同一账户（上游订阅账户的对话状态 / KV 缓存是账户私有的）。主要两套机制，在 `Select`（`scheduler.go:368`）里按优先级依次尝试：

### 1. PreviousResponseID 粘性（最强）

OpenAI Responses API 的 `previous_response_id` 指向的响应状态只存在处理它的那个账户上。`Select` 一进来就先查（`scheduler.go:380`）：有 `previous_response_id` → 直接路由到当初处理它的账户。命中后还会 `BindStickySession` 把会话也绑过去（`scheduler.go:408`）。

### 2. SessionHash 粘性会话

`selectBySessionHash`（`scheduler.go:454`）：

- 从请求信号派生一个 `sessionHash`，到 Redis 查 `getStickySessionAccountID`（`internal/service/openai_sticky_compat.go:122`）拿到绑定的账户。
- 命中后仍要层层校验：未被排除、可调度、平台匹配、请求兼容、传输兼容，再从 DB 复核（`recheckSelectedOpenAIAccountFromDB`）。
- **粘性逃逸（sticky escape）**：`shouldEscapeStickyAccount`（`scheduler.go:574`）——如果该账户的 EWMA 错误率或 TTFT 超阈值，**主动放弃粘性**，降级到负载均衡选号（`scheduler.go:502`）。这是 sub2api 区别于简单粘性的关键：粘性不是死绑，健康度恶化会自动跳船。
- 粘性会话有 TTL（`refreshStickySessionTTL`），账户变不可调度 / 换模型 / 换平台时自动清除（`deleteStickySessionAccountID`）。

> ⚠️ `auth_identity_channels` 表（`ent/schema/auth_identity_channel.go`）**不是路由亲和**——它是 OAuth 身份到 provider channel 的标识绑定（管理后台 `BindUserAuthIdentityChannel`），别和调度混淆。

---

## 四、优先渠道失败，会一直往下面的渠道重试吗？——不会，四重有界

这是 sub2api 设计得最细致的地方。重试分**三个层次**，每层都有独立上限，通过 `FailoverState`（`internal/handler/failover_loop.go`）状态机协调：

### 第 1 层：同账户 HTTP 重试（转发层）

`internal/service/gateway_forward.go:366` 的 `for attempt := 1; attempt <= maxRetryAttempts` 循环：

- **`maxRetryAttempts = 5`**（`gateway_forward.go:24`）
- **`maxRetryElapsed = 10s`** 总耗时上限（`gateway_forward.go:32`）
- 指数退避：`retryBaseDelay = 300ms`，`retryMaxDelay = 3s`（`gateway_forward.go:55`）
- 这层主要处理可修复错误（如 thinking block 签名错误 → 过滤后原账户重试）

### 第 2 层：同账户业务重试（失败可重试错误）

`FailoverState.HandleFailoverError`（`failover_loop.go:68`）里，对 `RetryableOnSameAccount` 的临时错误（如 pool mode 的可重试状态码）：

- **`maxSameAccountRetries = 3`**（`failover_loop.go:36`），间隔 `500ms`
- 用尽后 `TempUnscheduleRetryableError` **临时封禁**该账户（`failover_loop.go:110`）

### 第 3 层：跨账户 failover（换渠道）

当错误满足 `shouldFailoverUpstreamError`（`gateway_forward.go:46`）——即 **401 / 403 / 429 / 529 或 ≥500**——触发换账户：

- 受 **`MaxAccountSwitches`** 限制，默认 **10 次**（Gemini 默认 **3 次**，`internal/handler/gateway_handler.go:81-82`）
- 每次把失败账户加入 `FailedAccountIDs` 排除集，`SwitchCount++`，达到上限返回 `FailoverExhausted` → 给客户端报错（`failover_loop.go:117`）
- Antigravity 平台换号有线性递增延时（`failover_loop.go:131`）

### 关键安全阀

- **流式已写入则禁止 failover**：`gateway_handler.go:888` 检查 `c.Writer.Size()` 变化，SSE 已经吐给客户端就不能再换账户（否则流拼接腐化）。
- **客户端断开立即终止**：`ctx.Err() != nil` → `FailoverCanceled`（`failover_loop.go:78`），不会对着已取消的 context 空转误报 502。
- **单账户分组 503 特例**：`HandleSelectionExhausted`（`failover_loop.go:148`）——所有候选都被排除时，若是 503 容量耗尽，会**清空排除列表 + 退避 2s 后重选**，给恢复中的账户第二次机会。

**总结**：不会无限重试。换账户最多 `MaxAccountSwitches` 次（默认 10），叠加同账户 3 次业务重试 + 5 次 / 10s HTTP 重试，且流式响应一旦开始就锁死不再换。

### failover 判定函数速查

| 函数 | 位置 | 判定 |
|------|------|------|
| `shouldRetryUpstreamError` | `gateway_forward.go:35` | OAuth 账号仅 403 重试；API Key 账号看 `ShouldHandleErrorCode` |
| `shouldFailoverUpstreamError` | `gateway_forward.go:46` | 401 / 403 / 429 / 529 或 ≥500 → 换账户 |
| `retryBackoffDelay` | `gateway_forward.go:55` | 指数退避 300ms → 3s 上限 |
| `HandleFailoverError` | `failover_loop.go:68` | 同账户重试 → 临时封禁 → 换账户计数 |
| `HandleSelectionExhausted` | `failover_loop.go:148` | 候选耗尽时 503 退避重试 |

---

## 五、与 new-api 的对比

> 完整的 new-api 分析见同目录 [`new-api-channel-scheduling-analysis.md`](./new-api-channel-scheduling-analysis.md)。

| 维度 | new-api | sub2api |
|------|---------|---------|
| 调度单位 | 渠道（channel） | 账户（account / 订阅） |
| 优先级语义 | 数值**大**优先，严格分层 | 数值**小**优先；通用路径严格分层，OpenAI 路径是软因子 |
| 层内分配 | 加权随机（weight） | 通用路径 LRU；OpenAI 路径 Top-K 加权随机（评分当权重） |
| 健康感知 | 仅 auto-ban（401 禁用） | **EWMA 错误率 + TTFT 实时健康度**，纳入评分 + 粘性逃逸 |
| 亲和性 | 规则引擎（model / path / UA + KeySource），**为 prompt cache** | 会话哈希 + previous_response_id，**为对话状态 / KV**；带健康度逃逸 |
| 重试上限 | `RetryTimes`（默认 **0**，不重试！） | 换账户默认 **10**（Gemini 3）+ 同账户 3 + HTTP 5 次 / 10s |
| 重试计数 vs 优先级 | retry 当优先级层下标（巧妙但隐晦） | 完全解耦，独立计数 |
| 流式安全 | 未特殊处理 | 流式已写则禁止 failover |

---

## 六、对 team-api 的借鉴提示

1. **EWMA 健康度评分**（`openai_account_scheduler.go`）是 sub2api 最值得借鉴的——把错误率、首 token 延迟、负载、队列做成归一化因子加权求和，比 new-api 的纯优先级 + 权重精细得多。team-api 的 `chn_health_scores` 表可以做成实时 EWMA 而非离散分数。
2. **粘性逃逸机制**（`shouldEscapeStickyAccount`）——粘性不等于死绑，健康度恶化自动跳船。team-api 的亲和性若做，务必加这层，否则一个坏渠道会粘死一批会话。
3. **重试层次解耦**——sub2api 把「同账户 HTTP 重试 / 同账户业务重试 / 跨账户 failover」分成三个独立有界层次，而 new-api 是混在一起的 `retry` 计数。team-api 设计 `relay/scheduler/` 时建议学 sub2api 的分层。
4. **流式安全阀**——SSE 已写入禁止 failover，这个细节 new-api 没有，team-api 做流式代理必须考虑。
5. **注意优先级数值语义不统一**——new-api 大优先，sub2api 小优先。team-api 自己实现时要统一，别两套参考代码看混了。

### 关键源码索引

| 关注点 | 文件:函数 |
|--------|----------|
| OpenAI 调度主入口 | `internal/service/openai_account_scheduler.go:368` — `Select` |
| 多维评分计算 | `internal/service/openai_account_scheduler.go:791` — `buildOpenAIAccountLoadPlan` |
| 候选打分公式 | `internal/service/openai_account_scheduler.go:955` — `item.score = ...` |
| Top-K 加权随机序 | `internal/service/openai_account_scheduler.go:736` — `buildOpenAIWeightedSelectionOrder` |
| EWMA 健康度 | `internal/service/openai_account_scheduler.go:214` — `updateEWMAAtomic`；`:225` — `report` |
| 通用调度（严格优先级 + LRU） | `internal/service/gateway_scheduling.go:1757` — `selectAccountForModelWithPlatform` |
| 优先级比较 | `internal/service/gateway_scheduling.go:1863` — `acc.Priority < selected.Priority` |
| PreviousResponse 粘性 | `internal/service/openai_account_scheduler.go:380` |
| SessionHash 粘性 | `internal/service/openai_account_scheduler.go:454` — `selectBySessionHash` |
| 粘性逃逸 | `internal/service/openai_account_scheduler.go:574` — `shouldEscapeStickyAccount` |
| 粘性会话缓存 | `internal/service/openai_sticky_compat.go:122` — `getStickySessionAccountID` |
| 同账户 HTTP 重试 | `internal/service/gateway_forward.go:366` — retry loop |
| failover 判定 | `internal/service/gateway_forward.go:35/46` — `shouldRetryUpstreamError` / `shouldFailoverUpstreamError` |
| failover 状态机 | `internal/handler/failover_loop.go` — `FailoverState` / `HandleFailoverError` / `HandleSelectionExhausted` |
| 跨账户循环 + 换号上限 | `internal/handler/gateway_handler.go:586` — 主循环；`:81` — `maxAccountSwitches` |
