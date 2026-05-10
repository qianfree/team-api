# Anthropic Claude Messages API 协议文档

> 基于 Anthropic 官方 API 文档整理，适用于 team-api 项目 Relay 层的 Claude 协议适配开发。
>
> 官方文档地址：https://docs.anthropic.com/en/api/messages

---

## 目录

- [1. API 概览](#1-api-概览)
- [2. 请求头 (Headers)](#2-请求头-headers)
- [3. 请求体参数 (Request Body)](#3-请求体参数-request-body)
- [4. 消息内容块类型 (Content Block Types)](#4-消息内容块类型-content-block-types)
- [5. 工具定义 (Tools)](#5-工具定义-tools)
- [6. tool_choice 配置](#6-tool_choice-配置)
- [7. 响应格式 (Response)](#7-响应格式-response)
- [8. stop_reason 值](#8-stop_reason-值)
- [9. 流式响应 (SSE Streaming)](#9-流式响应-sse-streaming)
- [10. 错误格式](#10-错误格式)
- [11. Prompt Caching (cache_control)](#11-prompt-caching-cache_control)

---

## 1. API 概览

| 属性 | 说明 |
|------|------|
| **端点** | `POST https://api.anthropic.com/v1/messages` |
| **功能** | 发送结构化的输入消息列表（文本和/或图像），模型生成对话中的下一条消息 |
| **用途** | 支持单轮查询和无状态多轮对话 |
| **Content-Type** | `application/json` |
| **当前 API 版本** | `2023-06-01` |

### 基本请求示例

```bash
curl https://api.anthropic.com/v1/messages \
     --header "x-api-key: $ANTHROPIC_API_KEY" \
     --header "anthropic-version: 2023-06-01" \
     --header "content-type: application/json" \
     --data '{
         "model": "claude-sonnet-4-20250514",
         "max_tokens": 1024,
         "messages": [
             {"role": "user", "content": "Hello, world"}
         ]
     }'
```

### 基本响应示例

```json
{
  "content": [
    {
      "text": "Hi! My name is Claude.",
      "type": "text"
    }
  ],
  "id": "msg_013Zva2CMHLNnXjNJJKqJ2EF",
  "model": "claude-sonnet-4-20250514",
  "role": "assistant",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "type": "message",
  "usage": {
    "input_tokens": 2095,
    "output_tokens": 503
  }
}
```

---

## 2. 请求头 (Headers)

| 请求头 | 是否必填 | 类型 | 说明 |
|--------|---------|------|------|
| `x-api-key` | **必填** | string | API 密钥，用于身份认证。通过 Anthropic Console 获取，每个密钥绑定一个 Workspace |
| `anthropic-version` | **必填** | string | API 版本号，当前为 `2023-06-01` |
| `anthropic-beta` | 可选 | string[] | Beta 功能标识。多个 Beta 用逗号分隔（如 `beta1,beta2`），或多次指定该请求头 |
| `content-type` | **必填** | string | 固定为 `application/json` |

---

## 3. 请求体参数 (Request Body)

### 完整参数一览

| 参数 | 是否必填 | 类型 | 默认值 | 说明 |
|------|---------|------|--------|------|
| `model` | **必填** | string | — | 模型 ID，如 `claude-sonnet-4-20250514`。长度限制：1-256 字符 |
| `messages` | **必填** | object[] | — | 输入消息列表。最大 100,000 条消息 |
| `max_tokens` | **必填** | integer | — | 最大生成 token 数，必须 >= 1 |
| `system` | 可选 | string / object[] | — | 系统提示词。可以是纯字符串或 content_block 数组 |
| `temperature` | 可选 | number | `1` | 随机性控制，范围 `0.0` ~ `1.0`。接近 0 适合分析/选择题，接近 1 适合创意/生成任务 |
| `top_p` | 可选 | number | — | 核采样 (Nucleus Sampling)，范围 `0` ~ `1`。与 `temperature` 二选一调整 |
| `top_k` | 可选 | integer | — | 仅从概率最高的 K 个 token 中采样，>= 0。用于去除低概率"长尾"响应 |
| `stop_sequences` | 可选 | string[] | — | 自定义停止序列列表。模型匹配到任一序列时停止生成 |
| `stream` | 可选 | boolean | `false` | 是否使用 SSE 流式响应 |
| `tools` | 可选 | object[] | — | 工具定义列表。模型可返回 `tool_use` 内容块 |
| `tool_choice` | 可选 | object | — | 工具调用策略：auto / any / tool / none |
| `thinking` | 可选 | object | — | 扩展思考配置 `{ type: "enabled", budget_tokens: >=1024 }` |
| `metadata` | 可选 | object | — | 请求元数据，含 `user_id` 字段 |
| `service_tier` | 可选 | string | — | 服务层级：`auto` 或 `standard_only` |
| `mcp_servers` | 可选 | object[] | — | MCP 服务器配置 |
| `container` | 可选 | string / null | — | 容器标识，用于跨请求复用容器 |

### messages 参数详解

每条消息包含以下字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `role` | enum | `user` 或 `assistant` |
| `content` | string / object[] | 消息内容。字符串等价于 `[{"type": "text", "text": "..."}]` |

**关键规则：**

- 模型按交替的 `user` / `assistant` 轮次训练。连续相同角色的消息会被合并。
- 没有独立的 `system` 角色，系统提示词通过顶层 `system` 参数传入。
- 若最后一条消息为 `assistant` 角色，响应内容会从该消息末尾继续，可用于约束模型输出。

**示例：**

```json
// 单轮对话
[{"role": "user", "content": "Hello, Claude"}]

// 多轮对话
[
  {"role": "user", "content": "Hello there."},
  {"role": "assistant", "content": "Hi, I'm Claude. How can I help you?"},
  {"role": "user", "content": "Can you explain LLMs in plain English?"}
]

// 约束模型输出（prefilled response）
[
  {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
  {"role": "assistant", "content": "The best answer is ("}
]
```

### thinking 参数详解

启用扩展思考后，响应将包含 `thinking` 内容块，展示模型的思考过程。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定为 `"enabled"` |
| `budget_tokens` | integer | 是 | 思考 token 预算，必须 >= 1024 且小于 `max_tokens` |

```json
{
  "thinking": {
    "type": "enabled",
    "budget_tokens": 16000
  },
  "max_tokens": 20000
}
```

### metadata 参数详解

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_id` | string / null | 用户外部标识，建议使用 UUID 或哈希值。最大长度 256 字符。不要包含姓名、邮箱等个人信息 |

### mcp_servers 参数详解

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 服务器名称 |
| `type` | string | 是 | 固定为 `"url"` |
| `url` | string | 是 | MCP 服务器 URL |
| `authorization_token` | string / null | 否 | 授权令牌 |
| `tool_configuration` | object / null | 否 | 工具配置 |
| `tool_configuration.enabled` | boolean / null | 否 | 是否启用 |
| `tool_configuration.allowed_tools` | string[] / null | 否 | 允许的工具列表 |

---

## 4. 消息内容块类型 (Content Block Types)

### 4.1 输入内容块 (Input Content Blocks)

#### text — 文本

```json
{
  "type": "text",
  "text": "描述内容..."
}
```

#### image — 图像

图像支持两种来源方式：

**Base64 编码：**

```json
{
  "type": "image",
  "source": {
    "type": "base64",
    "media_type": "image/jpeg",
    "data": "/9j/4AAQSkZJRg..."
  }
}
```

**URL 引用：**

```json
{
  "type": "image",
  "source": {
    "type": "url",
    "url": "https://example.com/image.jpg"
  }
}
```

支持的 `media_type`：`image/jpeg`、`image/png`、`image/gif`、`image/webp`。

#### tool_use — 工具调用（assistant 消息中）

```json
{
  "type": "tool_use",
  "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
  "name": "get_weather",
  "input": {
    "location": "San Francisco, CA"
  }
}
```

#### tool_result — 工具结果（user 消息中）

```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
  "content": "259.75 USD",
  "is_error": false
}
```

`content` 可以是字符串或内容块数组。`is_error` 为可选布尔值，标记是否为错误结果。

#### thinking — 思考过程（assistant 消息中）

```json
{
  "type": "thinking",
  "thinking": "Let me solve this step by step...",
  "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGg..."
}
```

#### redacted_thinking — 脱敏思考

```json
{
  "type": "redacted_thinking",
  "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGg..."
}
```

### 4.2 输出内容块 (Output Content Blocks)

| 类型 | 说明 | 结构 |
|------|------|------|
| `text` | 文本内容 | `{ type, text }` |
| `tool_use` | 工具调用 | `{ type, id, name, input }` |
| `thinking` | 扩展思考 | `{ type, thinking, signature }` |
| `redacted_thinking` | 脱敏思考 | `{ type, signature }` |
| `server_tool_use` | 服务器工具调用 | `{ type, id, name, input }` |
| `web_search_tool_result` | 网页搜索结果 | `{ type, tool_use_id, content, status_code }` |
| `code_execution_tool_result` | 代码执行结果 | `{ type, tool_use_id, content }` |
| `mcp_tool_use` | MCP 工具调用 | `{ type, id, name, input, server_name }` |
| `mcp_tool_result` | MCP 工具结果 | `{ type, tool_use_id, content, is_error }` |
| `container_upload` | 容器上传 | `{ type, ... }` |

---

## 5. 工具定义 (Tools)

### 自定义工具 (Custom Tool)

每个自定义工具定义包含以下字段：

| 字段 | 是否必填 | 类型 | 说明 |
|------|---------|------|------|
| `name` | **必填** | string | 工具名称，长度 1-128 字符。模型通过此名称调用工具 |
| `input_schema` | **必填** | object | JSON Schema 对象，定义工具的输入结构。必须包含 `type: "object"` |
| `type` | 可选 | string | 工具类型，固定为 `"custom"` |
| `description` | 可选 | string | 工具描述。建议尽可能详细，帮助模型更好地理解和使用工具 |
| `cache_control` | 可选 | object | 缓存控制断点 `{ type: "ephemeral", ttl: "5m" / "1h" }` |

**input_schema 结构：**

```json
{
  "type": "object",
  "properties": {
    "location": {
      "type": "string",
      "description": "The city and state, e.g. San Francisco, CA"
    },
    "unit": {
      "type": "string",
      "description": "Unit for the output - one of (celsius, fahrenheit)"
    }
  },
  "required": ["location"]
}
```

**完整工具定义示例：**

```json
[
  {
    "name": "get_stock_price",
    "description": "Get the current stock price for a given ticker symbol.",
    "input_schema": {
      "type": "object",
      "properties": {
        "ticker": {
          "type": "string",
          "description": "The stock ticker symbol, e.g. AAPL for Apple Inc."
        }
      },
      "required": ["ticker"]
    }
  }
]
```

### 内置工具类型

以下为 Anthropic 提供的服务器端内置工具：

| 工具类型 | Beta 版本标识 | 说明 |
|----------|--------------|------|
| `bash` | `2024-10-22` / `2025-01-24` | Bash 命令执行 |
| `code_execution` | `2025-05-22` | 代码沙箱执行 |
| `computer_use` | `2024-01-22` / `2025-01-24` | 计算机使用（屏幕操作） |
| `text_editor` | `2024-10-22` / `2025-01-24` / `2025-04-29` | 文本编辑器操作 |
| `web_search` | `2025-03-05` | 网页搜索 |

使用内置工具时需要在 `anthropic-beta` 请求头中指定对应的 Beta 版本标识。

### 工具调用完整流程示例

**1. 定义工具并发送请求：**

```json
{
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 1024,
  "tools": [
    {
      "name": "get_weather",
      "description": "Get the current weather in a given location",
      "input_schema": {
        "type": "object",
        "properties": {
          "location": {
            "type": "string",
            "description": "The city and state, e.g. San Francisco, CA"
          }
        },
        "required": ["location"]
      }
    }
  ],
  "messages": [
    {"role": "user", "content": "What is the weather in San Francisco?"}
  ]
}
```

**2. 模型返回 tool_use：**

```json
{
  "content": [
    {
      "type": "text",
      "text": "Let me check the weather for you."
    },
    {
      "type": "tool_use",
      "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
      "name": "get_weather",
      "input": {"location": "San Francisco, CA"}
    }
  ],
  "stop_reason": "tool_use"
}
```

**3. 将工具结果传回模型（下一条 user 消息）：**

```json
{
  "messages": [
    {"role": "user", "content": "What is the weather in San Francisco?"},
    {"role": "assistant", "content": [
      {"type": "text", "text": "Let me check the weather for you."},
      {"type": "tool_use", "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV", "name": "get_weather", "input": {"location": "San Francisco, CA"}}
    ]},
    {"role": "user", "content": [
      {"type": "tool_result", "tool_use_id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV", "content": "72°F, Sunny"}
    ]}
  ]
}
```

---

## 6. tool_choice 配置

控制模型如何使用提供的工具。

### 选项一览

| 类型 | 结构 | 说明 |
|------|------|------|
| **auto** | `{ "type": "auto" }` | 自动决定是否使用工具（默认行为） |
| **any** | `{ "type": "any" }` | 必须调用任意一个工具，但不指定具体哪个 |
| **tool** | `{ "type": "tool", "name": "get_weather" }` | 必须调用指定名称的工具 |
| **none** | `{ "type": "none" }` | 禁止使用任何工具 |

### disable_parallel_tool_use

每种 `tool_choice` 类型都可以附加 `disable_parallel_tool_use` 布尔字段：

```json
{
  "type": "auto",
  "disable_parallel_tool_use": true
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `disable_parallel_tool_use` | boolean | `false` | 设为 `true` 时，模型最多只输出一个工具调用 |

---

## 7. 响应格式 (Response)

### 完整响应结构

```json
{
  "id": "msg_013Zva2CMHLNnXjNJJKqJ2EF",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hi! My name is Claude."
    },
    {
      "type": "tool_use",
      "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
      "name": "get_weather",
      "input": {"location": "San Francisco, CA"}
    }
  ],
  "model": "claude-sonnet-4-20250514",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 2095,
    "output_tokens": 503,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0,
    "cache_creation": {
      "ephemeral_5m_input_tokens": 0,
      "ephemeral_1h_input_tokens": 0
    },
    "server_tool_use": {
      "web_search_requests": 0
    },
    "service_tier": "standard"
  },
  "container": {
    "id": "container_xxx",
    "expires_at": "2024-01-01T00:00:00Z"
  }
}
```

### 响应字段详解

| 字段 | 类型 | 必返回 | 说明 |
|------|------|--------|------|
| `id` | string | 是 | 唯一消息标识符，格式如 `msg_013Zva2CMHLNnXjNJJKqJ2EF` |
| `type` | string | 是 | 固定为 `"message"` |
| `role` | string | 是 | 固定为 `"assistant"` |
| `content` | object[] | 是 | 内容块数组 |
| `model` | string | 是 | 实际处理请求的模型 ID |
| `stop_reason` | string / null | 是 | 停止原因（见下一节） |
| `stop_sequence` | string / null | 是 | 匹配到的自定义停止序列，未匹配时为 null |
| `usage` | object | 是 | Token 用量统计 |
| `container` | object / null | 是 | 容器信息，使用容器工具时非 null |

### usage 字段详解

| 字段 | 类型 | 说明 |
|------|------|------|
| `input_tokens` | integer | 输入 token 数（不含缓存部分） |
| `output_tokens` | integer | 输出 token 数 |
| `cache_creation_input_tokens` | integer / null | 写入缓存的 token 数 |
| `cache_read_input_tokens` | integer / null | 从缓存读取的 token 数 |
| `cache_creation` | object / null | 按 TTL 细分的缓存创建统计 |
| `cache_creation.ephemeral_5m_input_tokens` | integer | 5 分钟 TTL 缓存写入 token 数 |
| `cache_creation.ephemeral_1h_input_tokens` | integer | 1 小时 TTL 缓存写入 token 数 |
| `server_tool_use` | object / null | 服务器工具使用统计 |
| `server_tool_use.web_search_requests` | integer | 网页搜索请求次数 |
| `service_tier` | string / null | 使用的服务层级：`standard` / `priority` / `batch` |

> **总输入 token 数** = `input_tokens` + `cache_creation_input_tokens` + `cache_read_input_tokens`

### container 字段详解

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 容器标识符 |
| `expires_at` | string | 容器过期时间（ISO 8601 格式） |

---

## 8. stop_reason 值

| 值 | 说明 |
|-----|------|
| `end_turn` | 模型自然完成回复 |
| `max_tokens` | 达到 `max_tokens` 上限或模型最大 token 限制 |
| `stop_sequence` | 匹配到自定义停止序列（`stop_sequences` 参数中定义的） |
| `tool_use` | 模型调用了工具 |
| `pause_turn` | 暂停长轮次。可将响应原样回传，让模型继续 |
| `refusal` | 流式分类器介入处理潜在的政策违规 |

> **注意：** 非流式模式下 `stop_reason` 一定不为 null。流式模式下，在 `message_start` 事件中为 null，其他事件中不为 null。

---

## 9. 流式响应 (SSE Streaming)

设置 `"stream": true` 后，响应通过 Server-Sent Events (SSE) 逐步返回。

### 事件流顺序

```
1. message_start              → 消息开始，包含完整 Message 对象（content 为空数组）
2. [多个内容块，每个包含]：
   a. content_block_start     → 内容块开始
   b. content_block_delta     → 内容增量（一个或多个）
   c. content_block_stop      → 内容块结束
3. message_delta              → 消息级更新（stop_reason、output_tokens）
4. message_stop               → 消息结束
```

期间可能穿插 `ping` 心跳事件和 `error` 错误事件。

### 事件类型详解

#### message_start — 消息开始

包含一个完整的 `message` 对象，`content` 为空数组，`stop_reason` 为 null。

```json
event: message_start
data: {"type": "message_start", "message": {"id": "msg_1nZdL29xx5MUA1yADyHTEsnR8uuvGzszyY", "type": "message", "role": "assistant", "content": [], "model": "claude-sonnet-4-20250514", "stop_reason": null, "stop_sequence": null, "usage": {"input_tokens": 25, "output_tokens": 1}}}
```

#### content_block_start — 内容块开始

```json
event: content_block_start
data: {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}
```

`index` 对应最终 `content` 数组中的下标位置。

对于 tool_use 类型：

```json
event: content_block_start
data: {"type": "content_block_start", "index": 1, "content_block": {"type": "tool_use", "id": "toolu_01T1x1fJ34qAmk2tNTrN7Up6", "name": "get_weather", "input": {}}}
```

对于 thinking 类型：

```json
event: content_block_start
data: {"type": "content_block_start", "index": 0, "content_block": {"type": "thinking", "thinking": ""}}
```

#### content_block_delta — 内容增量

每个 delta 事件包含 `index` 和 `delta` 对象，delta 的 `type` 决定了增量数据的格式：

| delta 类型 | 适用内容块 | 结构 |
|-----------|-----------|------|
| `text_delta` | text | `{ "type": "text_delta", "text": "..." }` |
| `input_json_delta` | tool_use | `{ "type": "input_json_delta", "partial_json": "..." }` |
| `thinking_delta` | thinking | `{ "type": "thinking_delta", "thinking": "..." }` |
| `signature_delta` | thinking | `{ "type": "signature_delta", "signature": "..." }` |

**text_delta 示例：**

```json
event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}}
```

**input_json_delta 示例：**

`partial_json` 是部分 JSON 字符串，需累积后在 `content_block_stop` 时解析为完整 JSON 对象。

```json
event: content_block_delta
data: {"type": "content_block_delta", "index": 1, "delta": {"type": "input_json_delta", "partial_json": "{\"location\": \"San Fra"}}
```

**thinking_delta 示例：**

```json
event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "Let me solve this step by step:\n\n1. First break down 27 * 453"}}
```

**signature_delta 示例：**

在 thinking 内容块的 `content_block_stop` 事件之前发送，用于验证思考块的完整性。

```json
event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "signature_delta", "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds..."}}
```

#### content_block_stop — 内容块结束

```json
event: content_block_stop
data: {"type": "content_block_stop", "index": 0}
```

#### message_delta — 消息级更新

包含 `stop_reason`、`stop_sequence` 和 `output_tokens`：

```json
event: message_delta
data: {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}, "usage": {"output_tokens": 15}}
```

#### message_stop — 消息结束

```json
event: message_stop
data: {"type": "message_stop"}
```

#### ping — 心跳

```json
event: ping
data: {"type": "ping"}
```

#### error — 错误

```json
event: error
data: {"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}
```

### 完整流式响应示例

#### 基本文本生成

**请求：**

```bash
curl https://api.anthropic.com/v1/messages \
     --header "anthropic-version: 2023-06-01" \
     --header "content-type: application/json" \
     --header "x-api-key: $ANTHROPIC_API_KEY" \
     --data '{
       "model": "claude-sonnet-4-20250514",
       "messages": [{"role": "user", "content": "Hello"}],
       "max_tokens": 256,
       "stream": true
     }'
```

**响应：**

```
event: message_start
data: {"type": "message_start", "message": {"id": "msg_1nZdL29xx5MUA1yADyHTEsnR8uuvGzszyY", "type": "message", "role": "assistant", "content": [], "model": "claude-sonnet-4-20250514", "stop_reason": null, "stop_sequence": null, "usage": {"input_tokens": 25, "output_tokens": 1}}}

event: content_block_start
data: {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}

event: ping
data: {"type": "ping"}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "!"}}

event: content_block_stop
data: {"type": "content_block_stop", "index": 0}

event: message_delta
data: {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}, "usage": {"output_tokens": 15}}

event: message_stop
data: {"type": "message_stop"}
```

#### 工具调用流式响应

**请求：**

```bash
curl https://api.anthropic.com/v1/messages \
     -H "content-type: application/json" \
     -H "x-api-key: $ANTHROPIC_API_KEY" \
     -H "anthropic-version: 2023-06-01" \
     -d '{
       "model": "claude-sonnet-4-20250514",
       "max_tokens": 1024,
       "tools": [
         {
           "name": "get_weather",
           "description": "Get the current weather in a given location",
           "input_schema": {
             "type": "object",
             "properties": {
               "location": {
                 "type": "string",
                 "description": "The city and state, e.g. San Francisco, CA"
               }
             },
             "required": ["location"]
           }
         }
       ],
       "tool_choice": {"type": "any"},
       "messages": [
         {"role": "user", "content": "What is the weather like in San Francisco?"}
       ],
       "stream": true
     }'
```

**响应：**

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_014p7gG3wDgGV9EUtLvnow3U","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","stop_sequence":null,"usage":{"input_tokens":472,"output_tokens":2},"content":[],"stop_reason":null}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: ping
data: {"type": "ping"}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Okay, let's check the weather for San Francisco, CA:"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_01T1x1fJ34qAmk2tNTrN7Up6","name":"get_weather","input":{}}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":" \"San Francisc"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"o, CA\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":89}}

event: message_stop
data: {"type":"message_stop"}
```

#### 扩展思考流式响应

**请求：**

```bash
curl https://api.anthropic.com/v1/messages \
     --header "x-api-key: $ANTHROPIC_API_KEY" \
     --header "anthropic-version: 2023-06-01" \
     --header "content-type: application/json" \
     --data '{
         "model": "claude-sonnet-4-20250514",
         "max_tokens": 20000,
         "stream": true,
         "thinking": {
             "type": "enabled",
             "budget_tokens": 16000
         },
         "messages": [
             {"role": "user", "content": "What is 27 * 453?"}
         ]
     }'
```

**响应：**

```
event: message_start
data: {"type": "message_start", "message": {"id": "msg_01...", "type": "message", "role": "assistant", "content": [], "model": "claude-sonnet-4-20250514", "stop_reason": null, "stop_sequence": null}}

event: content_block_start
data: {"type": "content_block_start", "index": 0, "content_block": {"type": "thinking", "thinking": ""}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "Let me solve this step by step:\n\n1. First break down 27 * 453"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "\n2. 453 = 400 + 50 + 3"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "\n3. 27 * 400 = 10,800"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "\n4. 27 * 50 = 1,350"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "\n5. 27 * 3 = 81"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "\n6. 10,800 + 1,350 + 81 = 12,231"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "signature_delta", "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds..."}}

event: content_block_stop
data: {"type": "content_block_stop", "index": 0}

event: content_block_start
data: {"type": "content_block_start", "index": 1, "content_block": {"type": "text", "text": ""}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 1, "delta": {"type": "text_delta", "text": "27 * 453 = 12,231"}}

event: content_block_stop
data: {"type": "content_block_stop", "index": 1}

event: message_delta
data: {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": null}}

event: message_stop
data: {"type": "message_stop"}
```

> **注意：** 根据 Anthropic 的版本策略，未来可能新增未知事件类型，客户端代码应优雅处理未知事件。

---

## 10. 错误格式

### 错误响应结构

```json
{
  "type": "error",
  "error": {
    "type": "authentication_error",
    "message": "invalid x-api-key"
  }
}
```

每个 API 响应都包含一个唯一的 `request-id` 响应头（如 `req_018EeWyXxfu5pfWkrYcMdjWG`），用于问题追踪。

### 错误类型与 HTTP 状态码映射

| HTTP 状态码 | 错误类型 | 说明 |
|------------|---------|------|
| 400 | `invalid_request_error` | 请求格式或内容有问题。其他未列出的 4XX 错误也使用此类型 |
| 401 | `authentication_error` | API 密钥有问题 |
| 403 | `permission_error` | API 密钥没有权限访问指定资源 |
| 404 | `not_found_error` | 请求的资源不存在 |
| 413 | `request_too_large` | 请求超过最大字节数限制（Messages API 最大 32MB） |
| 429 | `rate_limit_error` | 触发速率限制 |
| 500 | `api_error` | Anthropic 内部系统出现意外错误 |
| 529 | `overloaded_error` | Anthropic API 临时过载 |

### 请求大小限制

| 端点类型 | 最大请求大小 |
|---------|------------|
| Messages API | 32 MB |
| Token Counting API | 32 MB |
| Batch API | 256 MB |
| Files API | 500 MB |

### 流式响应中的错误

在 SSE 流式响应中，错误可能在 200 响应已返回后发生，此时错误不遵循标准 HTTP 错误机制，而是以 `error` 事件发送：

```
event: error
data: {"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}
```

---

## 11. Prompt Caching (cache_control)

### 概述

Prompt Caching 允许从提示词的特定前缀处恢复处理，显著减少重复任务或具有固定内容的提示的处理时间和成本。

### 工作原理

1. 系统检查提示词前缀（到指定缓存断点）是否已被缓存。
2. 如找到匹配，使用缓存版本，减少处理时间和成本。
3. 如未找到，处理完整提示词并在响应开始时缓存前缀。

### cache_control 参数

| 字段 | 是否必填 | 类型 | 说明 |
|------|---------|------|------|
| `type` | **必填** | string | 固定为 `"ephemeral"` |
| `ttl` | 可选 | string | 缓存有效期：`"5m"`（5 分钟，默认）或 `"1h"`（1 小时） |

```json
{
  "cache_control": {
    "type": "ephemeral",
    "ttl": "5m"
  }
}
```

### 使用方式

`cache_control` 可以添加在以下位置：

| 位置 | 示例 |
|------|------|
| `system` 内容块 | `[{"type": "text", "text": "...", "cache_control": {"type": "ephemeral"}}]` |
| `messages.content` 内容块 | `[{"type": "text", "text": "...", "cache_control": {"type": "ephemeral"}}]` |
| `tools` 定义 | `[{"name": "...", "cache_control": {"type": "ephemeral"}, ...}]` |

**缓存前缀顺序：** `tools` → `system` → `messages`，形成层级结构。

### 缓存最小 token 数

| 模型 | 最小可缓存 token 数 |
|------|-------------------|
| Claude Opus 4 / Sonnet 4 / Sonnet 3.7 / Opus 3 | 1024 tokens |
| Claude Haiku 3.5 / Haiku 3 | 2048 tokens |

### 缓存断点数量

- 最多可定义 **4 个**缓存断点。
- 只需一个断点即可——系统会自动检查之前约 20 个内容块边界处的缓存命中。
- 超过 20 个内容块的提示词，需要额外的 `cache_control` 参数确保内容可缓存。

### 计费

| 类型 | 说明 |
|------|------|
| 常规输入 token | 未缓存、未命中部分 |
| 缓存写入 (Cache Write) | 新写入缓存的 token，5 分钟 TTL 为基础价格的 125%，1 小时 TTL 为 200% |
| 缓存读取 (Cache Hit / Refresh) | 命中缓存，为基础价格的 10% |

缓存断点本身不产生费用。

### 缓存命中监控

通过响应 `usage` 字段追踪缓存性能：

| 字段 | 说明 |
|------|------|
| `cache_creation_input_tokens` | 写入缓存的 token 总数 |
| `cache_read_input_tokens` | 从缓存读取的 token 总数 |
| `cache_creation.ephemeral_5m_input_tokens` | 5 分钟 TTL 缓存写入 token 数 |
| `cache_creation.ephemeral_1h_input_tokens` | 1 小时 TTL 缓存写入 token 数 |
| `input_tokens` | 未使用缓存的输入 token 数 |

### 缓存失效规则

缓存层级为 `tools` → `system` → `messages`，某层级变更会使该层级及之后所有层级的缓存失效：

| 变更内容 | Tools 缓存 | System 缓存 | Messages 缓存 |
|---------|-----------|------------|--------------|
| 工具定义变更 | 失效 | 失效 | 失效 |
| 网页搜索开关 | 有效 | 失效 | 失效 |
| 引用开关 | 有效 | 失效 | 失效 |
| tool_choice 变更 | 有效 | 有效 | 失效 |
| 图像变更 | 有效 | 有效 | 失效 |
| thinking 参数变更 | 有效 | 有效 | 失效 |

### 混合 TTL 使用

可以在同一请求中使用 5 分钟和 1 小时 TTL，但约束条件：**较长 TTL 的缓存断点必须出现在较短 TTL 之前**（即 1 小时断点必须在 5 分钟断点之前）。

### 不可缓存的内容

- thinking 内容块不能直接标记 `cache_control`（但会随其他内容一起被缓存）
- 子内容块（如引用 citations）不能直接缓存，需缓存顶层块
- 空文本块不能被缓存
