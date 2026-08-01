# new-api 模型调度策略分析

> **用途**：参考项目 `new-api`（QuantumNous/new-api，Gin + GORM 实现）的渠道调度、亲和性、优先级、失败重试机制的实现级分析，供 team-api 的 `relay/scheduler/`、`internal/logic/relay/`、`chn_channel_affinities` 等模块设计与对照参考。
>
> **路径约定**：本文所有 `file:line` 引用均相对于 **new-api 参考项目根目录**。
> **分析时间**：2026-07-30。

---

## 目录

1. [核心数据结构：三层映射](#一核心数据结构三层映射)
2. [模型命中多个渠道时怎么分配](#二模型命中多个渠道时怎么分配先按优先级分层层内加权随机)
3. [优先级与权重怎么设置](#三优先级与权重怎么设置)
4. [渠道亲和性（Channel Affinity）](#四渠道亲和性channel-affinity规则驱动默认关闭为-prompt-cache-服务)
5. [失败重试机制](#五优先渠道失败了会一直重试剩下的渠道吗不会有界且多重门控)
6. [调度全流程图](#六调度全流程图)
7. [对 team-api 的借鉴提示](#七对-team-api-的借鉴提示)

---

## 一、核心数据结构：三层映射

new-api 把「哪个分组(group)的哪个模型(model)能由哪些渠道(channel)服务」预计算成一张内存映射，定时从 DB 同步（`model/channel_cache.go:19`）：

```go
var group2model2channels map[string]map[string][]int // enabled channel
```

启动时 `InitChannelCache`（`channel_cache.go:26`）把所有 `enabled` 渠道按 `group → model → []channelId` 建好索引，并且**在每个 model 的渠道列表里按 `Priority` 降序排好**（`channel_cache.go:70-77`）。这是后面「优先级分层」的基础。

每个渠道有两个关键字段（`model/channel.go:483`）：

- **`Priority`（int64）** —— 决定**优先级层（先试哪一层）**
- **`Weight`（int）** —— 决定**同一层内被选中的概率（权重）**

---

## 二、模型命中多个渠道时怎么分配：先按优先级分层，层内加权随机

核心函数 `GetRandomSatisfiedChannel(group, model, retry, requestPath)`（`model/channel_cache.go:114`）。算法分两步。

### 第 1 步：确定本次落在哪个优先级层（`retry` 当层索引用）

```go
// channel_cache.go:143-160
uniquePriorities := ...                          // 去重该 model 的所有优先级
sort.Sort(sort.Reverse(...))                     // 降序
if retry >= len(uniquePriorities) {
    retry = len(uniquePriorities) - 1            // 越界则钳制到最低层
}
targetPriority := sortedUniquePriorities[retry]  // retry=0→最高层, retry=1→次高层 ...
```

> **关键点：`retry` 参数直接当作优先级层的下标。** 第 0 次尝试取最高优先级层，失败重试到 `retry=1` 取下一层…… 这意味着「重试次数」和「优先级层数」是同一个计数器，不是两个独立维度。

### 第 2 步：在该层内做加权随机

```go
// channel_cache.go:162-208
for 渠道 in targetPriority层 {
    sumWeight += channel.GetWeight()
}
// 平滑处理
if sumWeight == 0 {            // 全是 weight 0 → 每个等权（有效权重 100）
    sumWeight = len*100; smoothingAdjustment = 100
} else if sumWeight/len < 10 { // 平均权重 < 10 → 放大 100 倍，减少精度偏差
    smoothingFactor = 100
}
randomWeight := rand.Intn(totalWeight)
for 渠道 in 层 {
    randomWeight -= 权重; if randomWeight < 0 { return 该渠道 }
}
```

**结论：分配策略 = 严格的优先级分层 + 层内加权随机。** 不会跨层混合；高优先级层只要有渠道，就一定先用完它才会降级。

DB 路径（关闭内存缓存时）走 `GetChannel`（`model/ability.go:108`），`getPriority`（`ability.go`）和 `getChannelQuery`（`ability.go:93`）用 SQL 做完全等价的事：`MAX(priority)` 子查询定位层，`weight DESC` 排序后 `weight+10` 加权随机——逻辑一致。

### 路径过滤（Advanced Custom 渠道）

`filterChannelsByRequestPathAndModel`（`channel_cache.go:216`）：只有 type 58（Advanced Custom）渠道会按 `requestPath + model` 过滤，其它类型一律放行。这允许同一渠道的不同路由服务于不同路径。

---

## 三、优先级与权重怎么设置

在管理后台编辑渠道时直接填 `优先级` 和 `权重` 两个字段，写入 `abilities` 表（`group, model, channel_id, priority, weight, enabled`）。

**设计语义**：

| 场景 | 配置方式 |
|------|---------|
| 主备渠道（A 挂了才用 B） | A 设高优先级、B 设低优先级 |
| 负载均衡（多渠道分流） | 相同优先级，用 **weight** 控制比例（weight 2:1 ≈ 2/3 vs 1/3 流量） |

weight 的平滑（`+10` 或 `×100`）保证 weight 配得很小（甚至 0）时仍能正常随机，不会因权重过小导致选不出来。

---

## 四、渠道亲和性（Channel Affinity）：规则驱动、默认关闭、为 prompt cache 服务

这是 new-api 相对 one-api 的**重要增强**，不是默认开的。代码在 `service/channel_affinity.go`，配置在 `operation_setting.GetChannelAffinitySetting()`。

**它解决的问题不是「会话粘性」，而是「上游 prompt cache 命中率」。** 把同一个用户/key 固定路由到同一渠道，让上游供应商（OpenAI/Anthropic）的 KV/prompt 缓存能命中，省钱。所以代码里专门统计 `cached_tokens`、`prompt_cache_hit_tokens`（`channel_affinity.go:742-877`）。

### 它是规则匹配，不是无脑粘

管理员配多条规则，每条规则含：

- `ModelRegex` / `PathRegex` / `UserAgentInclude` —— 命中条件
- `KeySources` —— 亲和性的「主键」取值来源，可从 `context_int`、`context_string`、`request_header`、`gjson`（请求体 JSON path）四种地方取（`channel_affinity.go:289-335`）
- `TTLSeconds`、`SkipRetryOnFailure`、`ParamOverrideTemplate`
- `IncludeRuleName` / `IncludeModelName` / `IncludeUsingGroup`（决定是否参与缓存 key）

缓存 key = `namespace:ruleName(:model)(:group):affinityValue`（`channel_affinity.go:337-350`），`affinityValue` 还会算个 SHA1 指纹存日志，不泄露原值。

### 分发流程（`middleware/distributor.go:105-133`）

在随机选渠道**之前**先查亲和缓存：

```go
if preferredChannelID, found := service.GetPreferredChannelByAffinity(...); found {
    // 命中亲和缓存 → 仍要校验：渠道存在 + 启用 + 支持该 requestPath
    if 校验通过 { 直接用 preferred; MarkChannelAffinityUsed(...) }
    if !affinityUsable && !ShouldKeepChannelAffinityOnChannelDisabled() {
        ClearCurrentChannelAffinityCache(c) // 渠道挂了就清缓存，别死粘
    }
}
```

### 写入

`c.Next()` 返回且 HTTP 状态 `< 400`（成功）时，`RecordChannelAffinity`（`channel_affinity.go:713`）把「用到的渠道 ID」按 TTL 写回缓存。注意有个 `SwitchOnSuccess` 开关——可记录真正成功的渠道而非初选渠道。

### 存储

两层 `HybridCache`——Redis（多实例共享）+ 内存 LRU（`hot.HotCache`，容量默认 10 万），可单条 / 按规则 / 全量清空（管理接口 `controller/channel_affinity_cache.go`）。

---

## 五、优先渠道失败了，会一直重试剩下的渠道吗？——不会，有界且多重门控

不会无限重试。重试循环在 `controller/relay.go:194`：

```go
for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
    channel, _ := getChannel(c, relayInfo, retryParam) // 内部调 CacheGetRandomSatisfiedChannel(retry)
    ...
    newAPIError = relayHandler(c, relayInfo)           // 转发
    if newAPIError == nil { return }                   // 成功
    processChannelError(...)                            // 记日志 + 可能自动禁用
    if !shouldRetry(c, newAPIError, common.RetryTimes-retryParam.GetRetry()) { break }
}
```

### 1. 重试上限 = `common.RetryTimes`，**默认是 0（完全不重试）**

`common/constants.go:133: var RetryTimes = 0`。`RetryTimes=0` 时 for 循环只跑一次，失败即返回。**必须管理员把 `RetryTimes` 调成 >0 才会跨渠道重试**（可在系统设置改，`model/option.go:542`）。

### 2. 重试次数 = 优先级层数，不是渠道个数

因为 `retry` 同时是优先级层下标（见第二节）。所以「能重试几次」实际是 `min(RetryTimes, 优先级层数)`：

| 配置 | 行为 |
|------|------|
| 3 渠道同层 + RetryTimes=3 | 同一层加权随机 3 次（可能命中不同渠道，也可能重复命中同一个） |
| 3 渠道分 3 层 + RetryTimes=3 | retry0→层1、retry1→层2、retry2→层3，逐层降级 |
| `retry >= 优先级层数` | **钳制到最低层**，继续在最低层加权随机，直到耗尽 RetryTimes |

### 3. 哪些错误才重试 —— `shouldRetry`（`relay.go:328`）多重判定

| 判定 | 行为 |
|------|------|
| 渠道连接级错误（`IsChannelError`） | **重试** |
| `IsSkipRetryError`（请求体错误等不可重试错误） | **停止** |
| 剩余次数 `<= 0` | 停止 |
| 命中 `specific_channel_id`（token 锁定指定渠道） | **停止**（不跨渠道） |
| 亲和性规则设了 `SkipRetryOnFailure` | **停止**（`ShouldSkipRetryAfterChannelAffinityFailure`，`relay.go:332`）——为保 prompt cache，宁可在亲和渠道上失败也不换渠道 |
| 按 HTTP 状态码 | 见下 |

**状态码门控**（`setting/operation_setting/status_code_ranges.go`）：

- 默认**会重试**：`1xx`、`3xx`、`4xx`（除 400/408）、`5xx`（除 504/524）
- **永不重试**：`504`、`524`（网关超时，重试也没用）、2xx（成功）
- 管理员可改 `AutomaticRetryStatusCodeRanges`

### 4. 失败渠道会被自动禁用（auto-ban）

`processChannelError`（`relay.go:360`）里：若 `ShouldDisableChannel(err) && AutoBan`，异步调 `service.DisableChannel`。默认 `AutomaticDisableStatusCodeRanges = {401}`——**401（密钥失效）会自动禁用该渠道**，并从 `group2model2channels` 缓存里移除（`CacheUpdateChannelStatus`，`channel_cache.go:271`），后续请求就不会再选到它。

### 5. "auto" 分组（多分组）：每组用完所有优先级才切下一组

`service/channel_select.go:84` 的 `CacheGetRandomSatisfiedChannel` 处理 `TokenGroup == "auto"`：遍历多个分组，**每个分组穷尽自己的优先级层后才换下一个分组**，同样受全局 `RetryTimes` 总量约束（详细示例见 `channel_select.go:70-83` 注释）。

---

## 六、调度全流程图

```
请求进来
  │
  ▼
Distribute 中间件 (middleware/distributor.go)
  │
  ├─ 指定 channel_id? ──是──► 固定渠道，失败不跨渠道重试
  │
  ├─ ① 先查亲和缓存 GetPreferredChannelByAffinity
  │      命中且渠道可用 → 直接用该渠道（为 prompt cache 命中）
  │
  └─ ② 未命中 → CacheGetRandomSatisfiedChannel
         │
         ├─ GetRandomSatisfiedChannel(group, model, retry=0, path)
         │     retry=0 → 最高优先级层 → 层内 Weight 加权随机
         │
         ▼
      转发 relayHandler
         │
      成功(<400) → RecordChannelAffinity 写回亲和缓存 → 返回
         │
      失败 → processChannelError（可能 auto-ban，如 401）
         │
      shouldRetry?
         ├─ 否（状态码不重试 / 越界 / 亲和 skip-retry）→ break，返回错误
         └─ 是 → retry++，回到 ②
              retry=1 → 次优先级层 → 加权随机 …… 直到 retry > RetryTimes
```

---

## 七、对 team-api 的借鉴提示

team-api 的 `relay/scheduler/`（调度、亲和性、重试）和 `internal/logic/relay/`、`chn_channel_affinities` 表正好对应这套东西。new-api 的几个设计值得参考，也有坑要避开：

1. **「retry 当优先级层下标」是个很巧的复用**，但语义不直观——很多人以为 `RetryTimes` 是「重试几个渠道」，其实是「降几层优先级」。team-api 可以考虑**显式分离**这两个维度更清晰。
2. **亲和性绑定 prompt cache 目的**而非会话粘性，这个定位很准，值得直接借鉴——`chn_channel_affinities` 表可以参考它的规则引擎（model/path/UA 匹配 + 多 KeySource + TTL）。
3. **默认 `RetryTimes=0`** 这个默认值偏保守，部署时要注意——team-api 若想要高可用，默认值应该给个合理的非零值。
4. **401 自动 auto-ban + 状态码可配置重试范围**，这套失败处理很实用，建议移植。

### 关键源码索引

| 关注点 | 文件:函数 |
|--------|----------|
| 内存映射 + 定时同步 | `model/channel_cache.go` — `InitChannelCache`, `SyncChannelCache` |
| 渠道选择（内存路径） | `model/channel_cache.go:114` — `GetRandomSatisfiedChannel` |
| 渠道选择（DB 路径） | `model/ability.go:108` — `GetChannel` |
| 优先级层定位（DB 路径） | `model/ability.go` — `getChannelQuery`, `getPriority` |
| 优先级 / 权重访问器 | `model/channel.go:483` — `GetPriority`, `GetWeight` |
| auto 分组 + 跨组重试 | `service/channel_select.go` — `CacheGetRandomSatisfiedChannel` |
| 亲和性全套 | `service/channel_affinity.go` — `GetPreferredChannelByAffinity`, `RecordChannelAffinity`, `MarkChannelAffinityUsed` |
| 分发中间件 | `middleware/distributor.go` — `Distribute` |
| 重试循环 + 判定 | `controller/relay.go:194` — relay loop；`relay.go:328` — `shouldRetry` |
| 状态码重试 / 禁用范围 | `setting/operation_setting/status_code_ranges.go` |
| 渠道状态缓存更新 / auto-ban | `model/channel_cache.go:271` — `CacheUpdateChannelStatus` |
