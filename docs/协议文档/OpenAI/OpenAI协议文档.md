# OpenAI Chat Completions API 协议文档

> 基于官方 API Reference (https://platform.openai.com/docs/api-reference/chat/create) 整理
>
> 本文档用于 team-api 协议转换模块的开发参考，涵盖 Chat Completions API 的完整请求/响应规范。

---

## 目录

- [1. API 概览](#1-api-概览)
- [2. 认证方式](#2-认证方式)
- [3. 请求参数](#3-请求参数)
- [4. 消息类型](#4-消息类型)
- [5. 响应格式](#5-响应格式)
- [6. 流式响应 (SSE)](#6-流式响应-sse)
- [7. Tool Calling 工作流](#7-tool-calling-工作流)
- [8. Structured Outputs](#8-structured-outputs)
- [9. 错误格式](#9-错误格式)
- [10. finish_reason 取值说明](#10-finish_reason-取值说明)
- [11. 协议转换注意事项](#11-协议转换注意事项)

---

## 1. API 概览

| 项目 | 说明 |
|------|------|
| **端点** | `POST https://api.openai.com/v1/chat/completions` |
| **功能** | 根据给定的对话消息，生成模型响应 |
| **Content-Type** | `application/json` |
| **协议版本** | Chat Completions API（OpenAI 建议新项目使用 Responses API，但 Chat Completions 仍是主流） |

### 基本请求示例

```bash
curl https://api.openai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

---

## 2. 认证方式

请求头中携带 Bearer Token：

```
Authorization: Bearer sk-xxxxxxxxxxxxxxxx
```

| 认证方式 | 说明 |
|---------|------|
| API Key | `Authorization: Bearer sk-xxx`，在平台 API Keys 页面创建 |
| Organization（可选） | `OpenAI-Organization: org-xxx`，指定组织 |
| Project（可选） | `OpenAI-Project: proj-xxx`，指定项目 |

---

## 3. 请求参数

### 3.1 完整参数表

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | **是** | — | 模型 ID，如 `gpt-4o`、`gpt-4.1`、`o3`、`o4-mini` |
| `messages` | array | **是** | — | 对话消息列表，详见 [第 4 节](#4-消息类型) |
| `max_completion_tokens` | integer | 否 | — | 生成 token 数上限（包含可见输出和推理 token）。推荐使用，替代已弃用的 `max_tokens` |
| `temperature` | number | 否 | `1` | 采样温度，范围 0-2。值越高越随机，越低越确定。建议与 `top_p` 二选一调整 |
| `top_p` | number | 否 | `1` | 核采样概率质量。如 0.1 表示只考虑前 10% 概率的 token。建议与 `temperature` 二选一调整 |
| `n` | integer | 否 | `1` | 为每条输入消息生成的候选响应数量。注意：按所有候选的总 token 数计费 |
| `stream` | boolean | 否 | `false` | 是否以流式（SSE）方式返回响应 |
| `stream_options` | object | 否 | `null` | 流式响应选项，仅在 `stream: true` 时有效。见下方详细说明 |
| `stop` | string / array | 否 | `null` | 最多 4 个停止序列。返回文本不包含停止序列。**注意：不支持最新推理模型 o3、o4-mini** |
| `presence_penalty` | number | 否 | `0` | 存在惩罚，范围 -2.0 到 2.0。正值增加模型谈论新话题的概率 |
| `frequency_penalty` | number | 否 | `0` | 频率惩罚，范围 -2.0 到 2.0。正值降低模型逐字重复相同内容的概率 |
| `logit_bias` | object | 否 | `null` | 修改指定 token 的生成概率。键为 token ID，值为 -100 到 100 的偏置值 |
| `logprobs` | boolean | 否 | `false` | 是否返回输出 token 的对数概率 |
| `top_logprobs` | integer | 否 | — | 每个位置返回的最可能 token 数量（0-20），需 `logprobs: true` |
| `response_format` | object | 否 | — | 指定输出格式，支持 `text`、`json_object`、`json_schema`。详见 [第 8 节](#8-structured-outputs) |
| `seed` | integer | 否 | — | 随机种子。尽可能确定性采样，配合 `system_fingerprint` 监控后端变更 |
| `service_tier` | string | 否 | `auto` | 处理层级：`auto`、`default`、`flex`、`priority` |
| `tools` | array | 否 | — | 工具定义列表。详见 [第 7 节](#7-tool-calling-工作流) |
| `tool_choice` | string / object | 否 | — | 控制工具调用行为。详见 [第 7 节](#7-tool-calling-工作流) |
| `parallel_tool_calls` | boolean | 否 | `true` | 是否允许并行调用多个工具 |
| `reasoning_effort` | string | 否 | `medium` | 推理模型的推理力度：`none`、`minimal`、`low`、`medium`、`high`、`xhigh` |
| `user` | string | 否 | — | **已弃用**。终端用户标识符，建议使用 `prompt_cache_key` 替代 |
| `prompt_cache_key` | string | 否 | — | 用于缓存类似请求的响应，优化缓存命中率。替代 `user` 字段 |
| `prompt_cache_retention` | string | 否 | — | 提示缓存保留策略，可设为 `24h` 启用扩展缓存 |
| `metadata` | object | 否 | — | 附加元数据，最多 16 个键值对。键最长 64 字符，值最长 512 字符 |
| `modalities` | array | 否 | `["text"]` | 输出模态类型，如 `["text"]` 或 `["text", "audio"]` |
| `audio` | object | 否 | — | 音频输出参数。当 `modalities` 包含 `"audio"` 时必填 |
| `store` | boolean | 否 | `false` | 是否存储此请求的输出，用于模型蒸馏和评估产品 |
| `web_search_options` | object | 否 | — | 网页搜索工具选项 |
| `safety_identifier` | string | 否 | — | 用户安全标识符（Beta），用于检测违反使用策略的用户 |
| `verbosity` | string | 否 | `medium` | 控制模型回复的详细程度：`low`、`medium`、`high` |
| `prediction` | object | 否 | — | 预测输出配置，当大部分响应内容已知时可显著提升响应速度 |
| `max_tokens` | integer | 否 | — | **已弃用**。被 `max_completion_tokens` 替代，不兼容 o 系列模型 |
| `function_call` | string / object | 否 | — | **已弃用**。被 `tool_choice` 替代 |
| `functions` | array | 否 | — | **已弃用**。被 `tools` 替代 |

### 3.2 stream_options 参数

| 字段 | 类型 | 说明 |
|------|------|------|
| `include_usage` | boolean | 是否在最后一个 chunk 中包含 usage 统计信息。默认 `false`。仅在 `stream: true` 时有效 |

示例：

```json
{
  "stream": true,
  "stream_options": {
    "include_usage": true
  }
}
```

### 3.3 audio 参数

当 `modalities` 包含 `"audio"` 时，需要指定音频输出参数：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `voice` | string | 是 | 语音类型：`alloy`、`ash`、`ballad`、`coral`、`echo`、`sage`、`shimmer`、`verse` |
| `format` | string | 是 | 音频格式：`wav`、`mp3`、`flac`、`opus`、`pcm16` |

示例：

```json
{
  "modalities": ["text", "audio"],
  "audio": {
    "voice": "alloy",
    "format": "mp3"
  }
}
```

### 3.4 prediction 参数（Predicted Output）

当已知大部分响应内容（如重新生成仅做少量修改的文件）时，可使用预测输出加速：

```json
{
  "prediction": {
    "type": "content",
    "content": "已知的预期输出内容..."
  }
}
```

### 3.5 web_search_options 参数

```json
{
  "web_search_options": {
    "search_context_size": "medium"
  }
}
```

`search_context_size` 取值：`low`、`medium`、`high`。

### 3.6 reasoning_effort 说明

| 模型 | 默认值 | 支持的值 | 说明 |
|------|--------|---------|------|
| `gpt-5.1` | `none` | `none`、`low`、`medium`、`high` | `none` 不执行推理；所有级别均支持工具调用 |
| `gpt-5.1` 之前的模型 | `medium` | `minimal`、`low`、`medium`、`high` | 不支持 `none` |
| `gpt-5-pro` | `high` | `high` | 仅支持 `high` |
| `gpt-5.1-codex-max` 之后的模型 | `medium` | 含 `xhigh` | 支持最高 `xhigh` |

---

## 4. 消息类型

`messages` 数组中的每条消息都包含 `role` 字段，不同角色对应不同的内容结构。

### 4.1 system / developer 消息

设定模型的行为和上下文。`developer` 是新版名称（推荐用于新模型如 gpt-4.1+），`system` 是传统名称。

**纯文本系统消息：**

```json
{
  "role": "system",
  "content": "You are a helpful assistant that translates English to French."
}
```

**新版 developer 消息：**

```json
{
  "role": "developer",
  "content": "You are a helpful assistant."
}
```

> 在协议转换时，`system` 和 `developer` 角色通常映射为等价的系统指令。部分供应商只支持 `system`，需要将 `developer` 转换为 `system`。

### 4.2 user 消息

用户输入消息，支持纯文本和多模态内容。

#### 纯文本

```json
{
  "role": "user",
  "content": "What is the capital of France?"
}
```

#### 多模态内容（文本 + 图片）

```json
{
  "role": "user",
  "content": [
    {
      "type": "text",
      "text": "What's in this image?"
    },
    {
      "type": "image_url",
      "image_url": {
        "url": "https://example.com/image.png",
        "detail": "high"
      }
    }
  ]
}
```

**image_url.detail 取值：**

| 值 | 说明 |
|---|------|
| `auto` | 自动选择（默认） |
| `low` | 低分辨率，更快更省 token |
| `high` | 高分辨率，更详细但消耗更多 token |

**image_url.url 支持格式：**
- HTTPS URL：`https://example.com/image.png`
- Base64 Data URI：`data:image/png;base64,iVBORw0KGgo...`

#### 音频输入

```json
{
  "role": "user",
  "content": [
    {
      "type": "input_audio",
      "input_audio": {
        "data": "base64编码的音频数据",
        "format": "wav"
      }
    },
    {
      "type": "text",
      "text": "What is being said in this audio?"
    }
  ]
}
```

**input_audio.format 取值：** `wav`、`mp3`、`flac`、`opus`、`pcm16`

#### 文件附件

```json
{
  "role": "user",
  "content": [
    {
      "type": "file",
      "file": {
        "file_id": "file-abc123",
        "file_data": "base64编码的文件数据",
        "filename": "document.pdf"
      }
    }
  ]
}
```

### 4.3 assistant 消息

模型生成的响应消息。

#### 纯文本响应

```json
{
  "role": "assistant",
  "content": "The capital of France is Paris."
}
```

#### 带工具调用的响应

```json
{
  "role": "assistant",
  "content": null,
  "tool_calls": [
    {
      "id": "call_abc123",
      "type": "function",
      "function": {
        "name": "get_weather",
        "arguments": "{\"location\": \"Paris\", \"unit\": \"celsius\"}"
      }
    }
  ]
}
```

**tool_calls 数组中每个元素的结构：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 工具调用的唯一 ID，格式为 `call_xxxxx` |
| `type` | string | 固定为 `"function"` |
| `function.name` | string | 要调用的函数名 |
| `function.arguments` | string | JSON 格式的函数参数（**字符串类型**，非对象） |
| `index` | integer | 可选，在流式响应中用于标识调用位置 |

#### 推理内容（部分推理模型）

```json
{
  "role": "assistant",
  "content": "最终回答内容",
  "reasoning_content": "这是模型的思考过程..."
}
```

> `reasoning_content` 字段在 DeepSeek、o1 等推理模型的响应中出现，包含模型的内部推理过程。

### 4.4 tool 消息

将工具执行结果返回给模型。

```json
{
  "role": "tool",
  "tool_call_id": "call_abc123",
  "content": "{\"temperature\": 22, \"unit\": \"celsius\"}"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role` | string | 是 | 固定为 `"tool"` |
| `tool_call_id` | string | 是 | 对应的 assistant 消息中 `tool_calls[].id` |
| `content` | string | 是 | 工具执行结果，通常是 JSON 字符串 |

### 4.5 消息角色汇总

| role | 发起方 | content 类型 | 特殊字段 |
|------|--------|-------------|---------|
| `system` | 客户端 | string | — |
| `developer` | 客户端 | string | — |
| `user` | 客户端 | string 或 array（多模态） | — |
| `assistant` | 模型 | string 或 null | `tool_calls`、`reasoning_content` |
| `tool` | 客户端 | string | `tool_call_id`（必填） |

---

## 5. 响应格式

### 5.1 完整响应结构

```json
{
  "id": "chatcmpl-B9MBs8CjcvOU2jLn4n570S5qMJKcT",
  "object": "chat.completion",
  "created": 1741569952,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I assist you today?",
        "refusal": null,
        "annotations": []
      },
      "logprobs": null,
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 19,
    "completion_tokens": 10,
    "total_tokens": 29,
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
  },
  "service_tier": "default",
  "system_fingerprint": "fp_fc9f1d7035"
}
```

### 5.2 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 完成请求的唯一标识符，格式 `chatcmpl-xxx` |
| `object` | string | 对象类型，固定为 `"chat.completion"` |
| `created` | integer | 创建时间（Unix 时间戳，秒） |
| `model` | string | 实际使用的模型 ID（可能与请求中的 `model` 不同） |
| `choices` | array | 响应选择项数组，见下方 |
| `usage` | object | Token 使用量统计，见下方 |
| `service_tier` | string | 实际使用的处理层级：`default`、`flex`、`priority` |
| `system_fingerprint` | string | **已弃用**。后端配置指纹，配合 `seed` 用于监控后端变更 |

### 5.3 choice 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `index` | integer | 选择项索引（当 `n > 1` 时有用） |
| `message` | object | 模型生成的消息对象 |
| `message.role` | string | 固定为 `"assistant"` |
| `message.content` | string / null | 模型生成的文本内容，当模型调用工具时可能为 `null` |
| `message.refusal` | string / null | 模型拒绝生成时的拒绝原因（仅当使用 structured outputs 时） |
| `message.annotations` | array | 注解列表（如 URL 引用来源等） |
| `message.tool_calls` | array / null | 工具调用列表，见 [第 7 节](#7-tool-calling-工作流) |
| `logprobs` | object / null | 对数概率信息（当 `logprobs: true` 时） |
| `finish_reason` | string | 完成原因，见 [第 10 节](#10-finish_reason-取值说明) |

### 5.4 usage 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `prompt_tokens` | integer | 输入 token 总数 |
| `completion_tokens` | integer | 输出 token 总数 |
| `total_tokens` | integer | token 总数 |
| `prompt_tokens_details` | object | 输入 token 细分（可选） |
| `prompt_tokens_details.cached_tokens` | integer | 缓存命中的 token 数 |
| `prompt_tokens_details.audio_tokens` | integer | 音频输入 token 数 |
| `completion_tokens_details` | object | 输出 token 细分（可选） |
| `completion_tokens_details.reasoning_tokens` | integer | 推理 token 数（o 系列模型） |
| `completion_tokens_details.audio_tokens` | integer | 音频输出 token 数 |
| `completion_tokens_details.accepted_prediction_tokens` | integer | 被 `prediction` 参数接受的预测 token 数 |
| `completion_tokens_details.rejected_prediction_tokens` | integer | 被 `prediction` 参数拒绝的预测 token 数 |

### 5.5 带工具调用的响应示例

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1741569952,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": null,
        "tool_calls": [
          {
            "id": "call_abc123",
            "type": "function",
            "function": {
              "name": "get_weather",
              "arguments": "{\"location\": \"Paris\", \"unit\": \"celsius\"}"
            }
          }
        ]
      },
      "logprobs": null,
      "finish_reason": "tool_calls"
    }
  ],
  "usage": {
    "prompt_tokens": 82,
    "completion_tokens": 17,
    "total_tokens": 99
  }
}
```

---

## 6. 流式响应 (SSE)

### 6.1 基本格式

当 `stream: true` 时，响应使用 Server-Sent Events (SSE) 格式：

- **Content-Type**: `text/event-stream`
- 每个 chunk 格式：`data: {JSON}\n\n`
- 流结束标记：`data: [DONE]\n\n`
- 每个chunk的 `object` 类型为 `"chat.completion.chunk"`

### 6.2 完整流式示例

```
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" How"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" can"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" I"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" help"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":" you"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"?"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":19,"completion_tokens":10,"total_tokens":29,"prompt_tokens_details":{"cached_tokens":0,"audio_tokens":0},"completion_tokens_details":{"reasoning_tokens":0,"audio_tokens":0,"accepted_prediction_tokens":0,"rejected_prediction_tokens":0}}}

data: [DONE]
```

### 6.3 流式 chunk 结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 与非流式响应相同的 completion ID |
| `object` | string | 固定为 `"chat.completion.chunk"` |
| `created` | integer | Unix 时间戳 |
| `model` | string | 实际使用的模型 |
| `choices` | array | 流式选择项数组 |
| `choices[].index` | integer | 选择项索引 |
| `choices[].delta` | object | 增量内容 |
| `choices[].delta.role` | string | 仅在第一个 chunk 中出现 |
| `choices[].delta.content` | string | 增量文本内容 |
| `choices[].delta.tool_calls` | array | 增量工具调用（流式工具调用场景） |
| `choices[].finish_reason` | string / null | 结束原因，最后一个 chunk 非 null |
| `choices[].logprobs` | object / null | 对数概率 |
| `usage` | object / null | 仅在最后一个 chunk 中出现（需 `stream_options.include_usage: true`） |
| `system_fingerprint` | string | 后端指纹 |

### 6.4 流式中的 tool_calls

当模型在流式响应中调用工具时，`delta.tool_calls` 增量拼接：

```
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc123","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"lo"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"cation"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\": \"Paris\"}"}}]},"finish_reason":null}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1741569952,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}
```

**关键点**：
- 第一个 chunk 包含 `id`、`type`、`function.name`
- 后续 chunk 通过 `function.arguments` 增量拼接参数字符串
- `index` 字段用于标识多个并行工具调用的位置
- 最后一个 chunk 的 `finish_reason` 为 `"tool_calls"`

### 6.5 include_usage 的使用

当设置 `stream_options.include_usage: true` 时，在 `data: [DONE]` 之前会发送一个包含完整 usage 的 chunk：

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion.chunk",
  "created": 1741569952,
  "model": "gpt-4o",
  "choices": [],
  "usage": {
    "prompt_tokens": 19,
    "completion_tokens": 10,
    "total_tokens": 29,
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
}
```

> 此 chunk 的 `choices` 为空数组，仅用于传递 usage 信息。

---

## 7. Tool Calling 工作流

### 7.1 工具定义格式

```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get current weather for a location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "City name, e.g. Paris"
            },
            "unit": {
              "type": "string",
              "enum": ["celsius", "fahrenheit"]
            }
          },
          "required": ["location"]
        },
        "strict": true
      }
    }
  ]
}
```

**function 对象字段：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 函数名称 |
| `description` | string | 否 | 函数描述，帮助模型决定何时调用 |
| `parameters` | object | 否 | JSON Schema 格式的参数定义 |
| `strict` | boolean | 否 | 是否启用严格模式（确保参数严格匹配 schema）。仅 `json_schema` 模式下有效 |

### 7.2 tool_choice 取值

| 值 | 说明 |
|---|------|
| `"none"` | 模型不调用任何工具，始终生成消息（无工具时的默认值） |
| `"auto"` | 模型自行决定生成消息或调用工具（有工具时的默认值） |
| `"required"` | 模型必须调用一个或多个工具 |
| `{"type": "function", "function": {"name": "my_func"}}` | 强制调用指定函数 |

### 7.3 完整工具调用流程

```
步骤 1: 客户端发送带工具定义的请求
────────────────────────────────────────
POST /v1/chat/completions
{
  "model": "gpt-4o",
  "messages": [
    {"role": "user", "content": "What's the weather in Paris?"}
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather for a location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {"type": "string"},
            "unit": {"type": "string", "enum": ["celsius", "fahrenheit"]}
          },
          "required": ["location"]
        }
      }
    }
  ]
}

步骤 2: 模型返回工具调用请求
────────────────────────────────────────
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": null,
      "tool_calls": [{
        "id": "call_abc123",
        "type": "function",
        "function": {
          "name": "get_weather",
          "arguments": "{\"location\":\"Paris\",\"unit\":\"celsius\"}"
        }
      }]
    },
    "finish_reason": "tool_calls"
  }]
}

步骤 3: 客户端执行工具，将结果追加到消息列表
────────────────────────────────────────
POST /v1/chat/completions
{
  "model": "gpt-4o",
  "messages": [
    {"role": "user", "content": "What's the weather in Paris?"},
    {"role": "assistant", "content": null, "tool_calls": [
      {
        "id": "call_abc123",
        "type": "function",
        "function": {
          "name": "get_weather",
          "arguments": "{\"location\":\"Paris\",\"unit\":\"celsius\"}"
        }
      }
    ]},
    {"role": "tool", "tool_call_id": "call_abc123", "content": "{\"temperature\": 22, \"condition\": \"sunny\"}"}
  ],
  "tools": [ ... 同上 ... ]
}

步骤 4: 模型根据工具结果生成最终响应
────────────────────────────────────────
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": "The current weather in Paris is sunny with a temperature of 22 degrees Celsius."
    },
    "finish_reason": "stop"
  }]
}
```

### 7.4 并行工具调用

当 `parallel_tool_calls: true`（默认）时，模型可能在一个响应中调用多个工具：

```json
{
  "role": "assistant",
  "content": null,
  "tool_calls": [
    {
      "id": "call_001",
      "type": "function",
      "function": {"name": "get_weather", "arguments": "{\"location\":\"Paris\"}"}
    },
    {
      "id": "call_002",
      "type": "function",
      "function": {"name": "get_weather", "arguments": "{\"location\":\"Tokyo\"}"}
    }
  ]
}
```

客户端需要为每个工具调用分别返回结果：

```json
[
  {"role": "tool", "tool_call_id": "call_001", "content": "{\"temperature\": 22}"},
  {"role": "tool", "tool_call_id": "call_002", "content": "{\"temperature\": 18}"}
]
```

---

## 8. Structured Outputs

### 8.1 response_format 取值

#### 纯文本模式（默认）

```json
{
  "response_format": {
    "type": "text"
  }
}
```

#### JSON 模式（旧版）

```json
{
  "response_format": {
    "type": "json_object"
  }
}
```

> 确保 `messages` 中包含要求输出 JSON 的指令，否则模型可能输出空白。

#### JSON Schema 模式（推荐，Structured Outputs）

```json
{
  "response_format": {
    "type": "json_schema",
    "json_schema": {
      "name": "weather_response",
      "strict": true,
      "schema": {
        "type": "object",
        "properties": {
          "location": {"type": "string"},
          "temperature": {"type": "number"},
          "unit": {"type": "string", "enum": ["celsius", "fahrenheit"]}
        },
        "required": ["location", "temperature", "unit"],
        "additionalProperties": false
      }
    }
  }
}
```

**json_schema 对象字段：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | Schema 名称 |
| `strict` | boolean | 否 | 设为 `true` 启用严格模式（推荐） |
| `schema` | object | 是 | JSON Schema 定义 |
| `description` | string | 否 | Schema 描述 |

**严格模式要求：**
- `additionalProperties` 必须设为 `false`
- 所有字段必须在 `required` 中列出（如需可选，使用单元素 `anyOf`）
- 嵌套对象也必须遵循以上规则

### 8.2 拒绝行为

当使用 Structured Outputs 时，如果模型拒绝生成，`message.refusal` 字段非 null：

```json
{
  "message": {
    "role": "assistant",
    "content": null,
    "refusal": "I'm sorry, but I cannot assist with that request."
  }
}
```

---

## 9. 错误格式

### 9.1 错误响应结构

```json
{
  "error": {
    "message": "Incorrect API key provided: sk-xxxx...xxxx.",
    "type": "invalid_request_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `error.message` | string | 人类可读的错误描述 |
| `error.type` | string | 错误类型 |
| `error.param` | string / null | 导致错误的参数名 |
| `error.code` | string / null | 错误码 |

### 9.2 错误类型映射

| error.type | HTTP 状态码 | 说明 |
|-----------|------------|------|
| `invalid_request_error` | 400 | 请求参数错误、模型不存在 |
| `authentication_error` | 401 | API Key 无效、缺失 |
| `permission_error` | 403 | 无权访问该模型或资源 |
| `insufficient_quota` | 402 | 余额不足、额度耗尽 |
| `rate_limit_error` | 429 | 请求频率超限 |
| `server_error` | 500 / 503 | 服务器内部错误、服务不可用 |

### 9.3 常见错误示例

**认证错误（401）：**

```json
{
  "error": {
    "message": "Incorrect API key provided.",
    "type": "authentication_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

**频率限制（429）：**

```json
{
  "error": {
    "message": "You exceeded your current quota, please check your plan and billing details.",
    "type": "rate_limit_error",
    "param": null,
    "code": "insufficient_quota"
  }
}
```

**请求参数错误（400）：**

```json
{
  "error": {
    "message": "Unsupported value: 'messages[0].role' does not support 'foo' value.",
    "type": "invalid_request_error",
    "param": "messages[0].role",
    "code": null
  }
}
```

**模型不存在（404）：**

```json
{
  "error": {
    "message": "The model `gpt-5-nonexistent` does not exist.",
    "type": "invalid_request_error",
    "param": null,
    "code": "model_not_found"
  }
}
```

**内容过滤：**

```json
{
  "error": {
    "message": "The content was filtered due to policy violations.",
    "type": "invalid_request_error",
    "param": null,
    "code": "content_filter"
  }
}
```

---

## 10. finish_reason 取值说明

| 值 | 说明 |
|---|------|
| `stop` | 模型自然结束输出或命中了 `stop` 序列 |
| `length` | 输出达到 `max_completion_tokens` / `max_tokens` 上限而被截断 |
| `tool_calls` | 模型决定调用一个或多个工具 |
| `content_filter` | 输出因内容过滤策略被截断 |

---

## 11. 协议转换注意事项

> 本节针对 team-api 的协议转换模块开发，总结在实现 OpenAI 与其他供应商协议互转时需要注意的要点。

### 11.1 请求参数映射

| OpenAI 参数 | Claude Messages | Gemini | 注意事项 |
|------------|----------------|--------|---------|
| `model` | `model` | `model` | 需做模型名称映射 |
| `messages` | `messages` | `contents` | 角色和内容结构完全不同 |
| `max_completion_tokens` | `max_tokens` | `generationConfig.maxOutputTokens` | 字段名不同 |
| `temperature` | `temperature` | `generationConfig.temperature` | 语义相同 |
| `top_p` | `top_p` | `generationConfig.topP` | 语义相同 |
| `stream` | `stream` | `stream` | 流式协议格式不同 |
| `tools` | `tools` | `tools` | 结构差异较大 |
| `tool_choice` | `tool_choice` | `tool_config` | 值的格式不同 |
| `stop` | `stop_sequences` | `generationConfig.stopSequences` | 数组格式 |
| `response_format` | N/A | `responseMimeType` | Claude 不支持 JSON Schema |
| `reasoning_effort` | `thinking.budget_tokens` | N/A | 推理控制方式完全不同 |
| `presence_penalty` | N/A | N/A | Claude/Gemini 不支持 |
| `frequency_penalty` | N/A | N/A | Claude/Gemini 不支持 |
| `logprobs` | N/A | N/A | Claude/Gemini 不支持 |

### 11.2 消息角色映射

| OpenAI role | Claude role | Gemini role |
|------------|------------|-------------|
| `system` | `system` | `systemInstruction` |
| `developer` | `system` | `systemInstruction` |
| `user` | `user` | `user` |
| `assistant` | `assistant` | `model` |
| `tool` | `tool_result` | `functionCall` / `functionResponse` |

### 11.3 usage 字段映射

| OpenAI Chat Completions | OpenAI Responses API | 说明 |
|------------------------|---------------------|------|
| `prompt_tokens` | `input_tokens` | 字段名不同 |
| `completion_tokens` | `output_tokens` | 字段名不同 |
| `total_tokens` | `total_tokens` | 相同 |
| `prompt_tokens_details` | `input_tokens_details` | 字段名不同 |
| `completion_tokens_details` | `output_tokens_details` | 字段名不同 |

### 11.4 流式协议差异

| 特性 | OpenAI | Claude |
|------|--------|--------|
| Content-Type | `text/event-stream` | `text/event-stream` |
| 数据格式 | `data: {JSON}\n\n` | `event: xxx\ndata: {JSON}\n\n` |
| 结束标记 | `data: [DONE]` | `event: message_stop` |
| 内容字段 | `choices[].delta.content` | `delta.text` |
| 事件类型 | 无 event 行 | `message_start`、`content_block_start`、`content_block_delta`、`message_delta`、`message_stop` |

### 11.5 工具调用差异

| 特性 | OpenAI | Claude |
|------|--------|--------|
| 工具定义位置 | `tools` 数组 | `tools` 数组（结构不同） |
| 调用结果返回 | `role: "tool"` 消息 | `role: "user"` 消息内嵌 `tool_result` content block |
| 调用 ID 字段 | `tool_call_id`（消息级） | `tool_use_id`（content block 级） |
| 并行调用 | `parallel_tool_calls` 参数控制 | 天然支持多个 tool_use content block |

### 11.6 关键实现提醒

1. **`function.arguments` 是字符串**：OpenAI 的 `tool_calls[].function.arguments` 是 JSON 字符串而非 JSON 对象，解析时需要先 `json.Unmarshal`。

2. **`content` 可以是字符串或数组**：user 消息的 `content` 字段有两种形态 —— 纯文本时为 `string`，多模态时为 `[]ContentPart` 数组。协议转换时需要判断类型。

3. **流式 tool_calls 增量拼接**：流式响应中的 tool_calls 是增量发送的，需要按 `index` 拼接 `function.arguments` 字符串。

4. **`system` 与 `developer` 角色**：新模型（gpt-4.1+）推荐使用 `developer`，旧模型使用 `system`。转换到其他供应商时统一映射为系统指令。

5. **`max_tokens` 与 `max_completion_tokens`**：`max_tokens` 已弃用但仍有大量客户端使用，实现时需兼容两个字段，优先使用 `max_completion_tokens`。

6. **推理模型的差异**：o 系列推理模型不支持 `temperature`、`top_p`、`presence_penalty`、`frequency_penalty`、`logprobs`、`logit_bias` 等参数，转换时需过滤。

7. **`stop` 参数的限制**：o3 和 o4-mini 不支持 `stop` 参数，转发时需忽略。

8. **`stream_options.include_usage`**：部分客户端依赖此选项获取流式响应的 usage 信息，实现时需要确保最后一个 chunk 包含 usage。

---

## 附录：项目代码参考

本项目中 OpenAI 协议相关的数据结构定义在以下文件中：

| 文件 | 内容 |
|------|------|
| `relay/dto/openai.go` | Chat Completions 请求/响应 DTO（`GeneralOpenAIRequest`、`ChatCompletionResponse` 等） |
| `relay/dto/openai_responses.go` | Responses API 请求/响应 DTO |
| `relay/dto/usage.go` | Token 使用量 DTO、文本补全/嵌入/图像生成 DTO |
| `relay/channel/openai/` | OpenAI 适配器实现 |
