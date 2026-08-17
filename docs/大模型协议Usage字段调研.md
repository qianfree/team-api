# 大模型协议 Usage 字段调研

> 调研对象：OpenAI Chat Completions、OpenAI Responses、Claude Messages、Gemini generateContent 四个协议响应中的 token 用量（usage）结构。
> 核心关注点：**存在缓存时，输入 token 是否包含缓存 token**。
> 调研日期：2026-08-17（基于各官方文档当时的版本）

## 核心结论

四家协议里三家是「包含」口径，**Claude 是唯一的「排除」口径**：

| 协议 | 输入字段 | 缓存 token 是否含在内 | 未缓存输入怎么得到 |
|---|---|---|---|
| OpenAI Chat Completions | `prompt_tokens` | ✅ 包含，`cached_tokens` 是它的子集 | `prompt_tokens - cached_tokens` |
| OpenAI Responses | `input_tokens` | ✅ 包含，`cached_tokens` 是它的子集 | `input_tokens - cached_tokens - cache_write_tokens` |
| **Claude Messages** | `input_tokens` | ❌ **排除**，缓存是独立并列字段 | `input_tokens` 本身就是未缓存部分；总输入 = 三个字段**相加** |
| Gemini | `promptTokenCount` | ✅ 包含，`cachedContentTokenCount` 是它的子集 | `promptTokenCount - cachedContentTokenCount` |

## 1. OpenAI Chat Completions（`/v1/chat/completions`）

官方文档示例（chat/object）：

```json
"usage": {
  "prompt_tokens": 1117,
  "completion_tokens": 46,
  "total_tokens": 1163,
  "prompt_tokens_details": {
    "cached_tokens": 0,
    "audio_tokens": 0
  },
  "completion_tokens_details": {
    "reasoning_tokens": 0,
    "audio_tokens": 0,
    "accepted_prediction_tokens": 0,
    "rejected_prediction_tokens": 0
  }
}
```

字段全集（据官方 SDK `completion_usage.py` 的 schema）：

| 字段 | 说明 |
|---|---|
| `prompt_tokens` | 总输入 token，**包含**缓存命中部分 |
| `completion_tokens` | 总输出 token，**包含** reasoning tokens |
| `total_tokens` | = prompt_tokens + completion_tokens |
| `prompt_tokens_details.cached_tokens` | 缓存命中读到的输入 token（prompt_tokens 的子集） |
| `prompt_tokens_details.cache_write_tokens` | 本次写入缓存的 token（较新字段，prompt_tokens 的子集） |
| `prompt_tokens_details.audio_tokens / text_tokens / image_tokens` | 按模态细分（prompt_tokens 的子集） |
| `completion_tokens_details.reasoning_tokens` | 推理 token（completion_tokens 的子集） |
| `completion_tokens_details.audio_tokens / text_tokens` | 按模态细分（completion_tokens 的子集） |
| `completion_tokens_details.accepted_prediction_tokens / rejected_prediction_tokens` | Predicted Outputs 功能相关 |

包含关系的官方例证（prompt caching guide）：`prompt_tokens: 2006, cached_tokens: 1920` —— 1920 是 2006 的一部分，剩 86 个未命中。

缓存计费：命中部分按折扣价计费（guide 现行口径 0.1×，实际各模型定价表不同，如 gpt-4o 为 0.5×）。可缓存的最小前缀为 1024 token，按 128 token 增量命中。

## 2. OpenAI Responses API（`/v1/responses`）

usage 结构明显比 Chat Completions 精简（据官方 SDK `response_usage.py`）：

```json
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
```

| 字段 | 说明 |
|---|---|
| `input_tokens` | 总输入，**包含**缓存命中和缓存写入（示例：2600 = 2000 cached + 400 write + 200 新输入） |
| `input_tokens_details.cached_tokens` | 缓存命中（子集） |
| `input_tokens_details.cache_write_tokens` | 本次写入缓存（子集）——Responses 特有字段，Chat Completions 侧较新才补上 |
| `output_tokens` | 总输出，**包含** reasoning_tokens（子集） |
| `output_tokens_details.reasoning_tokens` | 推理 token |
| `total_tokens` | = input_tokens + output_tokens |

注意：
- **没有**模态细分、没有预测 token 字段。
- 复用 `previous_response_id` 时每次都为整个已见对话历史计费，`input_tokens` 会随轮次增长，`cached_tokens` 会很大。

## 3. Claude Messages API（`/v1/messages`）

官方 API reference 的 usage 字段全集：

```json
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
```

| 字段 | 说明 |
|---|---|
| `input_tokens` | **仅计最后一个 cache 断点之后的输入**（未走缓存的常规输入） |
| `cache_read_input_tokens` | 从缓存读到的 |
| `cache_creation_input_tokens` | 本次写入缓存的 |
| `cache_creation.ephemeral_5m_input_tokens / ephemeral_1h_input_tokens` | 按 TTL 细分的缓存写入，两者之和 = cache_creation_input_tokens |
| `output_tokens` | 总输出（含 thinking），是计费权威值 |
| `output_tokens_details.thinking_tokens` | 思考 token（output_tokens 的子集，只读观测用） |
| `server_tool_use.web_search_requests / web_fetch_requests` | 服务端工具调用次数（按次计费） |
| `service_tier` | standard / priority / batch |
| `inference_geo` | 推理所在地理区域 |

官方原文（关键）：

> "Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`."

> "The `input_tokens` field represents only the tokens that come after the last cache breakpoint in your request - not all the input tokens you sent."

计费倍率（官方 prompt caching 文档）：

| Token 类型 | 倍率（相对基础输入价） |
|---|---|
| 常规输入 | 1.0× |
| 5 分钟缓存写入（ephemeral_5m） | 1.25× |
| 1 小时缓存写入（ephemeral_1h） | 2× |
| 缓存读取 | 0.1× |

老响应没有 `cache_creation` 细分时，`cache_creation_input_tokens` 默认按 5m（1.25×）计。

## 4. Gemini（`generateContent` 的 `usageMetadata`）

注意：Gemini 的用量不在 `usage` 字段里，而是响应顶层的 `usageMetadata`，且为 camelCase：

```json
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
```

| 字段 | 说明 |
|---|---|
| `promptTokenCount` | 总输入，**包含**缓存部分 |
| `cachedContentTokenCount` | 其中命中缓存的（显式 context caching；2.5 系列隐式缓存命中也体现在此字段） |
| `candidatesTokenCount` | 输出 token（不含思考） |
| `thoughtsTokenCount` | 思考 token，**输出侧**，按输出价计费 |
| `toolUsePromptTokenCount` | 工具使用 prompt 的 token |
| `totalTokenCount` | = promptTokenCount + thoughtsTokenCount + candidatesTokenCount |
| `promptTokensDetails[] / cacheTokensDetails[] / candidatesTokensDetails[] / toolUsePromptTokensDetails[]` | 按模态（TEXT/IMAGE/AUDIO/VIDEO）细分的列表 |
| `serviceTier` | 服务层级 |

官方原文（关键）：

> "When `cachedContent` is set, this is still the total effective prompt size meaning this includes the number of tokens in the cached content."

> "Total token count for the generation request (prompt + thoughts + response candidates)."

## 计费换算公式

```
OpenAI 系（两协议同构）:
  输入成本 = (input_tokens − cached_tokens) × P_in + cached_tokens × P_cached
  输出成本 = output_tokens × P_out            # reasoning 已含在 output 里，不另计

Claude:
  输入成本 = input_tokens × P_in
           + cache_creation_5m × P_in × 1.25 + cache_creation_1h × P_in × 2
           + cache_read × P_in × 0.1
  输出成本 = output_tokens × P_out            # thinking 已含在 output 里
  （总输入量展示用 = 三项相加，切勿把 input_tokens 当总输入）

Gemini:
  输入成本 = (promptTokenCount − cachedContentTokenCount) × P_in + cachedContentTokenCount × P_cached
  输出成本 = (candidatesTokenCount + thoughtsTokenCount) × P_out
```

## 协议互转的坑

最容易踩的坑是 **Claude ↔ 其他三家**：

- 把 Claude 的 `input_tokens` 直接映射成 OpenAI 的 `prompt_tokens`，会导致缓存场景下输入量少算（Claude 侧）或多算（反向）。
- 正确做法：Claude → 其他，做**加法**（`input + cache_read + cache_creation` → 对方口径的「总输入」）；其他 → Claude，做**减法**（总输入 − 缓存命中 → `input_tokens`，缓存命中 → `cache_read_input_tokens`）。
- 注意 OpenAI 的 `cached_tokens` 是「命中读取」语义，没有 Claude 的「写入 1.25×/2×」概念；反向转换时 Claude 的写入 token 一般并入常规输入或缓存读取，需按计费策略明确选一边。

## 流式响应中 usage 的位置

| 协议 | 流式 usage 出现位置 |
|---|---|
| OpenAI chat | 需设 `stream_options: {"include_usage": true}`，最后一个 chunk 带 usage |
| Responses | `response.completed` 事件的 `response.usage` |
| Claude | `message_start` 带输入侧 usage，`message_delta` 累计 `output_tokens`（已知问题：部分流式场景 `message_delta` 只带 `output_tokens`，输入侧字段缺失，需保留 `message_start` 的值合并） |
| Gemini | `streamGenerateContent` 最后一个 chunk 的 `usageMetadata` |

## 参考来源

- [Gemini API — Generating content（usageMetadata 完整规格）](https://ai.google.dev/api/generate-content)
- [OpenAI — Chat Completions API Reference（chat object）](https://platform.openai.com/docs/api-reference/chat/object)
- [OpenAI — Prompt caching guide](https://developers.openai.com/api/docs/guides/prompt-caching)
- [openai-python — completion_usage.py（PromptTokensDetails/CompletionTokensDetails 权威 schema）](https://github.com/openai/openai-python/blob/main/src/openai/types/completion_usage.py)
- [openai-python — response_usage.py（Responses usage 权威 schema）](https://github.com/openai/openai-python/blob/main/src/openai/types/responses/response_usage.py)
- [Anthropic — Messages API Reference（usage object）](https://platform.claude.com/docs/en/api/messages)
- [Anthropic — Prompt caching（计费倍率与 token 拆解）](https://platform.claude.com/docs/en/docs/build-with-claude/prompt-caching)
