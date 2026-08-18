# 大模型 Usage 字段与缓存 Token 语义规范

> **文档用途**：四大主流 AI 协议（OpenAI Chat/Responses、Claude、Gemini）的 usage 字段规格、缓存 token 语义、计费换算公式，以及项目代码适配指南。
>
> **调研日期**：2026-08-16 ~ 2026-08-17（基于官方文档联网核验）
>
> **覆盖范围**：OpenAI Chat Completions、OpenAI Responses、Claude Messages API、Gemini GenerateContent API

---

## 一、核心结论：缓存 Token 语义差异

四家协议中**三家是「包含」口径，Claude 是唯一的「排除」口径**：

| 协议 | 顶层输入字段 | 缓存 token 是否含在内 | 未缓存输入怎么得到 |
|---|---|---|---|
| OpenAI Chat Completions | `prompt_tokens` | ✅ **包含**，`cached_tokens` 是它的子集 | `prompt_tokens - cached_tokens - cache_write_tokens` |
| OpenAI Responses | `input_tokens` | ✅ **包含**，`cached_tokens` 是它的子集 | `input_tokens - cached_tokens - cache_write_tokens` |
| **Claude Messages** | `input_tokens` | ❌ **排除**，缓存是独立并列字段 | `input_tokens` 本身就是未缓存部分 |
| Gemini GenerateContent | `promptTokenCount` | ✅ **包含**，`cachedContentTokenCount` 是它的子集 | `promptTokenCount - cachedContentTokenCount` |

**Claude 总输入的计算方式（与其他三家不同）**：

```text
Claude 总输入 token = input_tokens + cache_read_input_tokens + cache_creation_input_tokens
```

官方原文（Anthropic）：
> "Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`."

> "The `input_tokens` field represents only the tokens that come after the last cache breakpoint in your request - not all the input tokens you sent."

---

## 二、各协议详细字段规格

### 2.1 OpenAI Chat Completions (`/v1/chat/completions`)

**顶层字段**：

| 字段 | 说明 |
|---|---|
| `prompt_tokens` | 总输入 token，**包含**缓存命中部分 |
| `completion_tokens` | 总输出 token，**包含** reasoning tokens |
| `total_tokens` | = prompt_tokens + completion_tokens |

**输入明细（`prompt_tokens_details`）**：

| 字段 | 说明 |
|---|---|
| `cached_tokens` | 缓存命中读到的输入 token（`prompt_tokens` 的子集） |
| `cache_write_tokens` | 本次写入缓存的 token（`prompt_tokens` 的子集） |
| `audio_tokens` / `text_tokens` / `image_tokens` | 按模态细分（`prompt_tokens` 的子集） |

**输出明细（`completion_tokens_details`）**：

| 字段 | 说明 |
|---|---|
| `reasoning_tokens` | 推理 token（`completion_tokens` 的子集） |
| `audio_tokens` / `text_tokens` | 按模态细分（`completion_tokens` 的子集） |
| `accepted_prediction_tokens` / `rejected_prediction_tokens` | Predicted Outputs 功能相关 |

**缓存计费**：
- 命中部分按折扣价计费（gpt-4o 为 0.5×）
- 可缓存的最小前缀为 1024 token，按 128 token 增量命中

**官方示例**（prompt caching guide）：
```json
{
  "usage": {
    "prompt_tokens": 2006,
    "prompt_tokens_details": {
      "cached_tokens": 1920
    }
  }
}
```
解读：1920 是 2006 的一部分，剩余 86 个未命中。

---

### 2.2 OpenAI Responses API (`/v1/responses`)

**顶层字段**：

| 字段 | 说明 |
|---|---|
| `input_tokens` | 总输入，**包含**缓存命中和缓存写入 |
| `output_tokens` | 总输出，**包含** reasoning_tokens |
| `total_tokens` | = input_tokens + output_tokens |

**输入明细（`input_tokens_details`）**：

| 字段 | 说明 |
|---|---|
| `cached_tokens` | 缓存命中（子集） |
| `cache_write_tokens` | 本次写入缓存（子集，Responses 特有字段） |

**输出明细（`output_tokens_details`）**：

| 字段 | 说明 |
|---|---|
| `reasoning_tokens` | 推理 token（子集） |

**注意事项**：
- 没有模态细分、没有预测 token 字段（较 Chat Completions 精简）
- 复用 `previous_response_id` 时每次都为整个已见对话历史计费，`input_tokens` 会随轮次增长

**官方示例**：
```json
{
  "usage": {
    "input_tokens": 2600,
    "input_tokens_details": {
      "cached_tokens": 2000,
      "cache_write_tokens": 400
    },
    "output_tokens": 300,
    "output_tokens_details": {
      "reasoning_tokens": 120
    },
    "total_tokens": 2900
  }
}
```
解读：2600 = 2000 cached + 400 write + 200 新输入。

---

### 2.3 Claude Messages API (`/v1/messages`)

**输入侧字段**：

| 字段 | 说明 |
|---|---|
| `input_tokens` | **仅计最后一个 cache 断点之后的输入**（未走缓存的常规输入） |
| `cache_read_input_tokens` | 从缓存读到的 |
| `cache_creation_input_tokens` | 本次写入缓存的 |
| `cache_creation.ephemeral_5m_input_tokens` | 创建 5 分钟缓存条目的 token |
| `cache_creation.ephemeral_1h_input_tokens` | 创建 1 小时缓存条目的 token |

**输出侧字段**：

| 字段 | 说明 |
|---|---|
| `output_tokens` | 总输出（含 thinking），**是计费权威值** |
| `output_tokens_details.thinking_tokens` | 思考 token（`output_tokens` 的子集，只读观测用） |

**非 token 元数据**：

| 字段 | 说明 |
|---|---|
| `server_tool_use.web_search_requests` / `web_fetch_requests` | 服务端工具调用次数（按次计费） |
| `service_tier` | standard / priority / batch |
| `inference_geo` | 推理所在地理区域 |

**计费倍率**（官方 prompt caching 文档）：

| Token 类型 | 倍率（相对基础输入价） |
|---|---|
| 常规输入 | 1.0× |
| 5 分钟缓存写入（ephemeral_5m） | 1.25× |
| 1 小时缓存写入（ephemeral_1h） | 2× |
| 缓存读取 | 0.1× |

**注意事项**：
- Claude 没有原生 `total_tokens` 字段
- `cache_creation.ephemeral_5m_input_tokens + ephemeral_1h_input_tokens` 等于 `cache_creation_input_tokens`，它们是 TTL 细分，不能重复相加
- 老响应没有 `cache_creation` 细分时，`cache_creation_input_tokens` 默认按 5m（1.25×）计

**官方完整示例**：
```json
{
  "usage": {
    "input_tokens": 2048,
    "cache_read_input_tokens": 1800,
    "cache_creation_input_tokens": 248,
    "output_tokens": 503,
    "cache_creation": {
      "ephemeral_5m_input_tokens": 148,
      "ephemeral_1h_input_tokens": 100
    },
    "output_tokens_details": { "thinking_tokens": 0 },
    "server_tool_use": { "web_search_requests": 0, "web_fetch_requests": 2 },
    "service_tier": "standard",
    "inference_geo": "global"
  }
}
```

---

### 2.4 Gemini GenerateContent API

**注意**：Gemini 的用量不在 `usage` 字段里，而是响应顶层的 `usageMetadata`，且为 camelCase。

**字段规格**：

| 字段 | 说明 |
|---|---|
| `promptTokenCount` | 总输入，**包含**缓存部分 |
| `cachedContentTokenCount` | 其中命中缓存的（显式 context caching；2.5 系列隐式缓存命中也体现在此字段） |
| `candidatesTokenCount` | 输出 token（不含思考） |
| `thoughtsTokenCount` | 思考 token，**输出侧**，按输出价计费 |
| `toolUsePromptTokenCount` | 工具使用 prompt 的 token |
| `totalTokenCount` | = promptTokenCount + thoughtsTokenCount + candidatesTokenCount |
| `promptTokensDetails[]` / `cacheTokensDetails[]` / `candidatesTokensDetails[]` / `toolUsePromptTokensDetails[]` | 按模态（TEXT/IMAGE/AUDIO/VIDEO）细分的列表 |
| `serviceTier` | 服务层级 |

**官方原文**（关键）：
> "When `cachedContent` is set, this is still the total effective prompt size meaning this includes the number of tokens in the cached content."

> "Total token count for the generation request (prompt + thoughts + response candidates)."

**官方示例**：
```json
{
  "usageMetadata": {
    "promptTokenCount": 1050,
    "cachedContentTokenCount": 1000,
    "candidatesTokenCount": 200,
    "thoughtsTokenCount": 150,
    "toolUsePromptTokenCount": 0,
    "totalTokenCount": 1400,
    "promptTokensDetails": [{ "modality": "TEXT", "tokenCount": 1050 }],
    "cacheTokensDetails": [{ "modality": "TEXT", "tokenCount": 1000 }],
    "candidatesTokensDetails": [{ "modality": "TEXT", "tokenCount": 200 }]
  }
}
```

---

## 三、计费换算公式

```text
【OpenAI 系（Chat Completions / Responses 同构）】
输入成本 = (input_tokens − cached_tokens − cache_write_tokens) × P_in
         + cached_tokens × P_cached
         + cache_write_tokens × P_cache_write
输出成本 = output_tokens × P_out
         # reasoning 已含在 output 里，不另计

【Claude】
输入成本 = input_tokens × P_in
         + cache_creation_5m × P_in × 1.25
         + cache_creation_1h × P_in × 2
         + cache_read × P_in × 0.1
输出成本 = output_tokens × P_out
         # thinking 已含在 output 里
         
# 总输入量展示用 = 三项相加，切勿把 input_tokens 当总输入

【Gemini】
输入成本 = (promptTokenCount − cachedContentTokenCount) × P_in
         + cachedContentTokenCount × P_cached
输出成本 = (candidatesTokenCount + thoughtsTokenCount) × P_out
```

---

## 四、协议互转的坑

### 4.1 最容易踩的坑：Claude ↔ 其他三家

**问题**：把 Claude 的 `input_tokens` 直接映射成 OpenAI 的 `prompt_tokens`，会导致缓存场景下输入量少算（Claude 侧）或多算（反向）。

**正确做法**：

```text
Claude → OpenAI / Gemini（加法）:
  顶层输入 = input_tokens + cache_read_input_tokens + cache_creation_input_tokens
  缓存明细 = cache_read_input_tokens（映射为 cached_tokens）

OpenAI / Gemini → Claude（减法）:
  input_tokens = 顶层输入 − 缓存命中 − 缓存写入
  cache_read_input_tokens = 缓存命中
  cache_creation_input_tokens = 缓存写入
```

**注意**：OpenAI 的 `cached_tokens` 是「命中读取」语义，没有 Claude 的「写入 1.25×/2×」概念；反向转换时 Claude 的写入 token 一般并入常规输入或缓存读取，需按计费策略明确选一边。

---

## 五、流式响应中 usage 的位置

| 协议 | 流式 usage 出现位置 |
|---|---|
| OpenAI Chat | 需设 `stream_options: {"include_usage": true}`，最后一个 chunk 带 usage |
| OpenAI Responses | `response.completed` 事件的 `response.usage` |
| Claude | `message_start` 带输入侧 usage，`message_delta` 累计 `output_tokens`<br>**已知问题**：部分流式场景 `message_delta` 只带 `output_tokens`，输入侧字段缺失，需保留 `message_start` 的值合并 |
| Gemini | `streamGenerateContent` 最后一个 chunk 的 `usageMetadata` |

---

## 六、推荐的内部统一模型

**不建议只保存通用的 `input_tokens` 和 `output_tokens`**。为了准确计费和协议转换，建议至少保存：

```text
input_total          # 总输入（含缓存）
input_ordinary       # 普通输入（不含缓存）
cache_read           # 缓存读取
cache_write          # 缓存写入
output_total         # 总输出
visible_output       # 可见输出
reasoning_or_thinking # 推理/思考 token
provider_total       # 供应商返回的 total_tokens
```

**推荐映射**：

| 协议 | `input_total` | `input_ordinary` | `cache_read` | `cache_write` |
|---|---|---|---|---|
| OpenAI Chat | `prompt_tokens` | `prompt_tokens - cached_tokens - cache_write_tokens` | `cached_tokens` | `cache_write_tokens` |
| OpenAI Responses | `input_tokens` | `input_tokens - cached_tokens - cache_write_tokens` | `cached_tokens` | `cache_write_tokens` |
| Claude | 三个输入桶之和 | `input_tokens` | `cache_read_input_tokens` | `cache_creation_input_tokens` |
| Gemini | `promptTokenCount` | `promptTokenCount - cachedContentTokenCount` | `cachedContentTokenCount` | 无对应生成响应字段 |

**输出侧注意事项**：

- OpenAI 的 `completion_tokens` / `output_tokens` 已包含 `reasoning_tokens`
- Claude 的 `output_tokens` 已包含 `thinking_tokens`
- Gemini 的 `candidatesTokenCount` 不包含 `thoughtsTokenCount`；其完整生成侧数量应依据 `totalTokenCount - promptTokenCount` 或直接分别保留 candidates 与 thoughts
- **所有减法都应做非负保护**，以防第三方兼容服务返回不一致数据

---

## 七、项目代码需要改进的地方

### 7.1 已正确处理的部分

✅ OpenAI Responses 已设置 `CacheIncludedInPrompt=true`，符合 OpenAI 顶层输入包含缓存的语义

✅ Gemini 已设置 `CacheIncludedInPrompt=true`，符合 `promptTokenCount` 包含 `cachedContentTokenCount` 的语义

✅ Claude 将缓存读取和缓存创建保留为独立桶，符合 Claude 语义

### 7.2 需要更新或明确标注的部分

#### 问题 1：OpenAI Chat 通用明细缺少官方 `cache_write_tokens`

**位置**：`relaykit/dto/usage.go`

**现状**：目前有项目自定义的 `cached_creation_tokens`，但它不是 OpenAI 官方 JSON 字段 `cache_write_tokens`。

**建议**：增加官方 `cache_write_tokens` 字段，不要与自定义字段混用。

---

#### 问题 2：OpenAI Responses 输入明细缺少 `cache_write_tokens`

**位置**：`relay/dto/openai_responses.go`

**现状**：`InputTokenDetails` 当前只有 `cached_tokens` 及模态扩展字段。

**建议**：增加官方 `cache_write_tokens` 字段。

---

#### 问题 3：Responses DTO 中存在非当前官方 schema 的扩展字段

**位置**：`relay/dto/openai_responses.go`

**现状**：`input_tokens_details.text_tokens/audio_tokens/image_tokens` 以及输出侧的文本、音频、预测 token 字段不在官方 Responses schema 中。

**建议**：如果为了兼容第三方上游而保留，应明确标记为兼容扩展，不能当作 OpenAI 官方字段。

---

#### 问题 4：Claude DTO 缺少最新 Usage 字段

**位置**：`relaykit/dto/claude.go`

**现状**：当前缺少：
- `output_tokens_details.thinking_tokens`
- `inference_geo`
- `server_tool_use.web_fetch_requests`

**建议**：补充上述官方字段。

---

#### 问题 5：Gemini DTO 缺少 `usageMetadata.serviceTier`

**位置**：`relaykit/dto/gemini.go`

**现状**：已覆盖主要 token 字段和模态明细，但未覆盖当前官方 `serviceTier`。

**建议**：补充该字段。

---

#### 问题 6：Claude 转 OpenAI 的 usage 口径不符合 OpenAI 子集语义 ⚠️

**位置**：`relaykit/relayconvert/internal/oai_chat/claude_to_openai_response.go`

**现状**：当前将 Claude `input_tokens` 直接映射为 OpenAI `prompt_tokens`，同时把 `cache_read_input_tokens` 放入 `cached_tokens`。这样 `cached_tokens` 并未包含在 `prompt_tokens` 中，与 OpenAI 官方语义不一致。

**正确的协议映射应为**：

```go
prompt_tokens = input_tokens + cache_read_input_tokens + cache_creation_input_tokens

prompt_tokens_details.cached_tokens = cache_read_input_tokens
prompt_tokens_details.cache_write_tokens = cache_creation_input_tokens

total_tokens = prompt_tokens + completion_tokens
```

---

#### 问题 7：Claude 内部 `TotalTokens` 不是跨协议总量

**位置**：Claude 路径

**现状**：当前使用 `input_tokens + output_tokens` 生成内部 `TotalTokens`，缓存 token 另存。

**说明**：这可以作为内部计费表示，但不能把该值解释为 Claude 请求实际处理的全部 token，也不能直接映射为 OpenAI `total_tokens`。

---

## 八、参考来源

以下页面均在 2026-08-16 ~ 2026-08-17 实际联网获取并读取正文：

### OpenAI

- [Chat Completions API Reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
- [Responses API Reference](https://developers.openai.com/api/reference/resources/responses/methods/create)
- [Prompt Caching Guide](https://developers.openai.com/api/docs/guides/prompt-caching)
- [openai-python — completion_usage.py（PromptTokensDetails/CompletionTokensDetails 权威 schema）](https://github.com/openai/openai-python/blob/main/src/openai/types/completion_usage.py)
- [openai-python — response_usage.py（Responses usage 权威 schema）](https://github.com/openai/openai-python/blob/main/src/openai/types/responses/response_usage.py)

### Anthropic

- [Messages API Reference](https://platform.claude.com/docs/en/api/messages)
- [Prompt Caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)

### Google Gemini

- [GenerateContent API Reference — UsageMetadata](https://ai.google.dev/api/generate-content#UsageMetadata)
- [Gemini API — Generating content](https://ai.google.dev/api/generate-content)

---

## 九、总结

对于缓存输入，四种协议应按两类处理：

```text
包含型：OpenAI Chat、OpenAI Responses、Gemini
独立桶型：Claude
```

最新版 OpenAI 又进一步把包含型输入拆成"缓存读取、缓存写入、普通输入"三个计费类别。因此，项目后续不应只维护 `CacheIncludedInPrompt` 一个布尔值，还应显式支持 `cache_write_tokens`，并在协议转换时分别处理缓存读、缓存写和普通输入。

**核心规则**：
- OpenAI/Gemini → 顶层输入包含缓存（减法得普通输入）
- Claude → 三个独立桶相加才是总输入（加法）
- 协议转换时必须显式处理缓存读/写/普通输入的拆分与合并
- 所有减法操作必须做非负保护
