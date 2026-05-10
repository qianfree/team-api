# Anthropic Claude Messages API 完整文档

> 最后更新: 2026-04-19 | API 版本: 2023-06-01
> 来源: https://docs.anthropic.com/en/api/messages

---

## 目录

1. [端点概览](#1-端点概览)
2. [Messages API 请求参数](#2-messages-api-请求参数)
3. [消息内容类型](#3-消息内容类型)
4. [响应格式](#4-响应格式)
5. [响应内容块类型](#5-响应内容块类型)
6. [流式响应 (SSE)](#6-流式响应-sse)
7. [扩展思维 (Extended Thinking)](#7-扩展思维-extended-thinking)
8. [工具使用 (Tool Use)](#8-工具使用-tool-use)
9. [视觉/图像支持 (Vision)](#9-视觉图像支持-vision)
10. [提示缓存 (Prompt Caching)](#10-提示缓存-prompt-caching)
11. [错误格式](#11-错误格式)
12. [模型 ID 与定价](#12-模型-id-与定价)
13. [Token 计数端点](#13-token-计数端点)
14. [Batch API](#14-batch-api)
15. [MCP 服务器](#15-mcp-服务器)

---

## 1. 端点概览

### Messages API

| 属性 | 值 |
|------|-----|
| **端点** | `POST https://api.anthropic.com/v1/messages` |
| **Content-Type** | `application/json` |
| **认证** | `x-api-key` 请求头 |
| **API 版本** | `anthropic-version: 2023-06-01` |
| **最大请求体** | 32 MB |

### 必需请求头

| 请求头 | 类型 | 必需 | 说明 |
|--------|------|------|------|
| `anthropic-version` | string | **是** | API 版本号，当前为 `2023-06-01` |
| `x-api-key` | string | **是** | API 密钥，每个密钥绑定到一个 Workspace |
| `anthropic-beta` | string[] | 否 | Beta 功能标识，多个用逗号分隔，如 `beta1,beta2` |
| `content-type` | string | **是** | `application/json` |

---

## 2. Messages API 请求参数

### 请求体 (Request Body)

| 参数 | 类型 | 必需 | 默认值 | 说明 |
|------|------|------|--------|------|
| `model` | string | **是** | - | 模型 ID，长度 1-256 字符 |
| `messages` | object[] | **是** | - | 输入消息数组，最多 100,000 条 |
| `max_tokens` | integer | **是** | - | 最大生成 token 数，>= 1 |
| `system` | string \| object[] | 否 | - | 系统提示词 |
| `temperature` | number | 否 | `1.0` | 随机性控制，范围 0-1 |
| `top_p` | number | 否 | - | 核采样，范围 0-1 |
| `top_k` | integer | 否 | - | Top-K 采样，>= 0 |
| `stop_sequences` | string[] | 否 | - | 自定义停止序列 |
| `stream` | boolean | 否 | `false` | 是否使用 SSE 流式响应 |
| `tools` | object[] | 否 | - | 工具定义数组 |
| `tool_choice` | object | 否 | - | 工具选择策略 |
| `thinking` | object | 否 | - | 扩展思维配置 |
| `metadata` | object | 否 | - | 请求元数据 |
| `service_tier` | enum | 否 | - | 服务层级：`auto`、`standard_only` |
| `container` | string \| null | 否 | - | 容器标识符，用于跨请求复用 |
| `mcp_servers` | object[] | 否 | - | MCP 服务器配置 |

### 参数详细说明

#### `model` (string, 必需)

要使用的模型 ID。

```json
"model": "claude-sonnet-4-20250514"
```

#### `messages` (object[], 必需)

输入消息数组。模型训练在交替的 `user` 和 `assistant` 对话轮次上运行。连续的 `user` 或 `assistant` 消息会被合并为一个轮次。

**消息对象:**

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `role` | enum | **是** | `user` 或 `assistant` |
| `content` | string \| object[] | **是** | 消息内容，可以是字符串或内容块数组 |

**示例:**

```json
// 单条用户消息
[{"role": "user", "content": "Hello, Claude"}]

// 多轮对话
[
  {"role": "user", "content": "Hello there."},
  {"role": "assistant", "content": "Hi, I'm Claude. How can I help you?"},
  {"role": "user", "content": "Can you explain LLMs in plain English?"}
]

// 预填充助手响应
[
  {"role": "user", "content": "What's the Greek name for Sun? (A) Sol (B) Helios (C) Sun"},
  {"role": "assistant", "content": "The best answer is ("}
]

// 等价写法
{"role": "user", "content": "Hello, Claude"}
// 等价于
{"role": "user", "content": [{"type": "text", "text": "Hello, Claude"}]}
```

**注意:** 系统提示词使用顶层 `system` 参数，消息中没有 `system` 角色。

#### `max_tokens` (integer, 必需)

最大生成 token 数。模型可能在此之前停止。不同模型有不同的最大值。

```json
"max_tokens": 1024
```

#### `system` (string | object[], 可选)

系统提示词，为 Claude 提供上下文和指令。

```json
// 字符串格式
"system": "Today's date is 2023-01-01."

// 数组格式（支持缓存控制）
"system": [
  {
    "text": "Today's date is 2024-06-01.",
    "type": "text"
  }
]
```

#### `temperature` (number, 可选, 默认 1.0)

控制响应随机性。接近 0.0 适合分析/多选题，接近 1.0 适合创意/生成任务。即使设为 0.0 也不会完全确定性。

```json
"temperature": 1.0
```

#### `top_p` (number, 可选)

核采样参数。应只修改 `temperature` 或 `top_p` 其中一个，不要同时修改。

```json
"top_p": 0.7
```

#### `top_k` (integer, 可选)

只从概率最高的 K 个选项中采样，用于去除"长尾"低概率响应。仅推荐高级场景使用。

```json
"top_k": 5
```

#### `stop_sequences` (string[], 可选)

自定义停止序列。当模型遇到这些字符串时会停止生成。

```json
"stop_sequences": ["\n\nHuman:", "\n\nAssistant:"]
```

#### `stream` (boolean, 可选, 默认 false)

是否使用 Server-Sent Events (SSE) 增量流式传输响应。**当 `max_tokens` > 21,333 时必须启用流式传输。**

```json
"stream": true
```

#### `metadata` (object, 可选)

请求元数据。

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_id` | string \| null | 用户外部标识符（UUID/哈希），最长 256 字符。不应包含姓名、邮箱等身份信息 |

```json
"metadata": {
  "user_id": "13803d75-b4b5-4c3e-b2a2-6f21399b021b"
}
```

#### `service_tier` (enum, 可选)

服务层级选择。

| 值 | 说明 |
|-----|------|
| `auto` | 自动选择优先或标准容量 |
| `standard_only` | 仅使用标准容量 |

#### `container` (string | null, 可选)

容器标识符，用于跨请求复用。

---

## 3. 消息内容类型

### 输入消息内容块

消息的 `content` 字段可以是字符串或内容块数组。支持以下内容块类型:

#### 3.1 文本内容块

```json
{
  "type": "text",
  "text": "描述这张图片的内容。"
}
```

#### 3.2 图像内容块

支持三种图像源类型:

**Base64 编码图像:**

```json
{
  "type": "image",
  "source": {
    "type": "base64",
    "media_type": "image/jpeg",
    "data": "base64编码的图像数据..."
  }
}
```

**URL 图像:**

```json
{
  "type": "image",
  "source": {
    "type": "url",
    "url": "https://example.com/image.jpg"
  }
}
```

**Files API 图像 (需要 `anthropic-beta: files-api-2025-04-14`):**

```json
{
  "type": "image",
  "source": {
    "type": "file",
    "file_id": "file_abc123"
  }
}
```

#### 3.3 工具结果内容块

用于将工具执行结果返回给模型:

```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
  "content": "259.75 USD"
}
```

`content` 可以是字符串或内容块数组（用于返回图像等复杂结果）。

#### 3.4 思维内容块 (Thinking)

扩展思维启用时，需要将上一轮的 thinking 块原样传回:

```json
{
  "type": "thinking",
  "thinking": "Let me solve this step by step...",
  "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds..."
}
```

#### 3.5 脱敏思维内容块 (Redacted Thinking)

```json
{
  "type": "redacted_thinking",
  "thinking": "加密内容...",
  "signature": "签名数据..."
}
```

#### 3.6 带缓存控制的内容块

任何内容块都可以附加 `cache_control`:

```json
{
  "type": "text",
  "text": "大段静态文本内容...",
  "cache_control": {
    "type": "ephemeral",
    "ttl": "5m"
  }
}
```

---

## 4. 响应格式

### 非流式响应 (HTTP 200)

```json
{
  "id": "msg_013Zva2CMHLNnXjNJJKqJ2EF",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hi! My name is Claude."
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
  "container": null
}
```

### 响应字段详解

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `id` | string | **是** | 唯一对象标识符，如 `msg_013Zva2CMHLNnXjNJJKqJ2EF` |
| `type` | enum | **是** | 固定为 `"message"` |
| `role` | enum | **是** | 固定为 `"assistant"` |
| `content` | object[] | **是** | 内容块数组，见第 5 节 |
| `model` | string | **是** | 处理请求的模型 ID |
| `stop_reason` | enum \| null | **是** | 停止原因，见下表 |
| `stop_sequence` | string \| null | **是** | 匹配到的自定义停止序列 |
| `usage` | object | **是** | Token 使用量，见下文 |
| `container` | object \| null | **是** | 容器信息（使用代码执行等工具时非空） |

### stop_reason 枚举值

| 值 | 说明 |
|-----|------|
| `end_turn` | 模型自然停止 |
| `max_tokens` | 超过请求的 `max_tokens` 或模型最大值 |
| `stop_sequence` | 遇到自定义停止序列 |
| `tool_use` | 模型调用了一个或多个工具 |
| `pause_turn` | 暂停长运行的轮次，可在后续请求中原样传回继续 |
| `refusal` | 流式分类器干预处理潜在的策略违规 |

### usage 对象

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `input_tokens` | integer | - | 使用的输入 token 数 |
| `output_tokens` | integer | - | 使用的输出 token 数 |
| `cache_creation_input_tokens` | integer \| null | - | 创建缓存条目使用的 token 数 |
| `cache_read_input_tokens` | integer \| null | - | 从缓存读取的 token 数 |
| `cache_creation` | object \| null | - | 按 TTL 分组的缓存创建明细 |
| `cache_creation.ephemeral_5m_input_tokens` | integer | `0` | 5 分钟缓存写入的 token 数 |
| `cache_creation.ephemeral_1h_input_tokens` | integer | `0` | 1 小时缓存写入的 token 数 |
| `server_tool_use` | object \| null | - | 服务器端工具使用统计 |
| `server_tool_use.web_search_requests` | integer | `0` | Web 搜索工具请求次数 |
| `service_tier` | enum \| null | - | 服务层级：`standard`、`priority`、`batch` |

**总输入 token 计算:** `input_tokens` + `cache_creation_input_tokens` + `cache_read_input_tokens`

### container 对象

当使用容器工具（如代码执行）时非空。

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 容器标识符 |
| `expires_at` | string | 容器过期时间 |

---

## 5. 响应内容块类型

### 5.1 text - 文本内容块

```json
{
  "type": "text",
  "text": "Hi! My name is Claude."
}
```

### 5.2 thinking - 思维内容块

扩展思维启用时返回。包含模型的内部推理过程。

```json
{
  "type": "thinking",
  "thinking": "Let me solve this step by step:\n\n1. First break down 27 * 453...",
  "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds..."
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定 `"thinking"` |
| `thinking` | string | 思维内容文本 |
| `signature` | string | 签名，用于验证 thinking 块的完整性，传回 API 时必须原样保留 |

**Claude 4 模型返回的是思维摘要 (Summarized Thinking)，不是完整思维。**

### 5.3 redacted_thinking - 脱敏思维内容块

安全系统标记时返回加密的思维内容。

```json
{
  "type": "redacted_thinking",
  "thinking": "加密的不可读内容...",
  "signature": "签名数据..."
}
```

### 5.4 tool_use - 工具使用内容块

```json
{
  "type": "tool_use",
  "id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
  "name": "get_stock_price",
  "input": {
    "ticker": "^GSPC"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定 `"tool_use"` |
| `id` | string | 工具调用唯一标识，如 `toolu_01...` |
| `name` | string | 调用的工具名称 |
| `input` | object | 工具输入参数（JSON 对象） |

### 5.5 server_tool_use - 服务器端工具使用

```json
{
  "type": "server_tool_use",
  "id": "server_tool_id",
  "name": "web_search",
  "input": { ... }
}
```

### 5.6 web_search_tool_result - Web 搜索工具结果

```json
{
  "type": "web_search_tool_result",
  "tool_use_id": "server_tool_id",
  "content": [ ... ]
}
```

### 5.7 code_execution_tool_result - 代码执行工具结果

```json
{
  "type": "code_execution_tool_result",
  "tool_use_id": "...",
  "content": [ ... ]
}
```

### 5.8 mcp_tool_use - MCP 工具使用

```json
{
  "type": "mcp_tool_use",
  "id": "...",
  "name": "...",
  "input": { ... },
  "server_name": "..."
}
```

### 5.9 mcp_tool_result - MCP 工具结果

```json
{
  "type": "mcp_tool_result",
  "tool_use_id": "...",
  "content": [ ... ]
}
```

### 5.10 container_upload - 容器上传

```json
{
  "type": "container_upload",
  "file_id": "file_abc123"
}
```

---

## 6. 流式响应 (SSE)

当设置 `"stream": true` 时，响应使用 Server-Sent Events (SSE) 格式，`Content-Type: text/event-stream`。

### 6.1 事件流顺序

```
1. message_start          → 包含空 content 的 Message 对象
2. 内容块系列（每个）:
   a. content_block_start  → 内容块开始
   b. content_block_delta  → 一个或多个增量事件
   c. content_block_stop   → 内容块结束
3. message_delta           → 顶层 Message 对象变更
4. message_stop            → 消息结束
```

可能穿插 `ping` 事件。

### 6.2 事件类型

#### message_start

```json
event: message_start
data: {
  "type": "message_start",
  "message": {
    "id": "msg_1nZdL29xx5MUA1yADyHTEsnR8uuvGzszyY",
    "type": "message",
    "role": "assistant",
    "content": [],
    "model": "claude-3-7-sonnet-20250219",
    "stop_reason": null,
    "stop_sequence": null,
    "usage": {
      "input_tokens": 25,
      "output_tokens": 1
    }
  }
}
```

#### content_block_start

```json
event: content_block_start
data: {
  "type": "content_block_start",
  "index": 0,
  "content_block": {
    "type": "text",
    "text": ""
  }
}
```

#### content_block_delta

见下文 Delta 类型。

#### content_block_stop

```json
event: content_block_stop
data: {
  "type": "content_block_stop",
  "index": 0
}
```

#### message_delta

```json
event: message_delta
data: {
  "type": "message_delta",
  "delta": {
    "stop_reason": "end_turn",
    "stop_sequence": null
  },
  "usage": {
    "output_tokens": 15
  }
}
```

#### message_stop

```json
event: message_stop
data: {
  "type": "message_stop"
}
```

#### ping

```json
event: ping
data: {"type": "ping"}
```

#### error

```json
event: error
data: {
  "type": "error",
  "error": {
    "type": "overloaded_error",
    "message": "Overloaded"
  }
}
```

### 6.3 Delta 类型

#### text_delta - 文本增量

```json
event: content_block_delta
data: {
  "type": "content_block_delta",
  "index": 0,
  "delta": {
    "type": "text_delta",
    "text": "ello frien"
  }
}
```

#### input_json_delta - 工具输入 JSON 增量

增量是**部分 JSON 字符串**，需要累积后解析。最终 `tool_use.input` 总是一个 JSON 对象。

```json
event: content_block_delta
data: {
  "type": "content_block_delta",
  "index": 1,
  "delta": {
    "type": "input_json_delta",
    "partial_json": "{\"location\": \"San Fra"
  }
}
```

**注意:** 当前模型一次只支持输出一个完整的 key-value 属性，因此工具使用时可能有事件间隔。

#### thinking_delta - 思维增量

```json
event: content_block_delta
data: {
  "type": "content_block_delta",
  "index": 0,
  "delta": {
    "type": "thinking_delta",
    "thinking": "Let me solve this step by step:\n\n1. First break down 27 * 453"
  }
}
```

#### signature_delta - 签名增量

在 `content_block_stop` 之前发送，用于验证 thinking 块的完整性。

```json
event: content_block_delta
data: {
  "type": "content_block_delta",
  "index": 0,
  "delta": {
    "type": "signature_delta",
    "signature": "EqQBCgIYAhIM1gbcDa9GJwZA2b3hGgxBdjrkzLoky3dl1pkiMOYds..."
  }
}
```

### 6.4 完整流式示例

#### 基本流式请求

```
请求:
POST /v1/messages
{
  "model": "claude-3-7-sonnet-20250219",
  "messages": [{"role": "user", "content": "Hello"}],
  "max_tokens": 256,
  "stream": true
}

响应:
event: message_start
data: {"type": "message_start", "message": {"id": "msg_1nZ...", "type": "message", "role": "assistant", "content": [], "model": "claude-3-7-sonnet-20250219", "stop_reason": null, "stop_sequence": null, "usage": {"input_tokens": 25, "output_tokens": 1}}}

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

#### 工具使用流式请求

```
请求:
POST /v1/messages
{
  "model": "claude-3-7-sonnet-20250219",
  "max_tokens": 1024,
  "tools": [{
    "name": "get_weather",
    "description": "Get the current weather in a given location",
    "input_schema": {
      "type": "object",
      "properties": {
        "location": {"type": "string", "description": "The city and state, e.g. San Francisco, CA"}
      },
      "required": ["location"]
    }
  }],
  "tool_choice": {"type": "any"},
  "messages": [{"role": "user", "content": "What is the weather like in San Francisco?"}],
  "stream": true
}

响应:
event: message_start
data: {"type":"message_start","message":{"id":"msg_014p7gG3wDgGV9EUtLvnow3U","type":"message","role":"assistant","model":"claude-3-haiku-20240307","stop_sequence":null,"usage":{"input_tokens":472,"output_tokens":2},"content":[],"stop_reason":null}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Okay"}}
...

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

#### 扩展思维流式请求

```
请求:
POST /v1/messages
{
  "model": "claude-3-7-sonnet-20250219",
  "max_tokens": 20000,
  "stream": true,
  "thinking": {
    "type": "enabled",
    "budget_tokens": 16000
  },
  "messages": [{"role": "user", "content": "What is 27 * 453?"}]
}

响应:
event: message_start
data: {"type": "message_start", "message": {"id": "msg_01...", ...}}

event: content_block_start
data: {"type": "content_block_start", "index": 0, "content_block": {"type": "thinking", "thinking": ""}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "Let me solve this step by step:\n\n1. First break down 27 * 453"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "\n2. 453 = 400 + 50 + 3"}}

...

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

---

## 7. 扩展思维 (Extended Thinking)

### 支持的模型

- `claude-opus-4-1-20250805` (Claude Opus 4.1)
- `claude-opus-4-20250514` (Claude Opus 4)
- `claude-sonnet-4-20250514` (Claude Sonnet 4)
- `claude-3-7-sonnet-20250219` (Claude Sonnet 3.7)

### 启用方式

```json
{
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 16000,
  "thinking": {
    "type": "enabled",
    "budget_tokens": 10000
  },
  "messages": [{"role": "user", "content": "Solve this complex problem..."}]
}
```

### thinking 对象参数

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `type` | enum | **是** | 固定 `"enabled"` |
| `budget_tokens` | integer | **是** | 思维 token 预算，>= 1024 且 < `max_tokens` |

### 响应格式

启用扩展思维后，响应按顺序包含:
1. `thinking` 内容块 - 模型的内部推理
2. `text` 内容块 - 最终回答

```json
{
  "content": [
    {
      "type": "thinking",
      "thinking": "Let me analyze this step by step...",
      "signature": "EqQBCgIYAhIM..."
    },
    {
      "type": "text",
      "text": "The answer is 42."
    }
  ]
}
```

### Claude 4 模型的思维摘要 (Summarized Thinking)

Claude 4 模型（Opus 4.1、Opus 4、Sonnet 4）返回思维**摘要**，不是完整思维。

| 特性 | Claude Sonnet 3.7 | Claude 4 模型 |
|------|-------------------|---------------|
| 思维输出 | 返回完整思维 | 返回思维摘要 |
| 交错思维 | 不支持 | 支持（需 `interleaved-thinking-2025-05-14` beta header） |

**计费说明:**
- **输入 token:** 原始请求中的 token（不含之前轮次的思维 token）
- **输出 token（计费）:** Claude 内部生成的原始思维 token
- **输出 token（可见）:** 响应中的摘要思维 token
- **不收费:** 生成摘要的 token

### 思维加密 (Thinking Encryption)

- 完整思维内容在 `signature` 字段中以加密形式返回
- 用于验证 thinking 块由 Claude 生成
- 流式响应中，签名通过 `signature_delta` 事件在 `content_block_stop` 之前发送
- `signature` 是不透明字段，不应解释或解析
- Claude 4 的签名值比之前模型显著更长
- 签名值跨平台兼容（Anthropic API、Amazon Bedrock、Vertex AI）

### 思维脱敏 (Thinking Redaction)

当安全系统标记部分思维内容时:
- 被标记的部分变为 `redacted_thinking` 块
- 内容被加密，不可人工阅读
- 传回 API 时会被解密，Claude 可继续推理
- **必须**原样传回所有 thinking 和 redacted_thinking 块

### 交错思维 (Interleaved Thinking)

仅 Claude 4 模型支持。需要 beta header: `interleaved-thinking-2025-05-14`

允许 Claude 在工具调用之间进行思维:
- 分析工具调用结果后再决定下一步
- 在推理步骤之间链式调用多个工具
- 基于中间结果做出更细致的决策

**注意事项:**
- 启用交错思维时，`budget_tokens` 可以超过 `max_tokens`（代表一个助手轮次内所有思维块的总预算）
- 仅支持通过 Messages API 使用的工具
- 在非 Claude 4 模型上传递此 header 无效果

### 工具使用中的思维保留

**关键规则:** 工具使用时，必须将上一轮助手的 thinking 块**完整、原样**传回 API。

原因:
1. **推理连续性:** thinking 块包含导致工具请求的逐步推理
2. **上下文维护:** 工具结果在 API 结构中表现为用户消息，但属于连续推理流

**重要:** 连续的 thinking 块序列必须与模型原始输出完全匹配，不能重新排列或修改。

### 扩展思维的限制

- 不兼容 `temperature` 或 `top_k` 修改
- 不兼容强制工具使用 (`tool_choice: {"type": "any"}` 或 `tool_choice: {"type": "tool", "name": "..."}`)
- 启用思维时可设置 `top_p` 为 1 到 0.95 之间的值
- 启用思维时不能预填充响应
- `max_tokens` 严格执行，超出上下文窗口会返回验证错误

### 性能建议

| 建议 | 说明 |
|------|------|
| 预算优化 | 从 1,024 开始，逐步增加。预算 > 32k 时建议使用批处理 |
| 起始值 | 复杂任务从 16k+ 开始 |
| 大预算 | 超过 32k 可能导致系统超时和连接限制 |
| 流式要求 | `max_tokens` > 21,333 时**必须**启用流式 |

---

## 8. 工具使用 (Tool Use)

### 工具定义 (tools)

```json
{
  "tools": [
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
}
```

### tools 数组中每个对象的字段

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `name` | string | **是** | 工具名称，1-128 字符 |
| `input_schema` | object | **是** | 工具输入的 JSON Schema |
| `input_schema.type` | enum | **是** | 固定 `"object"` |
| `input_schema.properties` | object \| null | 否 | 属性定义 |
| `input_schema.required` | string[] \| null | 否 | 必需属性列表 |
| `type` | enum \| null | 否 | 工具类型，`"custom"` |
| `description` | string | 否 | 工具描述（强烈建议提供） |
| `cache_control` | object \| null | 否 | 缓存控制断点 |

### cache_control 对象

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `type` | enum | **是** | 固定 `"ephemeral"` |
| `ttl` | enum | 否 | `"5m"` (默认) 或 `"1h"` |

### tool_choice 对象

控制模型如何使用提供的工具。

#### auto (默认)

模型自行决定是否使用工具。

```json
"tool_choice": {"type": "auto"}
```

#### any

模型必须使用某个工具（但不指定哪个）。

```json
"tool_choice": {"type": "any"}
```

#### tool

指定使用特定工具。

```json
"tool_choice": {
  "type": "tool",
  "name": "get_weather"
}
```

#### none

禁止使用工具。

```json
"tool_choice": {"type": "none"}
```

#### disable_parallel_tool_use

适用于 `auto`、`any`、`tool` 类型。设为 `true` 时，模型最多输出一个工具使用。

```json
"tool_choice": {
  "type": "auto",
  "disable_parallel_tool_use": true
}
```

### 工具类型列表

| 工具类型 | Beta Header | 说明 |
|----------|-------------|------|
| `custom` | - | 自定义客户端工具 |
| Bash tool (2024-10-22) | `computer-use-2024-10-22` | Bash 工具 |
| Bash tool (2025-01-24) | `computer-use-2025-01-24` | Bash 工具（更新版） |
| Code execution (2025-05-22) | `code-execution-2025-05-22` | 代码执行工具 |
| Computer use (2024-01-22) | `computer-use-2024-01-22` | 计算机使用工具 |
| Computer use (2025-01-24) | `computer-use-2025-01-24` | 计算机使用工具（更新版） |
| Text editor (2024-10-22) | `computer-use-2024-10-22` | 文本编辑器工具 |
| Text editor (2025-01-24) | `computer-use-2025-01-24` | 文本编辑器工具（更新版） |
| Text editor (2025-04-29) | `computer-use-2025-04-29` | 文本编辑器工具（最新版） |
| TextEditor_20250728 | - | 文本编辑器工具 |
| Web search (2025-03-05) | `web-search-2025-03-05` | Web 搜索工具 |

### 工具使用完整流程

```
1. 用户发送请求（带 tools 定义）
2. 模型返回 tool_use 内容块（stop_reason = "tool_use"）
3. 客户端执行工具
4. 客户端将 tool_result 作为 user 消息发回
5. 模型继续生成（可能再次调用工具或给出最终回答）
```

**tool_result 内容块:**

```json
{
  "type": "tool_result",
  "tool_use_id": "toolu_01D7FLrfh4GYq7yT1ULFeyMV",
  "content": "259.75 USD"
}
```

`content` 可以是字符串或内容块数组，支持返回图像等复杂结果。

---

## 9. 视觉/图像支持 (Vision)

### 支持的图像格式

| Media Type | 格式 |
|------------|------|
| `image/jpeg` | JPEG |
| `image/png` | PNG |
| `image/gif` | GIF |
| `image/webp` | WebP |

### 图像限制

| 限制 | 值 |
|------|-----|
| 单张最大尺寸 | 8000 x 8000 px |
| 单张最大文件大小 | 5 MB (API), 10 MB (claude.ai) |
| 每次请求最大图片数 | 100 张 (API), 20 张 (claude.ai) |
| 超过 20 张时尺寸限制 | 2000 x 2000 px |

### 图像源类型

#### Base64 编码

```json
{
  "type": "image",
  "source": {
    "type": "base64",
    "media_type": "image/jpeg",
    "data": "base64编码数据..."
  }
}
```

#### URL 引用

```json
{
  "type": "image",
  "source": {
    "type": "url",
    "url": "https://example.com/image.jpg"
  }
}
```

#### Files API (需要 `anthropic-beta: files-api-2025-04-14`)

```json
{
  "type": "image",
  "source": {
    "type": "file",
    "file_id": "file_abc123"
  }
}
```

### Token 估算

**公式:** `tokens = (width * height) / 750`

**推荐尺寸:** 长边不超过 1568 像素（约 1.15 兆像素），超过会自动缩放。

| 图像尺寸 | 像素 | 估算 Token | 每张成本 (Sonnet) |
|----------|------|------------|-------------------|
| 200x200 | 0.04 MP | ~54 | ~$0.00016 |
| 1000x1000 | 1 MP | ~1,334 | ~$0.004 |
| 1092x1092 | 1.19 MP | ~1,590 | ~$0.0048 |

### 不超过缩放的最大尺寸

| 宽高比 | 图像尺寸 |
|--------|----------|
| 1:1 | 1092x1092 px |
| 3:4 | 951x1268 px |
| 2:3 | 896x1344 px |
| 9:16 | 819x1456 px |
| 1:2 | 784x1568 px |

### 最佳实践

- 图像放在问题/指令之前效果最佳（image-then-text 结构）
- 多张图像时用 "Image 1:"、"Image 2:" 标注
- 不读取图像元数据
- 不用于识别人物
- 对计数、空间推理等任务有一定局限性

---

## 10. 提示缓存 (Prompt Caching)

### 工作原理

1. 系统检查是否存在已缓存的提示前缀
2. 如果命中，使用缓存版本，减少处理时间和成本
3. 如果未命中，处理完整提示并缓存前缀

缓存默认 5 分钟生命周期，每次使用时免费刷新。

### 支持的模型

所有当前模型均支持，包括 Claude Opus 4.1、Opus 4、Sonnet 4、Sonnet 3.7、Haiku 3.5、Haiku 3 等。

### 实现

在任何内容块上添加 `cache_control`:

```json
{
  "type": "text",
  "text": "大段静态文本...",
  "cache_control": {
    "type": "ephemeral",
    "ttl": "5m"
  }
}
```

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `type` | enum | **是** | 固定 `"ephemeral"` |
| `ttl` | enum | 否 | `"5m"` (默认，5分钟) 或 `"1h"` (1小时) |

### 缓存层级

前缀按以下顺序创建: `tools` -> `system` -> `messages`

### 缓存限制

| 模型 | 最小可缓存 token 数 |
|------|---------------------|
| Claude Opus 4, Sonnet 4, Sonnet 3.7, Opus 3 | 1,024 |
| Claude Haiku 3.5, Haiku 3 | 2,048 |

- 最多 4 个缓存断点
- 并发请求中，缓存条目在首次响应开始后才可用

### 可缓存内容

| 内容类型 | 可缓存 |
|----------|--------|
| Tools 工具定义 | 是 |
| System 系统消息 | 是 |
| Text 文本消息 | 是 |
| Images 图像 | 是 |
| Documents 文档 | 是 |
| Tool use / tool results | 是 |
| Thinking 块 | 间接缓存（随其他内容一起） |
| Citations 子内容块 | 间接缓存（缓存顶级块） |
| 空文本块 | 否 |

### 缓存失效规则

| 变更内容 | Tools 缓存 | System 缓存 | Messages 缓存 |
|----------|-----------|-------------|---------------|
| 工具定义变更 | 失效 | 失效 | 失效 |
| Web 搜索开关 | 有效 | 失效 | 失效 |
| Citations 开关 | 有效 | 失效 | 失效 |
| tool_choice 变更 | 有效 | 有效 | 失效 |
| 图像变更 | 有效 | 有效 | 失效 |
| Thinking 参数变更 | 有效 | 有效 | 失效 |
| 非工具结果传入 thinking 请求 | 有效 | 有效 | 失效 |

### 缓存性能追踪

响应 `usage` 中的关键字段:

| 字段 | 说明 |
|------|------|
| `cache_creation_input_tokens` | 新建缓存条目写入的 token 数 |
| `cache_read_input_tokens` | 从缓存读取的 token 数 |
| `input_tokens` | 未缓存也未创建缓存的 token 数 |

### 混合 TTL 计费

使用 1h 和 5m 混合 TTL 时:
- 1h 缓存必须出现在 5m 缓存之前
- 确定三个计费位置:
  - **位置 A:** 最高缓存命中的 token 数
  - **位置 B:** A 之后最高 1h cache_control 的 token 数
  - **位置 C:** 最后一个 cache_control 的 token 数

**费用计算:**
1. 缓存读取 token = A
2. 1h 缓存写入 token = (B - A)
3. 5m 缓存写入 token = (C - B)

### 缓存隔离

- **组织隔离:** 不同组织的缓存互不共享
- **精确匹配:** 缓存命中要求 100% 完全相同的提示段（包括所有文本和图像）
- **输出不受影响:** 缓存不影响输出 token 生成

### 定价

| 模型 | 基础输入 | 5m 缓存写入 | 1h 缓存写入 | 缓存命中/刷新 | 输出 |
|------|----------|-------------|-------------|---------------|------|
| Claude Opus 4.1 | $15/MTok | $18.75/MTok | $30/MTok | $1.50/MTok | $75/MTok |
| Claude Opus 4 | $15/MTok | $18.75/MTok | $30/MTok | $1.50/MTok | $75/MTok |
| Claude Sonnet 4 | $3/MTok | $3.75/MTok | $6/MTok | $0.30/MTok | $15/MTok |
| Claude Sonnet 3.7 | $3/MTok | $3.75/MTok | $6/MTok | $0.30/MTok | $15/MTok |
| Claude Haiku 3.5 | $0.80/MTok | $1/MTok | $1.6/MTok | $0.08/MTok | $4/MTok |
| Claude Haiku 3 | $0.25/MTok | $0.30/MTok | $0.50/MTok | $0.03/MTok | $1.25/MTok |

---

## 11. 错误格式

### 错误响应结构

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "具体错误描述"
  }
}
```

### HTTP 状态码与错误类型

| HTTP 状态码 | error.type | 说明 |
|-------------|-----------|------|
| 400 | `invalid_request_error` | 请求格式错误、参数无效 |
| 401 | `authentication_error` | API Key 无效 |
| 403 | `permission_error` | 无权限访问 |
| 404 | `not_found_error` | 资源不存在 |
| 413 | `request_too_large` | 请求体过大 |
| 429 | `rate_limit_error` | 请求频率超限 |
| 500 | `api_error` | API 内部错误 |
| 529 | `overloaded_error` | API 过载 |

### 请求大小限制

| API | 最大请求体大小 |
|-----|---------------|
| Messages API | 32 MB |
| Token Counting API | 32 MB |
| Batch API | 256 MB |
| Files API | 500 MB |

### 流式错误

在 SSE 流中可能发送错误事件:

```
event: error
data: {"type": "error", "error": {"type": "overloaded_error", "message": "Overloaded"}}
```

---

## 12. 模型 ID 与定价

### 当前模型

| 模型 ID | 名称 | 上下文窗口 | 最大输出 | 输入价格 | 输出价格 |
|---------|------|-----------|----------|----------|----------|
| `claude-opus-4-1-20250805` | Claude Opus 4.1 | 200K | - | $15/MTok | $75/MTok |
| `claude-opus-4-20250514` | Claude Opus 4 | 200K | - | $15/MTok | $75/MTok |
| `claude-sonnet-4-20250514` | Claude Sonnet 4 | 200K | - | $3/MTok | $15/MTok |
| `claude-3-7-sonnet-20250219` | Claude Sonnet 3.7 | 200K | - | $3/MTok | $15/MTok |

### 已弃用模型

| 模型 ID | 状态 |
|---------|------|
| `claude-3-5-sonnet-20241022` | 已弃用 |
| `claude-3-opus-20240229` | 已弃用 |

### 模型选择字符串长度

- `model` 字段要求 1-256 字符

### 扩展思维支持

| 模型 | 扩展思维 | 摘要思维 | 交错思维 |
|------|----------|----------|----------|
| Claude Opus 4.1 | 是 | 是 | 是 |
| Claude Opus 4 | 是 | 是 | 是 |
| Claude Sonnet 4 | 是 | 是 | 是 |
| Claude Sonnet 3.7 | 是 | 否 | 否 |

---

## 13. Token 计数端点

### 端点

| 属性 | 值 |
|------|-----|
| **端点** | `POST https://api.anthropic.com/v1/messages/count_tokens` |
| **Content-Type** | `application/json` |
| **认证** | `x-api-key` 请求头 |
| **API 版本** | `anthropic-version: 2023-06-01` |

### 请求参数

与 Messages API 共享部分参数:

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `model` | string | **是** | 模型 ID，1-256 字符 |
| `messages` | object[] | **是** | 输入消息数组（格式同 Messages API） |
| `system` | string \| object[] | 否 | 系统提示词 |
| `tools` | object[] | 否 | 工具定义 |
| `tool_choice` | object | 否 | 工具选择策略 |
| `thinking` | object | 否 | 扩展思维配置 |
| `mcp_servers` | object[] | 否 | MCP 服务器配置 |

**注意:** 不需要 `max_tokens` 参数。

### 请求示例

```bash
curl https://api.anthropic.com/v1/messages/count_tokens \
     --header "x-api-key: $ANTHROPIC_API_KEY" \
     --header "anthropic-version: 2023-06-01" \
     --header "content-type: application/json" \
     --data '{
         "model": "claude-3-7-sonnet-20250219",
         "messages": [
             {"role": "user", "content": "Hello, world"}
         ]
     }'
```

### 响应

```json
{
  "input_tokens": 2095
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `input_tokens` | integer | 所有提供的消息、系统提示词和工具的 token 总数 |

---

## 14. Batch API

### 端点

| 属性 | 值 |
|------|-----|
| **端点** | `POST https://api.anthropic.com/v1/messages/batches` |
| **最大请求体** | 256 MB |

### 概述

Batch API 允许异步处理大量消息请求。创建批处理后，系统异步执行，完成后可获取结果。

### 主要端点

| 操作 | 方法 | 端点 |
|------|------|------|
| 创建批处理 | POST | `/v1/messages/batches` |
| 获取批处理状态 | GET | `/v1/messages/batches/{batch_id}` |
| 列出批处理 | GET | `/v1/messages/batches` |
| 取消批处理 | POST | `/v1/messages/batches/{batch_id}/cancel` |
| 获取批处理结果 | GET | `/v1/messages/batches/{batch_id}/results` |

### 批处理请求格式

每个批处理请求包含多个子请求，每个子请求是一个完整的 Messages API 请求:

```json
{
  "requests": [
    {
      "custom_id": "request-1",
      "params": {
        "model": "claude-sonnet-4-20250514",
        "max_tokens": 1024,
        "messages": [{"role": "user", "content": "Hello"}]
      }
    },
    {
      "custom_id": "request-2",
      "params": {
        "model": "claude-sonnet-4-20250514",
        "max_tokens": 1024,
        "messages": [{"role": "user", "content": "World"}]
      }
    }
  ]
}
```

### 批处理定价

批处理请求使用标准 token 定价的 50%（`service_tier` 返回 `"batch"`）。

### 状态值

| 状态 | 说明 |
|------|------|
| `in_progress` | 正在处理 |
| `completed` | 已完成 |
| `failed` | 失败 |
| `expired` | 已过期 |
| `cancelled` | 已取消 |

> **注意:** Batch API 页面为 JavaScript 渲染，上述信息基于 API 参考文档整合。详细字段请参阅 Anthropic 官方文档。

---

## 15. MCP 服务器

### 概述

Messages API 支持通过 MCP（Model Context Protocol）连接远程 MCP 服务器，使 Claude 能够使用这些服务器提供的工具。

### mcp_servers 参数

```json
{
  "mcp_servers": [
    {
      "name": "my-mcp-server",
      "type": "url",
      "url": "https://mcp.example.com/sse",
      "authorization_token": "Bearer token...",
      "tool_configuration": {
        "allowed_tools": ["tool1", "tool2"],
        "enabled": true
      }
    }
  ]
}
```

### mcp_servers 对象字段

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `name` | string | **是** | MCP 服务器名称 |
| `type` | enum | **是** | 固定 `"url"` |
| `url` | string | **是** | MCP 服务器 URL |
| `authorization_token` | string \| null | 否 | 授权 token |
| `tool_configuration` | object \| null | 否 | 工具配置 |

### tool_configuration 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `allowed_tools` | string[] \| null | 允许使用的工具列表，null 表示允许所有 |
| `enabled` | boolean \| null | 是否启用，null 表示启用 |

### MCP 相关的响应内容块

- `mcp_tool_use` - MCP 工具调用
- `mcp_tool_result` - MCP 工具结果

---

## 附录 A: 完整请求示例

### 基本文本请求

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

### 带系统提示词和工具的请求

```bash
curl https://api.anthropic.com/v1/messages \
     --header "x-api-key: $ANTHROPIC_API_KEY" \
     --header "anthropic-version: 2023-06-01" \
     --header "content-type: application/json" \
     --data '{
         "model": "claude-sonnet-4-20250514",
         "max_tokens": 1024,
         "system": "You are a helpful weather assistant.",
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
             {"role": "user", "content": "What is the weather in SF?"}
         ]
     }'
```

### 带扩展思维和图像的请求

```bash
curl https://api.anthropic.com/v1/messages \
     --header "x-api-key: $ANTHROPIC_API_KEY" \
     --header "anthropic-version: 2023-06-01" \
     --header "content-type: application/json" \
     --data '{
         "model": "claude-sonnet-4-20250514",
         "max_tokens": 16000,
         "thinking": {
             "type": "enabled",
             "budget_tokens": 10000
         },
         "messages": [
             {
                 "role": "user",
                 "content": [
                     {
                         "type": "image",
                         "source": {
                             "type": "url",
                             "url": "https://example.com/chart.png"
                         }
                     },
                     {
                         "type": "text",
                         "text": "Analyze this chart in detail."
                     }
                 ]
             }
         ]
     }'
```

### 带提示缓存的请求

```bash
curl https://api.anthropic.com/v1/messages \
     --header "x-api-key: $ANTHROPIC_API_KEY" \
     --header "anthropic-version: 2023-06-01" \
     --header "content-type: application/json" \
     --data '{
         "model": "claude-sonnet-4-20250514",
         "max_tokens": 1024,
         "system": [
             {
                 "type": "text",
                 "text": "你是一个专业的客服助手，以下是公司的产品手册全文...(大段静态文本)",
                 "cache_control": {
                     "type": "ephemeral",
                     "ttl": "1h"
                 }
             }
         ],
         "messages": [
             {"role": "user", "content": "请介绍一下你们的产品"}
         ]
     }'
```

---

## 附录 B: 关键限制汇总

| 限制项 | 值 |
|--------|-----|
| 消息数组最大长度 | 100,000 条 |
| 单次请求最大图像数 | 100 张 |
| 单张图像最大尺寸 | 8000 x 8000 px |
| 单张图像最大文件大小 | 5 MB |
| model 字段长度 | 1-256 字符 |
| 工具名称长度 | 1-128 字符 |
| user_id 长度 | 最大 256 字符 |
| 扩展思维最小 budget | 1,024 tokens |
| 缓存断点最大数 | 4 个 |
| 流式必需条件 | max_tokens > 21,333 |
| Messages API 最大请求体 | 32 MB |
| Batch API 最大请求体 | 256 MB |
| Files API 最大文件大小 | 500 MB |
