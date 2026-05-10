# OpenAI Responses API 完整技术文档

> 文档来源：OpenAI 官方 API Reference (https://platform.openai.com/docs/api-reference/responses)
> 整理日期：2026-04-19

---

## 目录

1. [端点概览](#1-端点概览)
2. [请求参数详解](#2-请求参数详解)
3. [响应结构详解](#3-响应结构详解)
4. [输出项类型 (Output Item Types)](#4-输出项类型-output-item-types)
5. [SSE 流式事件类型](#5-sse-流式事件类型)
6. [输入格式 (Input Formats)](#6-输入格式-input-formats)
7. [工具类型 (Tool Types)](#7-工具类型-tool-types)
8. [推理参数 (Reasoning)](#8-推理参数-reasoning)
9. [错误格式](#9-错误格式)
10. [与 Chat Completions API 的差异](#10-与-chat-completions-api-的差异)

---

## 1. 端点概览

### 主端点

```
POST https://api.openai.com/v1/responses
```

创建一个模型 Response，支持文本/图像/文件输入，支持函数调用、网络搜索、文件搜索、代码解释器等工具。模型选择与 API key 关联的项目绑定。

**认证方式**：Bearer Token（API Key）

```
Authorization: Bearer $OPENAI_API_KEY
```

### 子端点

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/v1/responses` | 创建 Response |
| GET | `/v1/responses/{response_id}` | 检索已有 Response |
| DELETE | `/v1/responses/{response_id}` | 删除 Response |
| POST | `/v1/responses/{response_id}/cancel` | 取消进行中的 Response |
| POST | `/v1/responses/compact` | 压缩对话上下文 |
| GET | `/v1/responses/{response_id}/input_items` | 获取输入项列表 |
| POST | `/v1/responses/input_tokens` | 预估输入 token 数 |

---

## 2. 请求参数详解

### 顶层参数

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `model` | string | **是** | — | 模型 ID，如 `gpt-4o`、`o3`、`gpt-5.1` |
| `input` | string \| array | **是** | — | 用户输入。可以是纯文本字符串，也可以是内容项数组。详见第6节 |
| `instructions` | string | 否 | `""` | 系统级指令，相当于 Chat Completions 的 system message |
| `temperature` | float64 | 否 | `1.0` | 采样温度，0~2。越高越随机 |
| `top_p` | float64 | 否 | `1.0` | 核采样概率阈值 |
| `max_output_tokens` | uint | 否 | 模型默认 | 最大输出 token 数 |
| `tools` | array | 否 | `[]` | 可用工具列表。详见第7节 |
| `tool_choice` | string \| object | 否 | `"auto"` | 工具选择策略。详见下方 |
| `text` | object | 否 | `{format: {type: "text"}}` | 文本输出格式配置。详见下方 |
| `stream` | bool | 否 | `false` | 是否启用流式响应 |
| `stream_options` | object | 否 | — | 流式选项。详见下方 |
| `store` | bool | 否 | `true` | 是否存储响应以支持后续引用 |
| `metadata` | object | 否 | `{}` | 自定义元数据，键值对，最多 16 个键 |
| `reasoning` | object | 否 | — | 推理配置。详见第8节 |
| `truncation` | string | 否 | `"disabled"` | 上下文截断策略：`"auto"` 或 `"disabled"` |
| `previous_response_id` | string | 否 | — | 前一个 Response 的 ID，用于对话延续 |
| `conversation` | object | 否 | — | 对话状态管理对象，含 `id` 字段 |
| `include` | array | 否 | `[]` | 额外包含的数据，如 `["message.input_image.image_url"]` |
| `parallel_tool_calls` | bool | 否 | `true` | 是否允许并行工具调用 |
| `max_tool_calls` | uint | 否 | — | 最大工具调用次数 |
| `prompt` | object | 否 | — | 提示词配置（用于 prompt cache） |
| `prompt_cache_key` | string | 否 | — | 提示词缓存键 |
| `prompt_cache_retention` | string | 否 | — | 缓存保留时间 |
| `safety_identifier` | string | 否 | — | 安全标识符 |
| `service_tier` | string | 否 | `"auto"` | 服务层级：`"auto"`、`"default"`、`"flex"`、`"priority"` |
| `verbosity` | string | 否 | `"medium"` | 回复详细程度：`"low"`、`"medium"`、`"high"` |
| `logprobs` | bool | 否 | `false` | 是否返回 log probabilities |
| `top_logprobs` | int | 否 | `0` | 返回的 top log probabilities 数量（0~20） |
| `context_management` | object | 否 | — | 上下文管理配置。详见下方 |
| `user` | string | 否 | — | 终端用户标识符，用于监控和滥用检测 |
| `background` | bool | 否 | `false` | 是否以后台模式运行（异步） |

### tool_choice 可选值

| 值 | 类型 | 说明 |
|----|------|------|
| `"none"` | string | 不调用任何工具 |
| `"auto"` | string | 模型自动决定是否调用工具 |
| `"required"` | string | 必须调用至少一个工具 |
| `{"type": "function", "name": "func_name"}` | object | 指定调用某个函数工具 |
| `{"type": "mcp", "server_label": "...", "name": "..."}` | object | 指定调用某个 MCP 工具 |
| `{"type": "custom", "name": "..."}` | object | 指定调用某个自定义工具 |
| `{"type": "allowed_tools", "tools": [...]}` | object | 限制允许使用的工具列表 |
| `"apply_patch"` | string | 强制使用 apply_patch 工具 |
| `"shell"` | string | 强制使用 shell 工具 |

### text.format 对象

| 值类型 | 格式 | 说明 |
|--------|------|------|
| 纯文本 | `{"type": "text"}` | 默认，无格式约束 |
| JSON Schema | `{"type": "json_schema", "name": "...", "schema": {...}, "strict": true}` | 结构化输出，JSON Schema 约束 |
| JSON Object | `{"type": "json_object"}` | 旧版 JSON 输出（已过时，建议用 json_schema） |

**json_schema 格式详细字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"json_schema"` |
| `name` | string | 是 | Schema 名称 |
| `schema` | object | 是 | JSON Schema 定义 |
| `strict` | bool | 否 | 是否严格模式（默认 false） |
| `description` | string | 否 | Schema 描述 |

### stream_options 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `include_obfuscation` | bool | 是否在流中包含混淆信息 |

### context_management 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `compaction` | object | 上下文压缩配置 |
| `compaction.compact_threshold` | int | 触发压缩的 token 阈值 |

### service_tier 说明

| 值 | 说明 |
|----|------|
| `"auto"` | 自动选择最佳层级（默认） |
| `"default"` | 标准层级 |
| `"flex"` | 弹性层级，延迟可能更高但成本更低 |
| `"priority"` | 优先层级，最低延迟 |

### 请求示例

**基础文本请求**：

```json
{
  "model": "gpt-4o",
  "input": "Tell me a joke about programming."
}
```

**带工具和系统指令的请求**：

```json
{
  "model": "gpt-4o",
  "instructions": "You are a helpful assistant that provides concise answers.",
  "input": "What is the weather in San Francisco?",
  "tools": [
    {
      "type": "function",
      "name": "get_weather",
      "description": "Get the current weather in a location",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {"type": "string", "description": "City name"}
        },
        "required": ["location"]
      }
    }
  ],
  "tool_choice": "auto",
  "temperature": 0.7,
  "max_output_tokens": 1024,
  "store": true
}
```

**带结构化输出的请求**：

```json
{
  "model": "gpt-4o",
  "input": "List the top 3 programming languages.",
  "text": {
    "format": {
      "type": "json_schema",
      "name": "language_list",
      "strict": true,
      "schema": {
        "type": "object",
        "properties": {
          "languages": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "name": {"type": "string"},
                "rank": {"type": "integer"}
              },
              "required": ["name", "rank"]
            }
          }
        },
        "required": ["languages"]
      }
    }
  }
}
```

**流式请求**：

```json
{
  "model": "gpt-4o",
  "input": "Explain quantum computing in simple terms.",
  "stream": true
}
```

**带对话历史的请求（使用 previous_response_id）**：

```json
{
  "model": "gpt-4o",
  "input": "What about Tokyo?",
  "previous_response_id": "resp_abc123"
}
```

**带推理参数的请求**：

```json
{
  "model": "o3",
  "input": "Solve this math problem step by step: ...",
  "reasoning": {
    "effort": "high",
    "summary": "detailed"
  }
}
```

---

## 3. 响应结构详解

### 非流式响应对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | Response 唯一标识符，格式 `resp_xxxxx` |
| `object` | string | 固定值 `"response"` |
| `created_at` | int | 创建时间（Unix 时间戳，秒） |
| `completed_at` | int \| null | 完成时间（Unix 时间戳，秒） |
| `status` | string | 状态值。见下方状态枚举 |
| `error` | object \| null | 错误信息（如果有） |
| `incomplete_details` | object \| null | 未完成详情 |
| `instructions` | string \| null | 使用的系统指令 |
| `max_output_tokens` | int \| null | 最大输出 token 限制 |
| `model` | string | 实际使用的模型 ID |
| `output` | array | 输出项数组。详见第4节 |
| `parallel_tool_calls` | bool | 是否允许并行工具调用 |
| `previous_response_id` | string \| null | 前一个 Response 的 ID |
| `reasoning` | object \| null | 推理配置。详见第8节 |
| `store` | bool | 是否存储 |
| `temperature` | float64 \| null | 使用的温度值 |
| `text` | object \| null | 文本格式配置 |
| `tool_choice` | any | 工具选择策略 |
| `tools` | array | 可用工具列表 |
| `top_p` | float64 \| null | 使用的 top_p 值 |
| `truncation` | string \| null | 截断策略 |
| `usage` | object \| null | Token 使用量。见下方 |
| `user` | string \| null | 终端用户标识 |
| `metadata` | object \| null | 自定义元数据 |

### status 枚举

| 值 | 说明 |
|----|------|
| `"completed"` | 响应已完成 |
| `"failed"` | 响应失败 |
| `"in_progress"` | 响应正在进行 |
| `"cancelled"` | 响应已取消 |
| `"queued"` | 响应排队中（后台模式） |
| `"incomplete"` | 响应未完成（token 耗尽等） |

### incomplete_details 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `reason` | string | 未完成原因，如 `"max_output_tokens"` |

### usage 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `input_tokens` | int | 输入 token 数 |
| `output_tokens` | int | 输出 token 数 |
| `total_tokens` | int | 总 token 数 |
| `input_tokens_details` | object | 输入 token 细分 |
| `output_tokens_details` | object | 输出 token 细分 |

### input_tokens_details 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `cached_tokens` | int | 缓存命中的 token 数 |
| `text_tokens` | int | 文本 token 数 |
| `audio_tokens` | int | 音频 token 数 |
| `image_tokens` | int | 图像 token 数 |

### output_tokens_details 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `text_tokens` | int | 文本 token 数 |
| `audio_tokens` | int | 音频 token 数 |
| `reasoning_tokens` | int | 推理 token 数 |

### 响应示例

```json
{
  "id": "resp_abc123",
  "object": "response",
  "created_at": 1745000000,
  "completed_at": 1745000001,
  "status": "completed",
  "error": null,
  "incomplete_details": null,
  "instructions": "You are a helpful assistant.",
  "max_output_tokens": null,
  "model": "gpt-4o-2024-08-06",
  "output": [
    {
      "type": "message",
      "id": "msg_abc123",
      "status": "completed",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "Why do programmers prefer dark mode? Because light attracts bugs!",
          "annotations": []
        }
      ]
    }
  ],
  "parallel_tool_calls": true,
  "previous_response_id": null,
  "reasoning": null,
  "store": true,
  "temperature": 1.0,
  "text": {"format": {"type": "text"}},
  "tool_choice": "auto",
  "tools": [],
  "top_p": 1.0,
  "truncation": "disabled",
  "usage": {
    "input_tokens": 25,
    "output_tokens": 15,
    "total_tokens": 40,
    "input_tokens_details": {
      "cached_tokens": 0,
      "text_tokens": 25
    },
    "output_tokens_details": {
      "text_tokens": 15,
      "reasoning_tokens": 0
    }
  },
  "user": null,
  "metadata": {}
}
```

---

## 4. 输出项类型 (Output Item Types)

`output` 数组中的每个项都有一个 `type` 字段标识其类型。

### 4.1 message（文本消息）

模型生成的文本消息。

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"message"` |
| `id` | string | 消息 ID，格式 `msg_xxxxx` |
| `status` | string | `"completed"` 或 `"in_progress"` |
| `role` | string | `"assistant"` |
| `content` | array | 内容块数组，每项为 `output_text` 或 `refusal` |

**content 中的 output_text 项**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"output_text"` |
| `text` | string | 文本内容 |
| `annotations` | array | 注解列表（如 URL 引用、文件引用） |

**annotations 类型**：

| type | 字段 | 说明 |
|------|------|------|
| `url_citation` | `url`, `title`, `start_index`, `end_index` | URL 引用（来自 web_search） |
| `file_citation` | `file_id`, `filename`, `start_index`, `end_index` | 文件引用（来自 file_search） |
| `container_file_citation` | `file_id`, `start_index`, `end_index` | 容器文件引用（来自 code_interpreter） |

### 4.2 function_call（函数调用）

模型决定调用一个函数。

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"function_call"` |
| `id` | string | 调用项 ID |
| `call_id` | string | 函数调用 ID（用于匹配 function_call_output） |
| `name` | string | 函数名称 |
| `arguments` | string | 函数参数（JSON 字符串） |
| `status` | string | `"completed"` 或 `"in_progress"` |

### 4.3 function_call_output（函数调用结果）

函数调用的返回结果（通常出现在后续请求的 input 中）。

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"function_call_output"` |
| `call_id` | string | 对应的 function_call 的 call_id |
| `output` | string | 函数返回的结果 |

### 4.4 web_search_call（网络搜索调用）

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"web_search_call"` |
| `id` | string | 搜索调用 ID |
| `status` | string | `"completed"` / `"in_progress"` / `"failed"` |
| `action` | string | 搜索动作类型：`"search"` / `"open_page"` / `"find_in_page"` |
| `query` | string | 搜索查询 |

**web_search action 类型**：

| action | 说明 | 关联字段 |
|--------|------|----------|
| `search` | 执行搜索 | `query`（搜索词） |
| `open_page` | 打开搜索结果页面 | — |
| `find_in_page` | 在页面中查找内容 | — |

### 4.5 file_search_call（文件搜索调用）

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"file_search_call"` |
| `id` | string | 搜索调用 ID |
| `status` | string | `"completed"` / `"in_progress"` / `"failed"` |
| `queries` | array | 搜索查询列表 |
| `results` | array | 搜索结果列表（仅当 include 中请求时返回） |

### 4.6 code_interpreter_call（代码解释器调用）

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"code_interpreter_call"` |
| `id` | string | 调用 ID |
| `status` | string | `"completed"` / `"in_progress"` / `"failed"` |
| `code` | string | 执行的代码 |
| `container_id` | string | 容器 ID |
| `outputs` | array | 输出列表（logs 和 image 类型） |

**code_interpreter output 类型**：

| type | 字段 | 说明 |
|------|------|------|
| `logs` | `logs`（string） | 代码执行的标准输出日志 |
| `image` | `file_id`（string） | 生成的图像文件 ID |

### 4.7 reasoning（推理过程）

推理模型的思考过程输出。

| 子类型 | 说明 |
|--------|------|
| `reasoning.summary_text` | 推理摘要文本 |
| `reasoning.reasoning_text` | 完整推理文本 |

### 4.8 compaction（上下文压缩）

当对话过长触发自动压缩时出现。

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"compaction"` |
| `id` | string | 压缩操作 ID |
| `encrypted_content` | string | 加密后的压缩内容 |

### 4.9 computer_call（计算机操作调用）

用于 computer_use_preview 工具。

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"computer_call"` |
| `id` | string | 调用 ID |
| `action` | object | 要执行的操作 |
| `pending_safety_checks` | array | 待确认的安全检查列表 |

**computer_call action 类型**：

| action | 字段 | 说明 |
|--------|------|------|
| `click` | `x`, `y`, `button` | 鼠标点击 |
| `double_click` | `x`, `y` | 双击 |
| `drag` | `path`（数组 of `{x,y}`） | 拖拽路径 |
| `keypress` | `keys`（数组） | 按键 |
| `move` | `x`, `y` | 移动鼠标 |
| `screenshot` | — | 截屏 |
| `scroll` | `x`, `y`, `scroll_x`, `scroll_y` | 滚动 |
| `type` | `text` | 输入文字 |
| `wait` | — | 等待 |

### 4.10 image_generation_call（图像生成调用）

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"image_generation_call"` |
| `id` | string | 调用 ID |
| `status` | string | `"completed"` / `"in_progress"` / `"failed"` |
| `result` | string | 生成的图像（base64 或 URL） |
| `revised_prompt` | string | 修正后的提示词 |

### 4.11 mcp_call（MCP 工具调用）

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"mcp_call"` |
| `id` | string | 调用 ID |
| `server_label` | string | MCP 服务器标签 |
| `name` | string | 工具名称 |
| `arguments` | object | 工具参数 |
| `error` | string \| null | 错误信息 |
| `output` | string \| null | 工具输出 |

### 4.12 mcp_list_tools（MCP 工具列表）

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"mcp_list_tools"` |
| `id` | string | 列表 ID |
| `server_label` | string | MCP 服务器标签 |
| `tools` | array | 可用工具列表 |
| `error` | string \| null | 错误信息 |

### 4.13 mcp_approval_request / mcp_approval_response

MCP 工具执行前需要用户批准的请求和响应。

### 4.14 local_shell_call / local_shell_call_output

本地 Shell 命令执行（Codex CLI 等场景）。

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"local_shell_call"` |
| `id` | string | 调用 ID |
| `action` | array | 命令动作列表 |

**local_shell_call_output**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"local_shell_call_output"` |
| `call_id` | string | 对应的 call ID |
| `outcome` | string | `"exit"` / `"timeout"` |
| `exit_code` | int | 退出码（outcome=exit 时） |
| `output` | string | 命令输出 |

### 4.15 shell_call / shell_call_output

远程 Shell 命令执行。

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"shell_call"` |
| `id` | string | 调用 ID |
| `action` | object | `{type: "exec", commands: [...], timeout: int, max_output_length: int}` |

### 4.16 apply_patch_call / apply_patch_call_output

文件补丁操作。

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"apply_patch_call"` |
| `id` | string | 调用 ID |
| `action` | array | 操作列表 |

**apply_patch_call 的 action 类型**：

| type | 字段 | 说明 |
|------|------|------|
| `create_file` | `path`, `content` | 创建文件 |
| `delete_file` | `path` | 删除文件 |
| `update_file` | `path`, `diff` | 更新文件 |

### 4.17 custom_tool_call / custom_tool_call_output

自定义工具调用和输出。

### 4.18 item_reference

对已有项的引用。

---

## 5. SSE 流式事件类型

启用 `stream: true` 后，响应以 SSE（Server-Sent Events）格式返回。每个事件包含 `event:` 和 `data:` 两个字段。

### 事件格式

```
event: {event_type}
data: {json_payload}
```

### 事件生命周期总览

```
response.created
  → response.in_progress
    → response.output_item.added          (每个输出项开始)
      → response.content_part.added       (每个内容块开始)
        → response.output_text.delta      (文本增量，多次)
        → response.output_text.done       (文本完成)
      → response.content_part.done        (内容块完成)
    → response.output_item.done           (输出项完成)
    → response.function_call_arguments.delta  (函数参数增量，多次)
    → response.function_call_arguments.done    (函数参数完成)
    → response.reasoning_summary_text.delta    (推理摘要增量，多次)
  → response.completed                    (整体完成)
  → response.error                        (错误)
  → response.failed                       (失败)
```

### 5.1 response.created

Response 对象创建时触发。包含完整的 Response 对象（status 为 `in_progress`）。

```json
{
  "type": "response.created",
  "response": {
    "id": "resp_abc123",
    "object": "response",
    "created_at": 1745000000,
    "status": "in_progress",
    "output": [],
    ...
  }
}
```

### 5.2 response.in_progress

Response 开始处理时触发。与 `response.created` 类似但稍后触发。

```json
{
  "type": "response.in_progress",
  "response": { ... }
}
```

### 5.3 response.output_item.added

新的输出项（message 或 function_call）开始生成时触发。

```json
{
  "type": "response.output_item.added",
  "output_index": 0,
  "item": {
    "type": "message",
    "id": "msg_abc123",
    "status": "in_progress",
    "role": "assistant",
    "content": []
  }
}
```

### 5.4 response.content_part.added

新的内容块（如 output_text）开始生成时触发。

```json
{
  "type": "response.content_part.added",
  "item_id": "msg_abc123",
  "output_index": 0,
  "content_index": 0,
  "part": {
    "type": "output_text",
    "text": "",
    "annotations": []
  }
}
```

### 5.5 response.output_text.delta

文本增量输出，是流式响应中最频繁的事件。

```json
{
  "type": "response.output_text.delta",
  "item_id": "msg_abc123",
  "output_index": 0,
  "content_index": 0,
  "delta": "Hello"
}
```

### 5.6 response.output_text.done

文本输出完成时触发，包含完整文本。

```json
{
  "type": "response.output_text.done",
  "item_id": "msg_abc123",
  "output_index": 0,
  "content_index": 0,
  "text": "Hello, world!"
}
```

### 5.7 response.content_part.done

内容块完成时触发，包含最终内容。

```json
{
  "type": "response.content_part.done",
  "item_id": "msg_abc123",
  "output_index": 0,
  "content_index": 0,
  "part": {
    "type": "output_text",
    "text": "Hello, world!",
    "annotations": []
  }
}
```

### 5.8 response.output_item.done

输出项完成时触发，包含完整的输出项数据。

```json
{
  "type": "response.output_item.done",
  "output_index": 0,
  "item": {
    "type": "message",
    "id": "msg_abc123",
    "status": "completed",
    "role": "assistant",
    "content": [
      {
        "type": "output_text",
        "text": "Hello, world!",
        "annotations": []
      }
    ]
  }
}
```

### 5.9 response.function_call_arguments.delta

函数调用参数的增量输出。

```json
{
  "type": "response.function_call_arguments.delta",
  "item_id": "fc_abc123",
  "output_index": 1,
  "delta": "{\"location\":"
}
```

### 5.10 response.function_call_arguments.done

函数调用参数完整输出完成。

```json
{
  "type": "response.function_call_arguments.done",
  "item_id": "fc_abc123",
  "output_index": 1,
  "arguments": "{\"location\":\"San Francisco\"}"
}
```

### 5.11 response.reasoning_summary_text.delta

推理模型的摘要文本增量输出。

```json
{
  "type": "response.reasoning_summary_text.delta",
  "item_id": "msg_abc123",
  "output_index": 0,
  "summary_index": 0,
  "delta": "The problem requires..."
}
```

### 5.12 response.completed

整个 Response 完成时触发，包含完整的 Response 对象（含最终 usage 和 output）。

```json
{
  "type": "response.completed",
  "response": {
    "id": "resp_abc123",
    "object": "response",
    "created_at": 1745000000,
    "completed_at": 1745000001,
    "status": "completed",
    "output": [ ... ],
    "usage": {
      "input_tokens": 25,
      "output_tokens": 50,
      "total_tokens": 75
    },
    ...
  }
}
```

### 5.13 response.error

响应过程中发生错误。

```json
{
  "type": "response.error",
  "response": { ... }
}
```

### 5.14 response.failed

响应失败时触发。

```json
{
  "type": "response.failed",
  "response": { ... }
}
```

### 完整流式交互示例

```
event: response.created
data: {"type":"response.created","response":{"id":"resp_abc","status":"in_progress","output":[]}}

event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"type":"message","id":"msg_abc","status":"in_progress","role":"assistant","content":[]}}

event: response.content_part.added
data: {"type":"response.content_part.added","item_id":"msg_abc","output_index":0,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}}

event: response.output_text.delta
data: {"type":"response.output_text.delta","item_id":"msg_abc","output_index":0,"content_index":0,"delta":"Hello"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","item_id":"msg_abc","output_index":0,"content_index":0,"delta":" world"}

event: response.output_text.done
data: {"type":"response.output_text.done","item_id":"msg_abc","output_index":0,"content_index":0,"text":"Hello world"}

event: response.content_part.done
data: {"type":"response.content_part.done","item_id":"msg_abc","output_index":0,"content_index":0,"part":{"type":"output_text","text":"Hello world","annotations":[]}}

event: response.output_item.done
data: {"type":"response.output_item.done","output_index":0,"item":{"type":"message","id":"msg_abc","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Hello world","annotations":[]}]}}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_abc","status":"completed","output":[...],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}
```

---

## 6. 输入格式 (Input Formats)

`input` 字段支持三种格式：纯文本字符串、内容项数组、消息对象数组。

### 6.1 纯文本字符串

最简单的输入方式，等同于单条 user 消息。

```json
{
  "model": "gpt-4o",
  "input": "What is the capital of France?"
}
```

### 6.2 内容项数组

适合多模态输入，每项指定类型。

#### input_text（文本输入）

```json
{
  "type": "input_text",
  "text": "Describe this image."
}
```

#### input_image（图像输入）

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"input_image"` |
| `image_url` | string | 图像 URL（与 `file_id` 二选一） |
| `file_id` | string | 已上传文件的 ID |
| `detail` | string | 图像细节级别：`"auto"` / `"low"` / `"high"` |

```json
{
  "type": "input_image",
  "image_url": "https://example.com/photo.jpg",
  "detail": "high"
}
```

#### input_file（文件输入）

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"input_file"` |
| `file_data` | string | Base64 编码的文件内容 |
| `file_id` | string | 已上传文件的 ID |
| `filename` | string | 文件名（带扩展名，用于类型推断） |
| `url` | string | 文件 URL |

```json
{
  "type": "input_file",
  "file_data": "data:text/csv;base64,...",
  "filename": "data.csv"
}
```

### 6.3 消息对象数组

类似 Chat Completions 的 messages 格式，用于传递多轮对话。

每条消息是一个带有 `type: "message"` 的对象：

```json
{
  "type": "message",
  "role": "user",
  "content": "Hello!"
}
```

#### 支持的 role

| role | 说明 |
|------|------|
| `"user"` | 用户消息 |
| `"assistant"` | 助手消息 |
| `"system"` | 系统消息（建议使用顶层 `instructions` 代替） |
| `"developer"` | 开发者消息 |

#### content 格式

content 可以是字符串或内容项数组：

**字符串**：
```json
{
  "type": "message",
  "role": "user",
  "content": "Hello!"
}
```

**内容项数组**：
```json
{
  "type": "message",
  "role": "user",
  "content": [
    {"type": "input_text", "text": "What is in this image?"},
    {"type": "input_image", "image_url": "https://example.com/photo.jpg"}
  ]
}
```

**助手消息使用 output_text**：
```json
{
  "type": "message",
  "role": "assistant",
  "content": [
    {"type": "output_text", "text": "The image shows a sunset."}
  ]
}
```

### 6.4 function_call_output（函数结果输入）

用于将函数调用结果传回模型：

```json
{
  "type": "function_call_output",
  "call_id": "call_abc123",
  "output": "{\"temperature\": 72, \"condition\": \"sunny\"}"
}
```

### 6.5 完整多轮对话示例

```json
{
  "model": "gpt-4o",
  "input": [
    {"type": "message", "role": "user", "content": "What's the weather in SF?"},
    {
      "type": "message",
      "role": "assistant",
      "content": [],
    },
    {
      "type": "function_call",
      "call_id": "call_001",
      "name": "get_weather",
      "arguments": "{\"location\":\"San Francisco\"}"
    },
    {
      "type": "function_call_output",
      "call_id": "call_001",
      "output": "{\"temperature\": 72, \"condition\": \"sunny\"}"
    }
  ],
  "tools": [
    {
      "type": "function",
      "name": "get_weather",
      "description": "Get weather for a location",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {"type": "string"}
        },
        "required": ["location"]
      }
    }
  ]
}
```

---

## 7. 工具类型 (Tool Types)

`tools` 数组中的每个工具都有一个 `type` 字段。

### 7.1 function（函数工具）

最常用的工具类型，定义可供模型调用的函数。

```json
{
  "type": "function",
  "name": "get_weather",
  "description": "Get current weather for a location",
  "parameters": {
    "type": "object",
    "properties": {
      "location": {"type": "string", "description": "City name"},
      "unit": {"type": "string", "enum": ["celsius", "fahrenheit"]}
    },
    "required": ["location"]
  },
  "strict": false
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | `"function"` |
| `name` | string | 是 | 函数名称 |
| `description` | string | 否 | 函数描述 |
| `parameters` | object | 否 | JSON Schema 格式的参数定义 |
| `strict` | bool | 否 | 是否严格遵循 schema（默认 false） |

### 7.2 web_search（网络搜索）

允许模型搜索互联网。

```json
{
  "type": "web_search",
  "user_location": {
    "type": "approximate",
    "city": "San Francisco",
    "country": "US"
  },
  "search_context_size": "medium",
  "allowed_domains": ["wikipedia.org", "github.com"]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"web_search"` 或 `"web_search_2025_08_26"` |
| `user_location` | object | 用户位置（影响搜索结果本地化） |
| `search_context_size` | string | 搜索上下文大小：`"low"` / `"medium"` / `"high"` |
| `allowed_domains` | array | 限制搜索的域名列表 |

**user_location 对象**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"approximate"` |
| `city` | string | 城市 |
| `country` | string | 国家代码 |
| `region` | string | 地区 |
| `timezone` | string | 时区 |

### 7.3 file_search（文件搜索）

在向量存储中搜索相关文件。

```json
{
  "type": "file_search",
  "vector_store_ids": ["vs_abc123"],
  "filters": {
    "type": "eq",
    "key": "category",
    "value": "report"
  },
  "ranking": {
    "ranker": "auto",
    "score_threshold": 0.5
  },
  "max_num_results": 10
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"file_search"` |
| `vector_store_ids` | array | 向量存储 ID 列表 |
| `filters` | object | 搜索过滤器 |
| `ranking` | object | 排序配置 |
| `max_num_results` | int | 最大返回结果数 |

**过滤器类型**：

**ComparisonFilter（比较过滤器）**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 比较操作符：`"eq"` / `"ne"` / `"gt"` / `"gte"` / `"lt"` / `"lte"` |
| `key` | string | 过滤字段名 |
| `value` | any | 过滤值 |

**CompoundFilter（复合过滤器）**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"and"` / `"or"` |
| `filters` | array | 子过滤器数组 |

**ranking 对象**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `ranker` | string | 排序算法：`"auto"` / `"rrf"` |
| `score_threshold` | float | 最低相关性分数阈值（0~1） |
| `rrf` | object | Reciprocal Rank Fusion 权重 |

**rrf 权重对象**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `embedding_weight` | float | 向量嵌入权重 |
| `text_weight` | float | 文本权重 |

### 7.4 code_interpreter（代码解释器）

允许模型执行代码。

```json
{
  "type": "code_interpreter",
  "container": {
    "type": "auto",
    "file_ids": ["file_abc123"]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"code_interpreter"` |
| `container` | object | 容器配置 |

**container 对象**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"auto"` |
| `file_ids` | array | 预加载到容器中的文件 ID |

### 7.5 image_generation（图像生成）

允许模型生成图像。

```json
{
  "type": "image_generation",
  "quality": "high",
  "size": "1024x1024",
  "output_format": "png",
  "partial_images": 2,
  "input_image_mask": {
    "image_url": "data:image/png;base64,...",
    "mask": {"type": "auto"}
  },
  "moderation": "auto"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"image_generation"` |
| `quality` | string | 图像质量：`"low"` / `"medium"` / `"high"` |
| `size` | string | 尺寸：`"1024x1024"` / `"1536x1024"` / `"1024x1536"` |
| `output_format` | string | 输出格式：`"png"` / `"jpg"` / `"webp"` |
| `partial_images` | int | 流式返回中间图像数量（0~3） |
| `input_image_mask` | object | 图像编辑遮罩 |
| `moderation` | string | 内容审核级别：`"auto"` / `"low"` |

### 7.6 computer_use_preview（计算机使用）

允许模型操控虚拟计算机环境。

```json
{
  "type": "computer_use_preview",
  "display_width": 1024,
  "display_height": 768,
  "environment": "browser"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"computer_use_preview"` |
| `display_width` | int | 显示宽度（像素） |
| `display_height` | int | 显示高度（像素） |
| `environment` | string | 环境类型：`"browser"` / `"mac"` / `"windows"` / `"ubuntu"` |

### 7.7 mcp（Model Context Protocol）

连接外部 MCP 服务器获取工具。

```json
{
  "type": "mcp",
  "server_label": "github",
  "server_url": "https://mcp.github.com/sse",
  "allowed_tools": ["create_issue", "search_code"],
  "require_approval": "never",
  "oauth_token": "...",
  "headers": {
    "Authorization": "Bearer ..."
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"mcp"` |
| `server_label` | string | 服务器标签（唯一标识） |
| `server_url` | string | MCP 服务器 URL（与 `connector_id` 二选一） |
| `connector_id` | string | 预构建的连接器 ID |
| `allowed_tools` | array | 允许使用的工具列表 |
| `require_approval` | string | 批准要求：`"never"` / `"always"` |
| `oauth_token` | string | OAuth 令牌 |
| `headers` | object | 自定义请求头 |

**预构建 MCP 连接器 ID**：

| connector_id | 服务 |
|--------------|------|
| `connector_dropbox` | Dropbox |
| `connector_gmail` | Gmail |
| `connector_googlecalendar` | Google Calendar |
| `connector_googledrive` | Google Drive |
| `connector_microsoftteams` | Microsoft Teams |
| `connector_outlookcalendar` | Outlook Calendar |
| `connector_outlookemail` | Outlook Email |
| `connector_sharepoint` | SharePoint |

### 7.8 local_shell（本地 Shell）

允许模型执行本地 shell 命令（Codex CLI 等）。

```json
{
  "type": "local_shell"
}
```

### 7.9 shell（远程 Shell）

允许模型执行远程 shell 命令。

```json
{
  "type": "shell",
  "commands": ["ls -la", "cat README.md"],
  "timeout": 30,
  "max_output_length": 10000
}
```

### 7.10 custom（自定义工具）

自定义工具，支持文本或结构化输入。

```json
{
  "type": "custom",
  "name": "my_tool",
  "input_format": {
    "type": "text"
  }
}
```

**input_format 类型**：

| 格式 | 说明 |
|------|------|
| `{"type": "text"}` | 文本输入 |
| `{"type": "grammar", "lark": "..."}` | Lark 语法定义的结构化输入 |
| `{"type": "grammar", "regex": "..."}` | 正则表达式约束的结构化输入 |

### 7.11 apply_patch

文件补丁操作工具。

```json
{
  "type": "apply_patch"
}
```

---

## 8. 推理参数 (Reasoning)

### reasoning 对象

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `effort` | string | 否 | 模型相关 | 推理努力程度。见下方枚举 |
| `summary` | string | 否 | 模型相关 | 推理摘要模式。见下方枚举 |

### effort 枚举

| 值 | 说明 | 备注 |
|----|------|------|
| `"none"` | 不进行推理 | gpt-5.1 默认值 |
| `"minimal"` | 最小推理 | — |
| `"low"` | 低推理 | budget_tokens <= 2048 时映射 |
| `"medium"` | 中等推理 | budget_tokens <= 16384 时映射，大多数推理模型默认 |
| `"high"` | 高推理 | gpt-5-pro 仅支持此级别 |
| `"xhigh"` | 超高推理 | gpt-5.1-codex-max 及之后模型支持 |

**各模型的默认 effort**：

| 模型 | 默认 effort | 支持的 effort |
|------|------------|---------------|
| gpt-5.1 | `none` | none, minimal, low, medium, high |
| gpt-5-pro | `high` | 仅 high |
| o3 | `medium` | low, medium, high |
| o4-mini | `medium` | low, medium, high |
| gpt-4o | 不支持推理 | — |

### summary 枚举

| 值 | 说明 |
|----|------|
| `"auto"` | 自动决定是否生成摘要 |
| `"concise"` | 生成简洁摘要 |
| `"detailed"` | 生成详细摘要 |

> **注意**：`generate_summary` 已废弃，使用 `summary` 代替。

### 请求示例

```json
{
  "model": "o3",
  "input": "Solve: What is 15! / (12! * 3!)?",
  "reasoning": {
    "effort": "high",
    "summary": "detailed"
  }
}
```

### 响应中的 reasoning 字段

```json
{
  "reasoning": {
    "effort": "high",
    "summary": "detailed"
  }
}
```

### 与 Claude thinking 的映射关系

在协议转换层（team-api converter.go），Claude 的 `thinking.budget_tokens` 映射为 Responses 的 `reasoning.effort`：

| budget_tokens | reasoning.effort |
|---------------|-----------------|
| <= 2048 | `"low"` |
| <= 16384 | `"medium"` |
| > 16384 | `"high"` |

---

## 9. 错误格式

### 响应级错误

错误包含在 Response 对象的 `error` 字段中：

```json
{
  "id": "resp_abc123",
  "object": "response",
  "status": "failed",
  "error": {
    "code": "rate_limit_exceeded",
    "message": "You have exceeded your rate limit."
  }
}
```

### HTTP 错误响应

非成功 HTTP 状态码返回的 JSON 错误体：

```json
{
  "error": {
    "message": "Incorrect API key provided: sk_****1234.",
    "type": "invalid_request_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

### 常见错误类型

| HTTP 状态码 | error.type | 说明 |
|------------|-----------|------|
| 400 | `invalid_request_error` | 请求参数错误 |
| 401 | `authentication_error` | API Key 无效 |
| 402 | `insufficient_quota` | 额度不足 |
| 403 | `permission_error` | 权限不足 |
| 404 | `not_found_error` | 资源不存在 |
| 429 | `rate_limit_error` | 请求频率超限 |
| 500 | `internal_error` | 服务器内部错误 |
| 503 | `server_error` | 服务不可用 |

### 流式错误

流式模式下通过 SSE 事件传递错误：

```
event: response.error
data: {"type":"response.error","response":{"status":"failed","error":{"code":"...","message":"..."}}}

event: response.failed
data: {"type":"response.failed","response":{"status":"failed","error":{"code":"...","message":"..."}}}
```

---

## 10. 与 Chat Completions API 的差异

### 10.1 端点差异

| 项目 | Chat Completions | Responses API |
|------|-----------------|---------------|
| 端点 | `POST /v1/chat/completions` | `POST /v1/responses` |
| 设计理念 | 单轮补全 | 对话状态管理 |
| 对话历史 | 客户端维护 messages 数组 | 服务端存储，通过 `previous_response_id` 引用 |

### 10.2 请求参数差异

| Chat Completions | Responses API | 说明 |
|-----------------|---------------|------|
| `messages` | `input` + `instructions` | messages 拆分为 input（用户输入）和 instructions（系统指令） |
| `system` message | `instructions` | 系统提示移到顶层字段 |
| `developer` message | `instructions` | 开发者提示也移到顶层字段 |
| `max_tokens` | `max_output_tokens` | 字段名变化 |
| `max_completion_tokens` | `max_output_tokens` | 统一为一个字段 |
| `response_format` | `text.format` | 嵌套层级变化 |
| `reasoning_effort` | `reasoning.effort` | 从平级字段变为嵌套对象 |
| — | `reasoning.summary` | 新增推理摘要控制 |
| — | `previous_response_id` | 新增对话状态引用 |
| — | `conversation` | 新增对话 ID 管理 |
| — | `store` | 新增存储控制 |
| — | `background` | 新增后台执行模式 |
| — | `include` | 新增额外数据包含控制 |
| — | `service_tier` | 新增服务层级选择 |
| — | `verbosity` | 新增回复详细程度控制 |
| — | `context_management` | 新增上下文压缩管理 |
| — | `max_tool_calls` | 新增工具调用次数限制 |
| — | `prompt` / `prompt_cache_key` | 新增提示词缓存机制 |
| `logprobs` | `logprobs` | 相同 |
| `top_logprobs` | `top_logprobs` | 相同 |
| `tools[].function` | `tools[]` | Responses 的 function 工具扁平化（无嵌套 function 对象） |
| `tool_choice` | `tool_choice` | Responses 支持更多类型（mcp、custom、apply_patch 等） |

### 10.3 响应结构差异

| Chat Completions | Responses API | 说明 |
|-----------------|---------------|------|
| `id: "chatcmpl-..."` | `id: "resp_..."` | ID 前缀不同 |
| `object: "chat.completion"` | `object: "response"` | 对象类型不同 |
| `choices[].message.content` | `output[].content[].text` | 从 choices 数组变为 output 数组，content 拆分为内容块 |
| `choices[].message.tool_calls` | `output[]`（独立 function_call 项） | 工具调用从消息内嵌变为独立输出项 |
| `choices[].finish_reason` | `status` | finish_reason 变为顶层 status |
| `usage.prompt_tokens` | `usage.input_tokens` | 字段名变化 |
| `usage.completion_tokens` | `usage.output_tokens` | 字段名变化 |
| — | `output[].type` | 新增输出项类型标识 |
| — | `output[].id` | 新增输出项独立 ID |
| — | `completed_at` | 新增完成时间戳 |

### 10.4 工具调用差异

**Chat Completions 工具调用流程**：

```
1. 模型返回 tool_calls 数组（在 message 内）
2. 客户端执行函数
3. 客户端将结果作为 role="tool" 的 message 发回
```

**Responses API 工具调用流程**：

```
1. 模型返回独立的 function_call 输出项（在 output 数组中）
2. 客户端执行函数
3. 客户端将结果作为 function_call_output 输入项发回
4. 通过 call_id 匹配调用和结果
```

### 10.5 流式响应差异

| Chat Completions | Responses API | 说明 |
|-----------------|---------------|------|
| `data: {"choices":[{"delta":...}]}` | `event: response.output_text.delta\ndata: {"delta":...}` | 格式完全不同 |
| 无事件类型 | 有明确事件类型 | Responses 每个事件有 `event:` 行 |
| `data: [DONE]` | `event: response.completed` | 结束信号不同 |
| 单一流 | 结构化多级事件流 | Responses 有 created/added/delta/done/completed 多级 |
| delta 中混合所有类型 | 按类型分开发送 | 文本 delta 和工具调用 delta 分开 |

### 10.6 协议转换映射（team-api 实现细节）

在 team-api 的 `converter.go` 中实现了双向转换：

**Responses → Chat Completions（请求）**：

| Responses 字段 | Chat Completions 字段 |
|---------------|----------------------|
| `instructions` | `messages[0]` (role=system) |
| `input` (string) | `messages[1]` (role=user, content=string) |
| `input` (array, type=message) | `messages[n]` (role=message.role) |
| `input` (array, type=function_call_output) | `messages[n]` (role=tool, tool_call_id=call_id) |
| `tools[].type=function` | `tools[].type=function, function={name,description,parameters}` |
| `reasoning.effort` | `reasoning_effort` |
| `text.format` (json_schema) | `response_format` |
| `stream` + `true` | `stream=true, stream_options={include_usage:true}` |

**Chat Completions → Responses（请求）**：

| Chat Completions 字段 | Responses 字段 |
|----------------------|---------------|
| `messages[role=system/developer]` | `instructions`（合并为单个字符串） |
| `messages[role=user]` | `input[]` (type=message, role=user) |
| `messages[role=assistant]` | `input[]` (type=message, role=assistant) |
| `messages[role=assistant].tool_calls` | `input[]` (type=function_call) |
| `messages[role=tool]` | `input[]` (type=function_call_output) |
| `max_tokens` / `max_completion_tokens` | `max_output_tokens`（取较大值） |
| `response_format` | `text.format` |
| `reasoning_effort` | `reasoning.effort` + `reasoning.summary="detailed"` |
| `tools[].function` | `tools[]` (扁平化) |

**Responses → Chat Completions（响应）**：

| Responses 字段 | Chat Completions 字段 |
|---------------|----------------------|
| `output[type=message].content[type=output_text].text` | `choices[0].message.content` |
| `output[type=function_call]` | `choices[0].message.tool_calls[]` |
| `status="completed"` | `choices[0].finish_reason="stop"` |
| `status="completed"` + 有 tool_calls | `choices[0].finish_reason="tool_calls"` |
| `usage.input_tokens` | `usage.prompt_tokens` |
| `usage.output_tokens` | `usage.completion_tokens` |
| `usage.total_tokens` | `usage.total_tokens` |

**Responses SSE → Chat SSE（流式）**：

| Responses 事件 | Chat Completions 输出 |
|---------------|----------------------|
| `response.output_text.delta` | `choices[0].delta.content` |
| `response.reasoning_summary_text.delta` | `choices[0].delta.reasoning_content` |
| `response.output_item.added` (function_call) | `choices[0].delta.tool_calls[]` |
| `response.function_call_arguments.delta` | `choices[0].delta.tool_calls[].function.arguments` |
| `response.completed` | `choices[0].finish_reason` + usage chunk + `[DONE]` |
| `response.error` / `response.failed` | 错误返回 |

---

## 附录 A：team-api DTO 结构体参考

以下为 `relay/dto/openai_responses.go` 中定义的核心结构体，供开发参考。

### OpenAIResponsesRequest

```go
type OpenAIResponsesRequest struct {
    Model              string          `json:"model"`
    Input              json.RawMessage `json:"input,omitempty"`
    Include            json.RawMessage `json:"include,omitempty"`
    Instructions       json.RawMessage `json:"instructions,omitempty"`
    MaxOutputTokens    *uint           `json:"max_output_tokens,omitempty"`
    Metadata           json.RawMessage `json:"metadata,omitempty"`
    ParallelToolCalls  json.RawMessage `json:"parallel_tool_calls,omitempty"`
    PreviousResponseID string          `json:"previous_response_id,omitempty"`
    Reasoning          *Reasoning      `json:"reasoning,omitempty"`
    Store              json.RawMessage `json:"store,omitempty"`
    Stream             *bool           `json:"stream,omitempty"`
    StreamOptions      *StreamOptions  `json:"stream_options,omitempty"`
    Temperature        *float64        `json:"temperature,omitempty"`
    Text               json.RawMessage `json:"text,omitempty"`
    ToolChoice         json.RawMessage `json:"tool_choice,omitempty"`
    Tools              json.RawMessage `json:"tools,omitempty"`
    TopP               *float64        `json:"top_p,omitempty"`
    TopLogProbs        *int            `json:"top_logprobs,omitempty"`
    Truncation         json.RawMessage `json:"truncation,omitempty"`
    User               json.RawMessage `json:"user,omitempty"`
    MaxToolCalls       *uint           `json:"max_tool_calls,omitempty"`
    Prompt             json.RawMessage `json:"prompt,omitempty"`
    ServiceTier        string          `json:"service_tier,omitempty"`
    Conversation       json.RawMessage `json:"conversation,omitempty"`
    ContextManagement  json.RawMessage `json:"context_management,omitempty"`
}
```

### OpenAIResponsesResponse

```go
type OpenAIResponsesResponse struct {
    ID                 string              `json:"id"`
    Object             string              `json:"object"`
    CreatedAt          int                 `json:"created_at"`
    CompletedAt        int                 `json:"completed_at,omitempty"`
    Status             json.RawMessage     `json:"status"`
    Error              any                 `json:"error"`
    IncompleteDetails  any                 `json:"incomplete_details"`
    Instructions       any                 `json:"instructions"`
    MaxOutputTokens    *int                `json:"max_output_tokens"`
    Model              string              `json:"model"`
    Output             []ResponsesOutput   `json:"output"`
    ParallelToolCalls  bool                `json:"parallel_tool_calls"`
    PreviousResponseID any                 `json:"previous_response_id"`
    Reasoning          *ResponsesReasoning `json:"reasoning"`
    Store              bool                `json:"store"`
    Temperature        *float64            `json:"temperature"`
    Text               *ResponsesText      `json:"text,omitempty"`
    ToolChoice         any                 `json:"tool_choice"`
    Tools              []any               `json:"tools"`
    TopP               *float64            `json:"top_p"`
    Truncation         any                 `json:"truncation"`
    Usage              *ResponsesUsage     `json:"usage,omitempty"`
    User               any                 `json:"user"`
    Metadata           any                 `json:"metadata"`
}
```

### ResponsesStreamResponse

```go
type ResponsesStreamResponse struct {
    Type         string                   `json:"type"`
    Response     *OpenAIResponsesResponse `json:"response,omitempty"`
    Delta        string                   `json:"delta,omitempty"`
    Item         *ResponsesOutput         `json:"item,omitempty"`
    OutputIndex  *int                     `json:"output_index,omitempty"`
    ContentIndex *int                     `json:"content_index,omitempty"`
    SummaryIndex *int                     `json:"summary_index,omitempty"`
    ItemID       string                   `json:"item_id,omitempty"`
    Part         *ResponsesSummaryPart    `json:"part,omitempty"`
}
```

---

## 附录 B：关键代码文件索引

| 文件 | 路径 | 说明 |
|------|------|------|
| DTO 定义 | `relay/dto/openai_responses.go` | 所有 Responses API 请求/响应结构体 |
| 协议转换 | `relay/channel/openai/converter.go` | Responses ↔ Chat Completions 双向转换 |
| 响应处理 | `relay/channel/openai/responses.go` | Chat 响应 → Responses 格式的 SSE 转换 |
