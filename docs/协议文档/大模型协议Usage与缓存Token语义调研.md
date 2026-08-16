# 大模型协议 Usage 与缓存 Token 语义调研（官方联网核验版）

## 文档信息

- 核验日期：2026-08-16
- 核验方式：联网读取 OpenAI、Anthropic、Google 官方 API Reference 与 Prompt Caching 文档正文。
- 覆盖范围：OpenAI Chat Completions、OpenAI Responses、Claude Messages API、Gemini GenerateContent API。
- 不包含：Azure OpenAI、Amazon Bedrock、Vertex AI 等托管平台对字段的二次封装，也不把第三方兼容网关的扩展字段视为官方字段。

## 核心结论

| 协议 | 顶层输入字段 | 缓存读取字段 | 缓存写入字段 | 顶层输入是否包含缓存 |
| --- | --- | --- | --- | --- |
| OpenAI Chat Completions | `prompt_tokens` | `prompt_tokens_details.cached_tokens` | `prompt_tokens_details.cache_write_tokens` | **包含缓存读取和缓存写入** |
| OpenAI Responses | `input_tokens` | `input_tokens_details.cached_tokens` | `input_tokens_details.cache_write_tokens` | **包含缓存读取和缓存写入** |
| Claude Messages | `input_tokens` | `cache_read_input_tokens` | `cache_creation_input_tokens` | **不包含，三个字段是独立桶** |
| Gemini GenerateContent | `promptTokenCount` | `cachedContentTokenCount` | GenerateContent usage 无对应字段 | **包含缓存读取** |

不能把四种协议的“输入 token”直接按同一口径使用：

```text
OpenAI / Responses:
普通输入 = 顶层输入 - cached_tokens - cache_write_tokens

Claude:
总输入 = input_tokens + cache_read_input_tokens + cache_creation_input_tokens

Gemini:
普通输入 = promptTokenCount - cachedContentTokenCount
```

其中 `cache_write_tokens` 是 OpenAI 官方文档中的新字段。官方 Prompt Caching 文档说明：GPT-5.6 及后续模型系列会报告缓存写入 token；早期模型没有该字段或不产生对应的额外缓存写入计费。

## OpenAI Chat Completions

### 官方 Usage 结构

顶层字段：

| 字段 | 官方语义 |
| --- | --- |
| `prompt_tokens` | prompt 使用的 token 数 |
| `completion_tokens` | 生成 completion 使用的 token 数 |
| `total_tokens` | `prompt_tokens + completion_tokens` |
| `prompt_tokens_details` | 输入 token 明细 |
| `completion_tokens_details` | 输出 token 明细 |

截至核验日期，`prompt_tokens_details` 的官方字段为：

| 字段 | 官方语义 |
| --- | --- |
| `audio_tokens` | prompt 中的音频输入 token |
| `cache_write_tokens` | 写入缓存的 prompt token 原始数量 |
| `cached_tokens` | prompt 中命中缓存的 token |
| `image_tokens` | prompt 中的图像输入 token |
| `text_tokens` | prompt 中的文本输入 token |

`completion_tokens_details` 的官方字段为：

| 字段 | 官方语义 |
| --- | --- |
| `accepted_prediction_tokens` | Predicted Outputs 中被实际采用的预测 token |
| `audio_tokens` | 模型生成的音频 token |
| `reasoning_tokens` | 模型生成的推理 token |
| `rejected_prediction_tokens` | Predicted Outputs 中未被采用的预测 token；官方明确说明仍计入 completion token、计费及上下文限制 |
| `text_tokens` | 模型生成的文本 token |

### 缓存口径

OpenAI Prompt Caching 官方示例表明，顶层输入包含缓存读取、缓存写入和普通输入。例如：

```json
{
  "usage": {
    "input_tokens": 2600,
    "input_tokens_details": {
      "cached_tokens": 2000,
      "cache_write_tokens": 400
    }
  }
}
```

该示例的官方解释是：2000 个 token 从缓存读取，400 个 token 写入缓存，剩余 200 个既未读取也未写入。因此 Chat Completions 对应公式为：

```text
普通输入 token =
    prompt_tokens
  - prompt_tokens_details.cached_tokens
  - prompt_tokens_details.cache_write_tokens
```

`text_tokens`、`image_tokens`、`audio_tokens` 是按模态的拆分；`cached_tokens`、`cache_write_tokens` 是按缓存状态的拆分。两组维度可能重叠，不能把所有 details 字段直接求和。

## OpenAI Responses

### 官方 Usage 结构

```json
{
  "usage": {
    "input_tokens": 2600,
    "output_tokens": 120,
    "total_tokens": 2720,
    "input_tokens_details": {
      "cached_tokens": 2000,
      "cache_write_tokens": 400
    },
    "output_tokens_details": {
      "reasoning_tokens": 50
    }
  }
}
```

截至核验日期，Responses 官方 API Reference 中的字段为：

| 位置 | 字段 | 官方语义 |
| --- | --- | --- |
| `usage` | `input_tokens` | 输入 token 数 |
| `usage` | `output_tokens` | 输出 token 数 |
| `usage` | `total_tokens` | 使用的总 token 数 |
| `input_tokens_details` | `cached_tokens` | 从缓存取回的 token 数 |
| `input_tokens_details` | `cache_write_tokens` | 写入缓存的输入 token 数 |
| `output_tokens_details` | `reasoning_tokens` | 推理 token 数 |

缓存公式为：

```text
普通输入 token =
    input_tokens
  - input_tokens_details.cached_tokens
  - input_tokens_details.cache_write_tokens
```

Responses 当前官方 schema 没有在 `input_tokens_details` 中声明 `text_tokens`、`image_tokens`、`audio_tokens`，也没有在 `output_tokens_details` 中声明预测或模态明细。兼容服务可能返回扩展字段，但不能将它们标注成 OpenAI Responses 官方字段。

## Claude Messages API

### 官方 Usage 结构

Claude 官方 API Reference 当前声明的 token 字段包括：

| 字段 | 官方语义 |
| --- | --- |
| `input_tokens` | 未从缓存读取、也未用于创建缓存的输入 token；Prompt Caching 文档进一步说明，它通常对应最后一个缓存断点之后的 token |
| `cache_creation_input_tokens` | 用于创建缓存条目的输入 token |
| `cache_read_input_tokens` | 从缓存读取的输入 token |
| `output_tokens` | 输出 token 的权威总数，包含 thinking token |
| `cache_creation.ephemeral_5m_input_tokens` | 创建 5 分钟缓存条目的 token |
| `cache_creation.ephemeral_1h_input_tokens` | 创建 1 小时缓存条目的 token |
| `output_tokens_details.thinking_tokens` | 内部推理 token 明细，是 `output_tokens` 的子集 |

Usage 中还包含以下非 token 元数据：

- `inference_geo`
- `server_tool_use.web_fetch_requests`
- `server_tool_use.web_search_requests`
- `service_tier`，可能为 `standard`、`priority`、`batch` 或 `null`

### 缓存口径

Anthropic 官方 API Reference 和 Prompt Caching 文档均明确给出：

```text
总输入 token =
    input_tokens
  + cache_creation_input_tokens
  + cache_read_input_tokens
```

因此，Claude 的 `input_tokens` **不包含**缓存读取和缓存创建 token。它与 OpenAI、Gemini 的顶层输入字段语义不同。

另外：

- Claude 没有原生 `total_tokens` 字段。
- `cache_creation.ephemeral_5m_input_tokens + ephemeral_1h_input_tokens` 等于 `cache_creation_input_tokens`，它们是 TTL 细分，不能重复相加。
- `output_tokens_details.thinking_tokens <= output_tokens`；`output_tokens` 仍是计费使用的权威输出总量。
- Amazon Bedrock 等托管平台可能使用不同字段名和缓存规则，应单独核验对应平台文档。

## Gemini GenerateContent API

### 官方 UsageMetadata 结构

截至核验日期，Gemini 官方 `UsageMetadata` 字段为：

| 字段 | 官方语义 |
| --- | --- |
| `promptTokenCount` | prompt 的总有效 token 数；官方明确说明设置 `cachedContent` 时仍包含缓存内容 token |
| `cachedContentTokenCount` | prompt 中缓存内容的 token 数 |
| `candidatesTokenCount` | 所有生成候选响应的 token 总数 |
| `toolUsePromptTokenCount` | tool-use prompt 中的 token 数 |
| `thoughtsTokenCount` | thinking 模型的思考 token 数 |
| `totalTokenCount` | 生成请求的总 token 数，官方定义为 prompt + thoughts + response candidates |
| `promptTokensDetails[]` | 请求输入按模态的 token 明细 |
| `cacheTokensDetails[]` | 缓存输入按模态的 token 明细 |
| `candidatesTokensDetails[]` | 响应候选按模态的 token 明细 |
| `toolUsePromptTokensDetails[]` | tool-use 请求输入按模态的 token 明细 |
| `serviceTier` | 请求使用的服务层级，非 token 字段 |

### 缓存口径

Google 官方 API Reference 的原意非常明确：`promptTokenCount` 在设置 `cachedContent` 后仍表示总有效 prompt 大小，包含缓存内容。因此：

```text
普通输入 token = promptTokenCount - cachedContentTokenCount
```

Gemini 的 `thoughtsTokenCount` 不是 `candidatesTokenCount` 的子集。官方给出的总量定义是：

```text
totalTokenCount = promptTokenCount + thoughtsTokenCount + candidatesTokenCount
```

所以有 thinking 时，不能使用 `promptTokenCount + candidatesTokenCount` 代替 `totalTokenCount`。程序应优先采用上游返回的 `totalTokenCount`。

## 推荐的内部统一模型

不建议只保存通用的 `input_tokens` 和 `output_tokens`。为了准确计费和协议转换，建议至少保存：

```text
input_total
input_ordinary
cache_read
cache_write
output_total
visible_output
reasoning_or_thinking
provider_total
```

推荐映射：

| 协议 | `input_total` | `input_ordinary` | `cache_read` | `cache_write` |
| --- | --- | --- | --- | --- |
| OpenAI Chat | `prompt_tokens` | `prompt_tokens - cached_tokens - cache_write_tokens` | `cached_tokens` | `cache_write_tokens` |
| OpenAI Responses | `input_tokens` | `input_tokens - cached_tokens - cache_write_tokens` | `cached_tokens` | `cache_write_tokens` |
| Claude | 三个输入桶之和 | `input_tokens` | `cache_read_input_tokens` | `cache_creation_input_tokens` |
| Gemini | `promptTokenCount` | `promptTokenCount - cachedContentTokenCount` | `cachedContentTokenCount` | 无对应生成响应字段 |

输出侧需要额外注意：

- OpenAI 的 `completion_tokens` / `output_tokens` 已包含 `reasoning_tokens`。
- Claude 的 `output_tokens` 已包含 `thinking_tokens`。
- Gemini 的 `candidatesTokenCount` 不包含 `thoughtsTokenCount`；其完整生成侧数量应依据 `totalTokenCount - promptTokenCount` 或直接分别保留 candidates 与 thoughts。
- 所有减法都应做非负保护，以防第三方兼容服务返回不一致数据。

## 本仓库与最新官方协议的差异

### 已正确处理的部分

- OpenAI Responses 已设置 `CacheIncludedInPrompt=true`，符合 OpenAI 顶层输入包含缓存的语义：[`relay/channel/openai/responses.go`](relay/channel/openai/responses.go)。
- Gemini 已设置 `CacheIncludedInPrompt=true`，符合 `promptTokenCount` 包含 `cachedContentTokenCount` 的语义：[`relay/channel/gemini/response.go`](relay/channel/gemini/response.go)。
- Claude 将缓存读取和缓存创建保留为独立桶，符合 Claude 语义：[`relay/channel/claude/response.go`](relay/channel/claude/response.go)。

### 需要更新或明确标注的部分

1. OpenAI Chat 通用明细缺少官方 `cache_write_tokens`。

   [`relaykit/dto/usage.go`](relaykit/dto/usage.go) 目前有项目自定义的 `cached_creation_tokens`，但它不是 OpenAI 官方 JSON 字段 `cache_write_tokens`。两者不应使用同一个 JSON 名称或隐式混用。

2. OpenAI Responses 输入明细缺少 `cache_write_tokens`。

   [`relay/dto/openai_responses.go`](relay/dto/openai_responses.go) 的 `InputTokenDetails` 当前只有 `cached_tokens` 及模态扩展字段，应增加官方 `cache_write_tokens`。

3. Responses DTO 中存在非当前官方 schema 的扩展字段。

   `input_tokens_details.text_tokens/audio_tokens/image_tokens` 以及输出侧的文本、音频、预测 token 字段不在本次联网核验到的 OpenAI Responses 官方 schema 中。如果为了兼容第三方上游而保留，应明确标记为兼容扩展，不能当作 OpenAI 官方字段。

4. Claude DTO 缺少最新 Usage 字段。

   [`relaykit/dto/claude.go`](relaykit/dto/claude.go) 当前缺少：

   - `output_tokens_details.thinking_tokens`
   - `inference_geo`
   - `server_tool_use.web_fetch_requests`

5. Gemini DTO 缺少 `usageMetadata.serviceTier`。

   [`relaykit/dto/gemini.go`](relaykit/dto/gemini.go) 已覆盖主要 token 字段和模态明细，但未覆盖当前官方 `serviceTier`。

6. Claude 转 OpenAI 的 usage 口径不符合 OpenAI 子集语义。

   [`relaykit/relayconvert/internal/oai_chat/claude_to_openai_response.go`](relaykit/relayconvert/internal/oai_chat/claude_to_openai_response.go) 当前将 Claude `input_tokens` 直接映射为 OpenAI `prompt_tokens`，同时把 `cache_read_input_tokens` 放入 `cached_tokens`。这样 `cached_tokens` 并未包含在 `prompt_tokens` 中，与 OpenAI 官方语义不一致。

   严格的协议映射应为：

   ```text
   prompt_tokens =
       input_tokens
     + cache_read_input_tokens
     + cache_creation_input_tokens

   prompt_tokens_details.cached_tokens = cache_read_input_tokens
   prompt_tokens_details.cache_write_tokens = cache_creation_input_tokens
   total_tokens = prompt_tokens + completion_tokens
   ```

7. Claude 内部 `TotalTokens` 不是跨协议总量。

   当前 Claude 路径使用 `input_tokens + output_tokens` 生成内部 `TotalTokens`，缓存 token 另存。这可以作为内部计费表示，但不能把该值解释为 Claude 请求实际处理的全部 token，也不能直接映射为 OpenAI `total_tokens`。

## 官方来源

以下页面均在 2026-08-16 实际联网获取并读取正文：

### OpenAI

- [Chat Completions API Reference](https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create)
- [Responses API Reference](https://developers.openai.com/api/reference/resources/responses/methods/create)
- [Prompt Caching Guide](https://developers.openai.com/api/docs/guides/prompt-caching)

### Anthropic

- [Messages API Reference](https://platform.claude.com/docs/en/api/messages)
- [Prompt Caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)

### Google Gemini

- [GenerateContent API Reference - UsageMetadata](https://ai.google.dev/api/generate-content#UsageMetadata)

## 结论

对于缓存输入，四种协议应按两类处理：

```text
包含型：OpenAI Chat、OpenAI Responses、Gemini
独立桶型：Claude
```

最新版 OpenAI 又进一步把包含型输入拆成“缓存读取、缓存写入、普通输入”三个计费类别。因此，项目后续不应只维护 `CacheIncludedInPrompt` 一个布尔值，还应显式支持 `cache_write_tokens`，并在协议转换时分别处理缓存读、缓存写和普通输入。
