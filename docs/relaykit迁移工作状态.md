# relay → relaykit 协议转换迁移 · 工作状态

> 记录时间：2026-09-10（上一版 2026-09-09）
> 分支：`feat/relaykit`（基于 main，领先 2 个已提交 commit + 大量未提交工作区改动）
> 状态：**未提交**。全仓 `go build ./...` + `go vet` 通过；relaykit 模块测试全绿；
> relay 侧仅剩既有失败（见 §四）。§5.1 已修，另发现并修复 **2 个本次迁移引入的回归**（见 §3.6）。

---

## 一、本次任务目标

把 relay 层残留的「纯协议转换」代码全部迁进独立模块 `relaykit/`，让 relaykit 成为唯一转换路径，
宿主 `relay/` 只保留 HTTP/SSE/计费/调度等宿主职责。

## 二、开工前定下的 4 个决策（用户已拍板）

| 决策点 | 选定方案 |
|--------|----------|
| 注册表改造 | **允许单侧注册**（请求侧或响应侧可单独为空），反向方向才注册得进去 |
| 跨原生方向表达 | **中枢链式 + 直连覆盖**：请求走 `A→OpenAI→B` 步骤链，响应用直连转换器（保真度高）。<br>后修订：Claude→Gemini 请求侧改为直连（见 §六.11） |
| legacy 兜底策略 | **立即 hard-fail**：不再"解析失败回退旧实现"，legacy 随迁随删 |
| 迁移范围 | 全选：4 个跨原生响应桥 + openai/converter.go 反向方向 + o2c/o2g 双实现清理 + 小方向补全 |

## 三、已完成的工作

### 3.1 relaykit 注册表基建（新增/改造）

| 文件 | 说明 |
|------|------|
| `relaykit/relayconvert/text_converter_registry.go` | 放宽双侧非空约束 → 单侧注册（两侧都空才 panic），只把已配置的一侧写进对应注册表 |
| `relaykit/relayconvert/engine.go`（新增） | `ConvertRequestByID` / `ExecuteRequestConversion`：统一执行直连转换器与 StepConverters 步骤链，调用方无需区分 |
| `relaykit/relayconvert/stream_event.go`（新增） | `StreamEvent{Event, Data, Usage *dto.UsageWithDetails}`：非 OpenAI 客户端方向的通用流式帧封装（Event 非空=事件帧，空=纯 data 帧） |
| `relaykit/relayconvert/errors.go`（新增） | `ErrStatefulResponsesUnsupported` 哨兵（顶层包，宿主可 `errors.Is`） |
| `relaykit/relayconvert/convmeta/responses_stash.go`（新增） | `ResponsesStash` 能力接口：Responses 请求快照 stash/读取 |
| `request_registry.go` / `response_registry.go` | 补齐全部方向的 converter ID 常量（含跨原生、反向、Responses 双向、流式） |

**注意**：`StreamEvent.Usage` 中途从 `*dto.Usage` 升级为 `*dto.UsageWithDetails`（瘦类型会丢缓存明细 → 影响计费），
所有转换器已按新契约适配。

### 3.2 转换器迁移（4 个并行子任务，全部完成且测试通过）

| 子任务 | 产出包 | 内容 |
|--------|--------|------|
| A | `relaykit/relayconvert/internal/oai_chat/` | Claude 入站方向：`ClaudeToOpenAIRequestConverter`、`OpenAIToClaudeResponseConverter`、`OpenAIToClaudeStreamConverter` |
| B | `relaykit/relayconvert/internal/oai_gemini/` | Gemini 入站方向：`GeminiToOpenAIRequestConverter`、`OpenAIToGeminiResponseConverter`、`OpenAIToGeminiStreamConverter` |
| C | `relaykit/relayconvert/internal/oai_responses/`（新包）+ `relaykit/dto/openai_responses.go`（新） | Responses 双向 6 个转换器；Responses DTO 下沉到 relaykit，`relay/dto/openai_responses.go` 改为纯别名文件 |
| D | `internal/claude_gemini/` + `internal/native_responses/`（新包） | 8 个跨原生响应转换器（Claude↔Gemini、Claude/Gemini→Responses，各含流式） |

### 3.3 注册接线

`relaykit/relayconvert/register/register.go` 现注册 **12 个方向**：

- OpenAI 入站 → Claude / Gemini / Ollama(chat)（原有）
- 反向：Claude / Gemini / Responses 入站 → OpenAI 上游（新）
- OpenAI chat → Responses 上游（ChatViaResponses 桥接，新）
- 跨原生：Claude→Gemini、Gemini→Claude、Responses→Claude、Responses→Gemini
  （新；响应侧全部直连，请求侧 Claude→Gemini 已改直连，其余仍为步骤链）

### 3.4 宿主桥接层改造

| 文件 | 说明 |
|------|------|
| `relay/relaykit_bridge/route.go`（**新增**） | **唯一权威方向矩阵**：`KitFormat`（宿主↔relaykit 格式名映射，Responses 命名不同需显式转）、`EffectiveUpstreamFormat`（含 UseResponsesAPI / UpstreamSpeaksResponses 覆盖）、`RequestConverterIDForRoute`、`ResponseConverterIDForRoute`。另在此包 blank-import `register` 触发注册（放桥接包而非某调用方，保证所有 channel 包及其单测都能拿到注册结果） |
| `relay/relaykit_bridge/response.go` | 签名改 4 值 `(converted, usage, handled, err)`，hard-fail 语义；按格式解析上游响应（新增 OpenAI/Responses 两种） |
| `relay/relaykit_bridge/stream.go` | chunkWriter 支持 `StreamEvent`（事件帧/data 帧分派 + usage 明细捕获）；**收尾策略按客户端格式**：OpenAI/Gemini 客户端写 `[DONE]`（OpenAI 另补终止 chunk），Claude/Responses 客户端不写（终止语义由 message_stop / response.completed 承载） |
| `relay/handler/relaykit_bridge.go` | 重写：走统一矩阵、按入站格式解析请求、hard-fail、OpenAI 上游后处理（stream_options / reasoning_effort 注入，均"客户端未显式设置时"语义）、哨兵错误映射 |
| `relay/handler/relay_handler.go` | `convertRequestBody` 改三态处理（handled+err / handled / 未接管走 adaptor） |
| `relay/common/relay_info.go` | `*RelayInfo` 实现 `convmeta.ResponsesStash` |

### 3.5 宿主 legacy 删除（hard-fail 接线）

已改为纯桥接外壳并删除旧转换实现：

- `relay/channel/claude/response.go`（handleNonStreamToOpenAI / handleStreamToOpenAI，删约 270 行流式循环）
- `relay/channel/gemini/response.go`（同上，删约 300 行）
- `relay/channel/ollama/adaptor.go`（chat 非流式+流式，删约 150 行）
- `relay/channel/openai/claude_response.go`（重写为桥接外壳）
- `relay/channel/openai/gemini_response.go`（重写为桥接外壳，保留 `writeOpenAIErrorAsGemini` 等错误映射）
- 4 个跨原生桥 `claude/gemini_bridge.go`、`claude/responses_bridge.go`、`gemini/claude_bridge.go`、`gemini/responses_bridge.go`（由子任务改造，**见待办**）

---

## 四、当前验证状态

```
GOTOOLCHAIN=go1.25.8 go build ./...              ✅ 通过
GOTOOLCHAIN=go1.25.8 go vet ./relay/...          ✅ 通过
cd relaykit && GOWORK=off go test -count=1 ./... ✅ 全绿（14 个包）
GOTOOLCHAIN=go1.25.8 go test ./relay/...         ✅ 仅 relay/helper 既有失败（见 §六.7）
GOTOOLCHAIN=go1.25.8 go test ./internal/...      ⚠️ 3 个既有失败，与本次迁移无关：
    · internal/logic/admin  TestBuiltinRoleDefaultsMatchMigration
      —— 找不到 000020_admin_role_management.sql（迁移已重编号为 000020_v0_2_17_version.sql）
    · internal/dispatchadapter TestProbeFailure_DoesNotDecayHealthEwma —— 健康分 EWMA
    · internal/logic/billing   TestGenerateBillingSummary_TimeRule   —— 账单文案时段行
  （三处工作区均无改动，勿误当回归）
gofmt -l relay/ relaykit/                        ⚠️ 剩 10 个**存量**未格式化文件
  （ali/volcengine converter_test、relaykit dto/audio、goldentest、benchmark_test 等，
    本次迁移触及的 5 个文件已 gofmt -w）
```

---

## 五、重启后的待办

### 5.1 ~~修复 3 个流式错误传播测试~~ ✅ 已完成（2026-09-10）

**上一版留下的坑**：这 3 个测试当时是被「反转断言」改绿的——把「应返回错误」改成
「不返回错误、只查 StreamStatus」。但 `StreamEndReasonError` **不在** `IsPartialStreamEnd()`
集合内（`relay/common/stream_status.go:79`），于是 `DoResponse` 返回 `(usage, nil)`，
落到 `relay_handler.go` 的**成功路径**：`sess.Finish(true)` 让调度 FSM 收到「成功」、
熔断/健康分拿不到上游故障信号，并按正常成功结算（usage 近零）。旧实现是返回 error 的。

**已按原文档倾向的方案 1 修正**：
- `TryConvertStreamViaRelaykit` 签名改三态 `(*common.Usage, bool, error)`；
  `ok=true,err!=nil` = 已写出 SSE 但流以错误/中断结束，调用方原样 `return usage, err`。
- 新增 `streamConvertError`（`relay/relaykit_bridge/stream.go`）做错误分类，
  一律置 `ResponseWritten`（SSE 头与 200 已提交，不得改状态码/换渠道重试）。
- 客户端断开分支直接透出 `common.ErrStreamInterrupted`，9 个调用点不再各自
  读 `IsPartialStreamEnd()` 判定，统一收敛到桥接层。
- 3 个测试断言改回「应返回错误」，并加断分类与收尾事件。

**顺带修正的分类错误**：安全拦截（Gemini `promptFeedback.blockReason`）原先会被映射成
502 上游错误 → 正常渠道因用户发违规提示词被扣健康分乃至熔断。现新增
`relaykit/relayconvert.ErrContentBlocked` 哨兵（6 个 Gemini 出站转换器 wrap 返回），
宿主 `errors.Is` 识别后映射为请求类 4xx。非流式侧同理，新增
`relaykit_bridge.ResponseConvertError` 供 3 个 Gemini 上游非流式站点统一分类。
6 个方向（→Claude / →Responses / →OpenAI × 流式/非流式）均已有测试覆盖。

### 5.2 ~~核对子任务 D 的跨原生桥接线成果~~ ✅ 已核对（结论与担心相反）

`geminiToClaudeResponse` / `buildResponsesBodyFromGemini` **仍在且不能删**：
它们是 Code Assist 强制流式聚合路径（`gemini/adaptor.go:514,520`
`handleCodeAssistAggregatedStream`）复用的非流式装配函数，各文件头注释已写明。
计费 usage 已确认仍从**原始上游响应体**提取。

### 5.3 ~~收尾清理~~ ✅ 已完成（2026-09-10，legacy 删除落地）

删除过程中发现并补齐了 2 处迁移遗漏（见 §七的「删除时补的接线」）。

### 5.4 提交前检查

- `gofmt -l` 本次触及文件已清空（存量 10 个未动，避免污染本次 diff）
- `go vet ./relay/... ./internal/...` ✅
- 提交信息不得包含 AI 工具身份标识（项目规范）
- 建议拆 commit：注册表基建 / 转换器迁移 / 宿主接线 / legacy 删除 /
  **本次新增：流式错误传播 + 安全拦截分类 / Anthropic 端点与 provider 后处理回归修复**

---

## 3.6 本次新发现并修复的 2 个回归（重点，评审必看）

两者同一个根因：**relaykit 的转换矩阵按「协议格式」裁决、与供应商无关**，
命中后 `adaptor.ConvertRequest` 整个不再被调用。而 §3.3 新增的「非 OpenAI 入站 →
OpenAI 上游」三个反向方向，把原本归 adaptor 的路径整片接管走了。

### 回归 A：Anthropic 兼容端点渠道的 Claude 入站双向断链（严重）

ali / zhipu / deepseek / moonshot / volcengine / minimax 这 6 家主协议是 OpenAI，
但对 `RelayModeClaudeMessages` **另挂独立的 Anthropic 兼容端点**
（`/anthropic/v1/messages`、`/apps/anthropic/v1/messages`、`/api/coding/v1/messages`），
Claude 入站时上游**实际说 Claude**：请求体原样发、响应体也是 Claude
（其 `DoResponse` 正是委托给 `claude.Adaptor`）。

但 `helper.ProviderNativeFormat` 只按渠道类型返回单一格式（这几家 → openai），
且 `inboundMatchesChannelNative` 因此判 false 使 `canPassThrough` 也不兜底，于是：
- 请求侧：Claude 体被 relaykit 转成 OpenAI chat，POST 到 Anthropic 端点 → 上游 400；
- 响应侧：返回的 Claude 响应体被当 OpenAI 解析 → 转换失败。

**修复**：新增 `constant.HasNativeClaudeEndpoint(providerType)`，在
`relaykit_bridge.EffectiveUpstreamFormat` 与 `handler.inboundMatchesChannelNative`
两处叠加判定 —— Claude 入站 + 该供应商 ⇒ 有效上游格式 = Claude ⇒ 同格式、矩阵返回空串
⇒ 不转换，回到 adaptor / 直连原路径。

> 登记判据：`GetRequestURL` 对 `ClaudeMessages` 返回了与 chat 端点**不同**的地址。
> 共用同一端点的（aws / baidu_v2 / mistral / openai / xai / xunfei）上游只说 OpenAI，
> 不属于本集合，Claude 入站仍须转换。新增此类供应商时必须同步登记。

### 回归 B：5 个供应商的私有请求后处理被绕过

xai / baidu_v2 / zhipu / ali / deepseek 在格式转换之外还有私有请求适配，
relaykit 只补了通用的 `model` / `stream_options` / `reasoning_effort`：

| provider | 丢失的后处理 | 后果 |
|---|---|---|
| xai | `-search`/`-high`/`-low` 后缀剥离 + `search_parameters`/`reasoning_effort` | 未配映射时 `grok-4-search` 原样发上游 → 400 |
| baidu_v2 | `-search` 后缀剥离 + `web_search` 注入 | 同上 |
| zhipu | `applyGLMCompatibility`（top_p 裁剪、图片前缀剥离）+ 思考参数方言 | GLM 参数越界、思考失效 |
| ali | DashScope 按上游模型剥离 `thinking_budget` | 上游拒绝不支持该参数的模型 |
| deepseek | `thinking` 对象（V3 `type` + R1 `reasoning_effort:"none"`） | `-thinking`/`-nothinking` 静默失效 |

模型名那条最实际：未配映射时 `UpstreamModel = row.ModelName`
（`internal/dispatchadapter/catalog.go:443`）**带着后缀**，relaykit 无条件写
`GetUpstreamModelName()`，而 adaptor 本来会剥掉后缀再写回。

**修复**：新增可选能力接口 `common.RequestPostProcessor`
（`PostProcessConvertedRequest(ctx, info, body) ([]byte, error)`），
`handler.convertRequestBody` **仅在 relaykit 接管时**调用（走 adaptor 那条路径的
`ConvertRequest` 内部已含同一段，重复执行会双写）。5 家各把自己 `ConvertRequest`
的尾段抽成 `postProcessRequest`，与钩子共用同一实现，杜绝两份漂移。

已核对**不受影响**：minimax / moonshot 的 Claude 入站分支只做模型名映射（relaykit 已覆盖）；
其余约 15 个 OpenAI 兼容 provider 的 `ConvertRequest` 无格式外后处理。

---

## 七、legacy 删除完成记录（2026-09-10 已全部落地）

净删约 **6400 行**（git diff --stat：+530 / -6907）。逐项：

| 项 | 结果 |
|----|------|
| `openai/converter.go` o2r 段（`ConvertOpenAIToResponses` + c2r 助手） | ✅ 删除（chat→Responses 请求转换由 relaykit `ConverterOpenAIChatToOpenAIResponses` 接管，handler 桥注入 reasoning_effort） |
| `openai/converter.go` c2o/g2o/r2o（`ConvertClaudeToOpenAI` 等） | ⚠️ **保留**：Ollama 渠道的 Claude/Gemini/Responses 入站走 `ConvertToOpenAI`→OpenAI→Ollama 链（矩阵未注册 Claude→Ollama 方向，属「未迁方向保留」设计） |
| `openai/responses.go` `handleResponsesInboundStream`/`NonStream`（约 600 行） | ✅ 改为 relaykit 桥接壳（**这是迁移漏掉的接线**——转换器早注册了但 openai adaptor 一直在调 legacy 实现）；孤儿助手 `chatCompletionToResponsesResponse`/`extractStreamEmbeddedError` 删除，`extractResponsesRequestEcho` 保留（`BuildResponsesObjectMap` 导出依赖） |
| `claude/converter.go`（446 行）+ `converter_test.go` | ✅ 整文件删除（o2c 请求侧全套） |
| `gemini/converter.go`（686 行）+ `converter_test.go` | ✅ 整文件删除（o2g 请求侧全套） |
| 18 个 adaptor 的 `ConvertToOpenAI` 守卫 | ✅ 删 17 个（死代码：非 OpenAI 入站→OpenAI 上游已全由矩阵接管）；**ollama 保留**（见上） |
| claude/gemini/openai adaptor 的入站格式 switch | ✅ 删除，改为同格式路径 + **hard-fail 守卫**（矩阵意外未接管时显式报错，不静默错格式透传） |
| `deepseek` `convertResponsesToChatForDeepSeek` | ✅ 删除，分支改 hard-fail（relaykit Responses→OpenAI + PostProcess 钩子接管） |
| stale 注释（passthrough/bridge 头注释中的旧函数名） | ✅ 更新为 relaykit 表述 |

**删除时补的接线**（不做这两步直接删会断链）：
1. **openai adaptor responses 入站桥接**：迁移只注册了转换器、没改 adaptor 调用点。已把两个 inbound 处理器改为调 `TryConvertStream/ResponseViaRelaykit`（非 200 透传语义保留，计费仍取上游原始 chat 体口径）。
2. **Responses 客户端流错误收尾**：桥接层在转换器以错误退出且未产出任何终止态事件时补发 `response.failed`（否则 200 SSE 静默结束、codex 等客户端挂起）；转换器已自行收尾的（如 Gemini SAFETY 发 completed 拒答）不追加第二终止事件。相关测试已按新契约改写。

**验证**：`go build ./...` ✅、`go vet ./relay/... ./internal/...` ✅、relaykit 模块全绿 ✅、
relay 测试仅剩 §六.7 既有失败 ✅、触及文件 gofmt 干净 ✅。

## 六、重要设计约束备忘（重启后容易忘）

1. **格式名两侧不同**：宿主 `constant.RelayFormatResponses = "responses"`，relaykit `types.RelayFormatOpenAIResponses = "openai_responses"`。
   跨模块必须走 `relaykit_bridge.KitFormat()`，**不能直接强转**（其余格式字符串值相同可强转）。
2. **计费 usage 一律取上游原始口径**，不从转换后的响应体取。OpenAI 上游方向需置
   `CacheIncludedInPrompt = true`（prompt 含 cached，计费按明细扣减，避免双重计费）。
3. **转换器只能依赖 `convmeta.Meta` 接口**，禁止 import 宿主 `relay/...` 或 GoFrame。
   需要宿主状态时走能力接口（如 `ResponsesStash`）+ 类型断言，断言失败静默降级。
4. **`relay/dto` 是 `relaykit/dto` 的类型别名**（`relaykit_types.go` + `openai_responses.go`），两侧同型，边界零转换。
5. **转换器 ID 元数据方法（ID/From/To/Quality）与注册用的配对 spec ID 不是一回事**：
   注册以 `RegisterTextConverter` 传入的 spec.ID 为准，元数据方法只是自描述。
6. relaykit 是独立 module：测试要 `cd relaykit && GOWORK=off go test ./...`；
   宿主构建要 `GOTOOLCHAIN=go1.25.8`（否则 stdlib 重编译失败）。
7. `relay/helper` 的 `TestSafeUpstreamErrorMessage_*` 失败是**既有问题**（中英文案期望不一致），
   与本次迁移无关，不要误当回归去修。
8. **矩阵按格式裁决、与供应商无关**——这是本次两个回归的共同根因。新增/修改方向时必须自问：
   ① 该供应商对这个 relay mode 是否另有原生端点（→ `constant.HasNativeClaudeEndpoint` 类判定）？
   ② 该供应商的 `ConvertRequest` 里除格式转换外是否还有私有适配
   （→ 实现 `common.RequestPostProcessor`）？漏掉任一条都是**静默**降级或断链。
9. **`ProviderNativeFormat` 是「按渠道类型的单一格式」，不是全部真相**：多协议供应商的
   有效上游格式取决于 (供应商, relay mode)，权威计算在 `relaykit_bridge.EffectiveUpstreamFormat`，
   `handler.inboundMatchesChannelNative`（直连判定）必须与它保持同步，否则直连与转换两条路会对同一
   请求给出不同格式结论。
10. **provider 后处理钩子只在 relaykit 路径调用**：adaptor 路径的 `ConvertRequest` 内部已含同一段，
    两处都调会双写参数。5 家的实现均把尾段抽成 `postProcessRequest` 与钩子共用，改动时不要只改一处。
11. **跨原生请求侧优先直连，不要新增中枢步骤链**：Claude→Gemini 已由
    `internal/claude_gemini/ClaudeToGeminiRequestConverter` 直连。链式（`StepConverters`）会让
    中间格式承载不了的字段静默消失——实测丢 thinking 签名、精确 thinking budget（5000 被放大成 8192）、
    `top_k`、`tool_use.id` 四项，见 `claude_to_gemini_request_test.go` 的
    `TestClaudeToGeminiRequest_PreservesWhatChainLoses`。新增跨原生方向时先写直连转换器，
    链式只作过渡手段。Gemini 的 JSON Schema 关键字白名单在 `internal/shared.CleanGeminiToolParams`，
    OpenAI→Gemini 与 Claude→Gemini 两条路径共用一处，改白名单是两边同时生效。

---

## 八、模型名后缀（虚拟模型）机制移除（2026-09-10）

经用量核实无任何带后缀模型（`-thinking`/`-nothinking`/`-none`/`-minimal`/`-low`/`-medium`/
`-high`/`-xhigh`/`-max`/`-search`）在用，整套「模型名后缀 → 虚拟模型」机制作为技术债务移除。
思考/搜索能力此后一律由**客户端请求参数**表达（`thinking` 对象、`reasoning_effort`、
`reasoning.effort` 等），网关只做协议转换不做能力注入。

### 删除清单

**宿主侧：**
- `relay/helper/thinking.go`（`ParseThinkingSuffix` + `ThinkingInfo`）整文件删
- `resolveRelayModel` 简化为纯目录校验（不再「字面优先 + 剥后缀回退」两段查找）
- `RelayInfo` 删 `ThinkingEnabled`/`ThinkingDisabled`/`ReasoningEffort` 三字段
  及 `Get/SetReasoningEffort`（convmeta.Meta 相应方法一并删，本就零调用方）
- 各注入器删除：claude `injectClaudeThinking/Effort`、gemini `injectGeminiThinking`、
  openai `injectReasoningEffort/injectResponsesReasoning`、deepseek 三个 inject* +
  `isDeepSeekV4Model`、zhipu `injectThinkingParams`
- `canPassThrough` 删 thinking 后缀排除条件；handler 桥删两处 effort 注入（stream_options 保留）
- xai/baidu_v2 的 `-search`/`-high`/`-low` 剥离删除，两家 + deepseek 的
  `RequestPostProcessor` 钩子实现删除（剩余职责 relaykit 已覆盖）；**钩子接口保留**，
  现存实现方仅 zhipu（GLM 参数兼容）与 ali（DashScope 参数裁剪）

**relaykit 侧：**
- `internal/shared/thinking_adapter.go`、`reasoning/` 包整体删除
- `convmeta.Options` 删 `PreserveThinkingSuffix`；Claude/Gemini Options 删
  `ThinkingAdapter*` 字段（宿主侧写死 TODO 的配置一并清）
- oai_chat 请求转换器不再解析上游模型名后缀

### 保留（勿误删）

- **客户端参数转换线**：`c2oConvertThinkingToReasoningEffort` / `g2oConvertThinkingConfig`
  （Claude thinking 对象 / Gemini thinkingConfig → chat `reasoning_effort` 字段）——
  这是协议转换不是后缀注入，宿主 converter.go 与 relaykit oai_chat 各一份
- `bil_usage_logs.reasoning_effort` 列与 `UsageRecord.ReasoningEffort` 字段保留
  （dao 生成 + 历史数据），此后恒为空串
- `RelayInfo.BaseModelName` 字段保留（responses 路由记录在用），现恒等于 OriginModelName

### 行为变化

- 请求带旧后缀名（如 `gpt-4o-thinking`）→ 目录查不到 → **404 model not found**
  （原为剥后缀回退命中 + 注入思考参数）
- 名字本身含 `-max` 等字样的真实目录模型（qwen-max）不受影响（本就字面优先）
