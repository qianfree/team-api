# OpenAI Responses API 协议文档

> 基于 OpenAI 官方 API Reference 整理，用于协议转换器实现参考。
> 端点基础路径：`https://api.openai.com/v1/responses`
> 官方文档：https://platform.openai.com/docs/api-reference/responses

---

## 目录

- [1. API 概述](#1-api-概述)
- [2. 创建响应 — POST /v1/responses](#2-创建响应--post-v1responses)
  - [2.1 请求体参数](#21-请求体参数)
  - [2.2 输入项类型（Input Items）](#22-输入项类型input-items)
  - [2.3 输出项类型（Output Items）](#23-输出项类型output-items)
  - [2.4 工具定义（Tools）](#24-工具定义tools)
  - [2.5 工具选择（Tool Choice）](#25-工具选择tool-choice)
  - [2.6 推理配置（Reasoning）](#26-推理配置reasoning)
  - [2.7 文本输出格式（Text Format）](#27-文本输出格式text-format)
  - [2.8 上下文管理（Context Management）](#28-上下文管理context-management)
  - [2.9 对话管理（Conversation）](#29-对话管理conversation)
  - [2.10 流式选项（Stream Options）](#210-流式选项stream-options)
  - [2.11 其他请求参数](#211-其他请求参数)
- [3. 响应对象（Response Object）](#3-响应对象response-object)
  - [3.1 完整响应结构](#31-完整响应结构)
  - [3.2 响应示例](#32-响应示例)
  - [3.3 状态值（Status）](#33-状态值status)
  - [3.4 Usage 统计](#34-usage-统计)
  - [3.5 错误对象](#35-错误对象)
  - [3.6 不完整详情](#36-不完整详情)
- [4. 其他端点](#4-其他端点)
  - [4.1 获取响应 — GET /v1/responses/{response_id}](#41-获取响应--get-v1responsesresponse_id)
  - [4.2 删除响应 — DELETE /v1/responses/{response_id}](#42-删除响应--delete-v1responsesresponse_id)
  - [4.3 取消响应 — POST /v1/responses/{response_id}/cancel](#43-取消响应--post-v1responsesresponse_idcancel)
  - [4.4 压缩对话 — POST /v1/responses/compact](#44-压缩对话--post-v1responsescompact)
  - [4.5 列出输入项 — GET /v1/responses/{response_id}/input_items](#45-列出输入项--get-v1responsesresponse_idinput_items)
  - [4.6 计算输入 Token — POST /v1/responses/input_tokens](#46-计算输入-token--post-v1responsesinput_tokens)
- [5. 与 Chat Completions API 的对比](#5-与-chat-completions-api-的对比)

---

## 1. API 概述

OpenAI Responses API 是 OpenAI 推出的新一代模型响应 API，旨在替代 Chat Completions API。它提供了更丰富的功能：

- **统一的输入/输出模型**：使用 `input` 和 `output` 数组代替传统的 `messages` 结构
- **内置工具支持**：原生支持 Web 搜索、文件搜索、代码解释器、图像生成、计算机控制等内置工具
- **MCP 协议集成**：支持通过 Model Context Protocol 连接第三方工具服务器
- **推理控制**：支持调节推理力度（effort）和生成推理摘要（summary）
- **上下文管理**：支持对话压缩（compaction）以管理长对话
- **后台模式**：支持异步处理响应
- **结构化输出**：支持 JSON Schema 约束的结构化输出
- **多轮对话**：通过 `previous_response_id` 或 `conversation` 对象实现

端点总览：

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/v1/responses` | 创建模型响应 |
| GET | `/v1/responses/{response_id}` | 获取响应 |
| DELETE | `/v1/responses/{response_id}` | 删除响应 |
| POST | `/v1/responses/{response_id}/cancel` | 取消响应 |
| POST | `/v1/responses/compact` | 压缩对话 |
| GET | `/v1/responses/{response_id}/input_items` | 列出输入项 |
| POST | `/v1/responses/input_tokens` | 计算输入 Token 数 |

---

## 2. 创建响应 — POST /v1/responses

创建模型响应。提供文本、图片或文件输入以生成文本或 JSON 输出。可以让模型调用自定义代码或使用内置工具（如 Web 搜索、文件搜索）。

### 2.1 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | **是** | — | 模型 ID，如 `gpt-4o`、`o3`、`gpt-4.1` |
| `input` | string / array | **是** | — | 模型输入，可以是字符串（等同于 user 角色的文本输入）或输入项数组 |
| `instructions` | string | 否 | null | 系统（或开发者）消息，插入到模型上下文中。使用 `previous_response_id` 时不会继承之前的 instructions |
| `tools` | array | 否 | `[]` | 模型可以调用的工具数组 |
| `tool_choice` | string / object | 否 | `"auto"` | 模型如何选择工具 |
| `temperature` | number | 否 | 1 | 采样温度，0-2 之间。建议与 `top_p` 只调整一个 |
| `top_p` | number | 否 | 1 | 核采样概率质量。建议与 `temperature` 只调整一个 |
| `max_output_tokens` | integer | 否 | null | 输出 Token 上限（含可见输出和推理 Token） |
| `max_tool_calls` | integer | 否 | — | 内置工具最大总调用次数（跨所有内置工具合计） |
| `stream` | boolean | 否 | false | 是否启用流式响应 |
| `stream_options` | object | 否 | null | 流式选项，仅在 `stream: true` 时设置 |
| `store` | boolean | 否 | true | 是否存储响应以供后续 API 检索 |
| `reasoning` | object | 否 | — | 推理配置 |
| `text` | object | 否 | — | 文本输出格式配置 |
| `previous_response_id` | string | 否 | null | 上一个响应的唯一 ID，用于创建多轮对话。不能与 `conversation` 同时使用 |
| `conversation` | object | 否 | null | 对话对象，指定此响应所属的对话 |
| `background` | boolean | 否 | false | 是否在后台运行模型响应 |
| `include` | array | 否 | — | 指定要包含的附加输出数据 |
| `parallel_tool_calls` | boolean | 否 | true | 是否允许模型并行调用工具 |
| `truncation` | string | 否 | `"disabled"` | 截断策略：`"auto"` 或 `"disabled"` |
| `metadata` | object | 否 | `{}` | 最多 16 个键值对的元数据。键最长 64 字符，值最长 512 字符 |
| `user` | string | 否 | null | 终端用户标识（**已弃用**，使用 `prompt_cache_key` 和 `safety_identifier` 替代） |
| `prompt_cache_key` | string | 否 | — | 用于缓存优化的稳定标识符，替代 `user` 字段 |
| `prompt_cache_retention` | string | 否 | — | 提示缓存保留策略，设为 `24h` 启用扩展缓存 |
| `safety_identifier` | string | 否 | — | 用于检测违规用户的安全标识符 |
| `service_tier` | string | 否 | `"auto"` | 服务等级：`"auto"` / `"default"` / `"flex"` / `"priority"` |
| `prompt` | object | 否 | — | 提示模板引用及其变量 |
| `logprobs` | integer | 否 | — | 返回每个位置最可能的 Token 数量，0-20 |

#### `include` 参数支持的值

| 值 | 说明 |
|----|------|
| `web_search_call.action.sources` | 包含 Web 搜索工具调用的来源 |
| `code_interpreter_call.outputs` | 包含代码解释器中 Python 代码执行的输出 |
| `computer_call_output.output.image_url` | 包含计算机调用的截图图片 URL |
| `file_search_call.results` | 包含文件搜索工具调用的搜索结果 |
| `message.input_image.image_url` | 包含输入消息中的图片 URL |
| `message.output_text.logprobs` | 包含助手消息的 logprobs |
| `reasoning.encrypted_content` | 包含推理项的加密内容，用于无状态多轮对话 |

### 2.2 输入项类型（Input Items）

`input` 参数支持多种形式：

1. **纯字符串**：等同于 user 角色的文本输入
2. **输入项数组**：包含不同类型内容项的数组

#### 2.2.1 input_text — 文本输入

```json
{
  "type": "input_text",
  "text": "Hello, how are you?"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"input_text"` |
| `text` | string | 是 | 文本内容 |

#### 2.2.2 input_image — 图片输入

```json
{
  "type": "input_image",
  "image_url": "https://example.com/image.png",
  "detail": "auto"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"input_image"` |
| `image_url` | string | 否* | 图片 URL（完整 URL 或 base64 data URL）。与 `file_id` 二选一 |
| `file_id` | string | 否* | 上传文件的 ID。与 `image_url` 二选一 |
| `detail` | string | 否 | 图片细节级别：`"high"` / `"low"` / `"auto"`，默认 `"auto"` |

#### 2.2.3 input_file — 文件输入

```json
{
  "type": "input_file",
  "file_url": "https://example.com/document.pdf",
  "filename": "document.pdf"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"input_file"` |
| `file_url` | string | 否* | 文件 URL。与 `file_id` 和 `content` 三选一 |
| `file_id` | string | 否* | 上传文件的 ID。与 `file_url` 和 `content` 三选一 |
| `content` | string | 否* | 文件内容。与 `file_url` 和 `file_id` 三选一 |
| `filename` | string | 否 | 文件名 |

#### 2.2.4 message — 消息输入

用于带角色指示的消息输入。`developer` 或 `system` 角色的指令优先级高于 `user` 角色。`assistant` 角色的消息被视为模型在之前交互中生成的。

**作为输入项使用（简写格式）：**

```json
{
  "role": "user",
  "content": "Tell me a story."
}
```

**完整格式（从 API 返回时）：**

```json
{
  "type": "message",
  "id": "msg_abc123",
  "role": "user",
  "content": [
    {
      "type": "input_text",
      "text": "Tell me a story."
    }
  ],
  "status": "completed"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 条件 | 固定值 `"message"`，API 返回时总有，输入时可省略 |
| `id` | string | 否 | 消息唯一 ID，API 返回时填充 |
| `role` | string | 是 | 角色：`"user"` / `"assistant"` / `"system"` / `"developer"` |
| `content` | string / array | 是 | 消息内容，可以是字符串或内容项数组 |
| `status` | string | 否 | 项目状态：`"in_progress"` / `"completed"` / `"incomplete"` |

#### 2.2.5 item_reference — 项引用

```json
{
  "type": "item_reference",
  "id": "msg_abc123"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"item_reference"` |
| `id` | string | 是 | 要引用的项 ID |

#### 2.2.6 function_call_output — 函数调用输出

```json
{
  "type": "function_call_output",
  "call_id": "call_abc123",
  "output": "{\"temperature\": 72}"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"function_call_output"` |
| `call_id` | string | 是 | 对应函数调用的唯一 ID |
| `output` | string / array | 是 | 函数调用的输出，可以是 JSON 字符串或内容输出数组 |
| `id` | string | 否 | API 返回时填充的唯一 ID |
| `status` | string | 否 | 项目状态 |

`output` 的内容输出数组可包含以下类型：
- `input_text`：文本输出
- `input_image`：图片输出
- `input_file`：文件输出

#### 2.2.7 computer_call_output — 计算机调用输出

```json
{
  "type": "computer_call_output",
  "call_id": "call_abc123",
  "output": {
    "type": "computer_screenshot",
    "image_url": "data:image/png;base64,..."
  },
  "acknowledged_safety_checks": [
    {
      "id": "safety_check_001",
      "code": "malicious_content",
      "message": "Potential malicious content detected"
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"computer_call_output"` |
| `call_id` | string | 是 | 对应计算机调用的 ID |
| `output` | object | 是 | 计算机截图对象 |
| `acknowledged_safety_checks` | array | 否 | 已确认的安全检查列表 |
| `id` | string | 否 | API 返回时填充 |
| `status` | string | 否 | 项目状态 |

**computer_screenshot 对象：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"computer_screenshot"` |
| `image_url` | string | 否 | 截图图片 URL |
| `file_id` | string | 否 | 截图文件 ID |

#### 2.2.8 mcp_approval_response — MCP 审批响应

```json
{
  "type": "mcp_approval_response",
  "approval_request_id": "apr_abc123",
  "approve": true,
  "reason": "This operation is safe"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"mcp_approval_response"` |
| `approval_request_id` | string | 是 | 要回应的审批请求 ID |
| `approve` | boolean | 是 | 是否批准 |
| `id` | string | 否 | 唯一 ID |
| `reason` | string | 否 | 决定原因 |

### 2.3 输出项类型（Output Items）

响应的 `output` 数组可以包含以下类型的项。

#### 2.3.1 message — 输出消息

模型生成的消息输出。

```json
{
  "type": "message",
  "id": "msg_abc123",
  "status": "completed",
  "role": "assistant",
  "content": [
    {
      "type": "output_text",
      "text": "Hello! How can I help you?",
      "annotations": []
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"message"` |
| `id` | string | 输出消息唯一 ID |
| `status` | string | 状态：`"in_progress"` / `"completed"` / `"incomplete"` |
| `role` | string | 固定值 `"assistant"` |
| `content` | array | 内容数组，包含 `output_text` 和/或 `refusal` |

**output_text 内容类型：**

```json
{
  "type": "output_text",
  "text": "Hello!",
  "annotations": [
    {
      "type": "url_citation",
      "url": "https://example.com",
      "title": "Example",
      "start_index": 0,
      "end_index": 10
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"output_text"` |
| `text` | string | 模型输出的文本 |
| `annotations` | array | 注解数组，包含以下类型 |

**注解类型：**

| 类型 | 字段 | 说明 |
|------|------|------|
| `file_citation` | `type`, `file_id`, `filename`, `index` | 文件引用 |
| `url_citation` | `type`, `url`, `title`, `start_index`, `end_index` | URL 引用 |
| `container_file_citation` | `type`, `file_id`, `filename`, `start_index`, `end_index`, `index` | 容器文件引用 |
| `file_path` | `type`, `file_id`, `index` | 文件路径引用 |

**refusal 内容类型：**

```json
{
  "type": "refusal",
  "refusal": "I'm sorry, but I cannot assist with that request."
}
```

#### 2.3.2 function_call — 函数调用

```json
{
  "type": "function_call",
  "id": "fc_abc123",
  "call_id": "call_abc123",
  "name": "get_weather",
  "arguments": "{\"location\": \"San Francisco\"}",
  "status": "completed"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"function_call"` |
| `id` | string | 函数调用的唯一 ID |
| `call_id` | string | 用于映射函数调用输出的标识符 |
| `name` | string | 要运行的函数名 |
| `arguments` | string | 传递给函数的 JSON 字符串参数 |
| `status` | string | 状态：`"in_progress"` / `"completed"` / `"incomplete"` |

#### 2.3.3 web_search_call — Web 搜索调用

```json
{
  "type": "web_search_call",
  "id": "ws_abc123",
  "status": "completed",
  "action": {
    "type": "search",
    "query": "latest news about AI",
    "sources": [
      {
        "type": "url",
        "url": "https://example.com/article"
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"web_search_call"` |
| `id` | string | Web 搜索调用的唯一 ID |
| `status` | string | 状态 |
| `action` | object | 搜索动作描述 |

**action 类型：**

| 类型 | 说明 | 关键字段 |
|------|------|----------|
| `search` | 执行 Web 搜索查询 | `query`（已弃用）, `sources` |
| `open_page` | 打开搜索结果中的 URL | `url`, `sources` |
| `find_in_page` | 在已加载页面中搜索 | `pattern`, `url` |

#### 2.3.4 file_search_call — 文件搜索调用

```json
{
  "type": "file_search_call",
  "id": "fs_abc123",
  "status": "completed",
  "queries": ["quarterly revenue"],
  "results": [
    {
      "file_id": "file-abc123",
      "text": "Q4 revenue was $5.2M...",
      "score": 0.95
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"file_search_call"` |
| `id` | string | 文件搜索调用的唯一 ID |
| `status` | string | 状态：`"in_progress"` / `"searching"` / `"incomplete"` / `"failed"` |
| `queries` | array | 搜索查询列表 |
| `results` | array | 搜索结果列表 |

**搜索结果项：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `file_id` | string | 文件唯一 ID |
| `text` | string | 从文件中检索到的文本 |
| `score` | number | 相关性评分（0-1） |

#### 2.3.5 computer_call — 计算机调用

```json
{
  "type": "computer_call",
  "id": "cu_abc123",
  "call_id": "call_abc123",
  "action": {
    "type": "click",
    "x": 100,
    "y": 200,
    "button": "left"
  },
  "pending_safety_checks": [],
  "status": "completed"
}
```

**action 类型：**

| 类型 | 字段 | 说明 |
|------|------|------|
| `click` | `x`, `y`, `button` | 鼠标点击。`button`: `"left"` / `"right"` / `"wheel"` / `"back"` / `"forward"` |
| `double_click` | `x`, `y` | 双击 |
| `drag` | `path` (坐标数组 `[{x, y}, ...]`) | 拖拽操作 |
| `keypress` | `keys` (字符串数组) | 按键操作 |
| `move` | `x`, `y` | 鼠标移动 |
| `scroll` | `x`, `y`, `scroll_x`, `scroll_y` | 滚动操作 |
| `type` | `text` | 输入文本 |
| `wait` | — | 等待 |
| `screenshot` | — | 截图 |

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"computer_call"` |
| `id` | string | 计算机调用的唯一 ID |
| `call_id` | string | 用于响应的标识符 |
| `action` | object | 要执行的动作 |
| `pending_safety_checks` | array | 待处理的安全检查列表 |
| `status` | string | 状态 |

#### 2.3.6 reasoning — 推理

```json
{
  "type": "reasoning",
  "id": "rs_abc123",
  "summary": [
    {
      "type": "summary_text",
      "text": "I analyzed the input and determined..."
    }
  ],
  "encrypted_content": "encrypted-data...",
  "status": "completed"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"reasoning"` |
| `id` | string | 推理内容唯一标识符 |
| `summary` | array | 推理摘要内容数组，每项包含 `type: "summary_text"` 和 `text` |
| `encrypted_content` | string | 加密的推理内容（在 `include` 中包含 `reasoning.encrypted_content` 时填充） |
| `status` | string | 状态 |

推理项还可以包含 `reasoning_text` 类型的内容：

```json
{
  "type": "reasoning",
  "id": "rs_abc123",
  "content": [
    {
      "type": "reasoning_text",
      "text": "Let me think about this step by step..."
    }
  ]
}
```

#### 2.3.7 image_generation_call — 图像生成调用

```json
{
  "type": "image_generation_call",
  "id": "ig_abc123",
  "status": "completed",
  "result": "base64-encoded-image-data"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"image_generation_call"` |
| `id` | string | 图像生成调用的唯一 ID |
| `status` | string | 状态 |
| `result` | string | Base64 编码的生成图像 |

#### 2.3.8 code_interpreter_call — 代码解释器调用

```json
{
  "type": "code_interpreter_call",
  "id": "ci_abc123",
  "status": "completed",
  "code": "print('Hello, World!')",
  "container_id": "container_abc123",
  "outputs": [
    {
      "type": "logs",
      "logs": "Hello, World!\n"
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"code_interpreter_call"` |
| `id` | string | 代码解释器调用的唯一 ID |
| `status` | string | 状态：`"in_progress"` / `"completed"` / `"incomplete"` / `"interpreting"` / `"failed"` |
| `code` | string | 要运行的代码，不可用时为 null |
| `container_id` | string | 运行代码的容器 ID |
| `outputs` | array | 输出列表，包含 `logs` 或 `image` 类型 |

**outputs 类型：**

| 类型 | 字段 | 说明 |
|------|------|------|
| `logs` | `type: "logs"`, `logs` | 日志输出 |
| `image` | `type: "image"`, `url` | 图像输出（URL） |

#### 2.3.9 shell_call — Shell 调用

```json
{
  "type": "shell_call",
  "id": "sh_abc123",
  "call_id": "call_abc123",
  "status": "completed",
  "action": {
    "type": "exec",
    "command": ["ls", "-la"],
    "max_output_length": 10000,
    "timeout": 30000
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"shell_call"` |
| `id` | string | Shell 调用的唯一 ID |
| `call_id` | string | 模型生成的调用 ID |
| `status` | string | 状态 |
| `action` | object | Shell 命令和限制 |

**action 字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"exec"` |
| `command` | array | 有序的 Shell 命令列表 |
| `max_output_length` | integer | 最大输出字符数（UTF-8） |
| `timeout` | integer | 最大运行时间（毫秒） |

#### 2.3.10 local_shell_call — 本地 Shell 调用

```json
{
  "type": "local_shell_call",
  "id": "lsh_abc123",
  "call_id": "call_abc123",
  "status": "completed",
  "action": {
    "type": "exec",
    "command": ["npm", "test"],
    "env": {"NODE_ENV": "test"},
    "timeout": 60000,
    "user": "runner",
    "working_directory": "/app"
  }
}
```

与 `shell_call` 类似，但包含额外的本地执行环境参数。

#### 2.3.11 apply_patch_call — 应用补丁调用

```json
{
  "type": "apply_patch_call",
  "id": "ap_abc123",
  "call_id": "call_abc123",
  "status": "completed",
  "action": {
    "type": "create_file",
    "path": "src/newfile.js",
    "patch": "--- /dev/null\n+++ src/newfile.js\n@@ -0,0 +1 @@\n+console.log('hello');"
  }
}
```

**action 操作类型：**

| 操作类型 | 字段 | 说明 |
|----------|------|------|
| `create_file` | `path`, `patch`, `type` | 创建新文件 |
| `delete_file` | `path`, `type` | 删除文件 |
| `update_file` | `path`, `patch`, `type` | 更新文件（unified diff） |

#### 2.3.12 mcp_call — MCP 工具调用

```json
{
  "type": "mcp_call",
  "id": "mcp_abc123",
  "server_label": "deepwiki",
  "name": "read_wiki",
  "arguments": "{\"page\": \"Go_(programming_language)\"}",
  "output": "Go is a statically typed...",
  "status": "completed",
  "approval_request_id": "apr_abc123",
  "error": null
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"mcp_call"` |
| `id` | string | 工具调用的唯一 ID |
| `server_label` | string | MCP 服务器标签 |
| `name` | string | 运行的工具名称 |
| `arguments` | string | JSON 字符串格式的参数 |
| `output` | string | 工具调用输出 |
| `error` | string | 错误信息（如果有） |
| `status` | string | 状态：`"in_progress"` / `"completed"` / `"incomplete"` / `"calling"` / `"failed"` |
| `approval_request_id` | string | MCP 工具调用审批请求的唯一标识符 |

#### 2.3.13 mcp_list_tools — MCP 工具列表

```json
{
  "type": "mcp_list_tools",
  "id": "mlt_abc123",
  "server_label": "deepwiki",
  "tools": [
    {
      "name": "read_wiki",
      "description": "Read a wiki page",
      "inputSchema": {"type": "object", "properties": {...}},
      "annotations": {}
    }
  ],
  "error": null
}
```

#### 2.3.14 mcp_approval_request — MCP 审批请求

```json
{
  "type": "mcp_approval_request",
  "id": "apr_abc123",
  "server_label": "deepwiki",
  "name": "delete_page",
  "arguments": "{\"page\": \"test\"}"
}
```

#### 2.3.15 custom_tool_call — 自定义工具调用

```json
{
  "type": "custom_tool_call",
  "id": "ctc_abc123",
  "call_id": "call_abc123",
  "name": "my_tool",
  "input": "some input data",
  "status": "completed"
}
```

**custom_tool_call_output — 自定义工具调用输出：**

```json
{
  "type": "custom_tool_call_output",
  "call_id": "call_abc123",
  "output": "tool output data"
}
```

#### 2.3.16 compaction — 压缩项

```json
{
  "type": "compaction",
  "id": "cmp_001",
  "encrypted_content": "gAAAAABpM0Yj-...="
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"compaction"` |
| `id` | string | 压缩项 ID |
| `encrypted_content` | string | 加密的压缩摘要内容 |

### 2.4 工具定义（Tools）

`tools` 参数是一个数组，支持以下工具类别：

1. **内置工具**：OpenAI 提供的工具（Web 搜索、文件搜索等）
2. **MCP 工具**：通过 MCP 协议连接第三方系统
3. **函数调用（自定义工具）**：用户自定义的函数

#### 2.4.1 function — 函数工具

```json
{
  "type": "function",
  "name": "get_weather",
  "description": "Get the current weather for a location",
  "parameters": {
    "type": "object",
    "properties": {
      "location": {
        "type": "string",
        "description": "City name"
      }
    },
    "required": ["location"]
  },
  "strict": true
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"function"` |
| `name` | string | 是 | 函数名称 |
| `description` | string | 否 | 函数描述，帮助模型决定是否调用 |
| `parameters` | object | 否 | JSON Schema 格式的参数描述 |
| `strict` | boolean | 否 | 是否强制严格参数验证，默认 `true` |

#### 2.4.2 web_search / web_search_preview — Web 搜索工具

```json
{
  "type": "web_search_preview",
  "search_context_size": "medium",
  "user_location": {
    "type": "approximate",
    "city": "San Francisco",
    "region": "California"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"web_search"` 或 `"web_search_preview"` 或 `"web_search_preview_2025_03_11"` |
| `search_context_size` | string | 上下文窗口使用量指导：`"low"` / `"medium"` / `"high"`，默认 `"medium"` |
| `user_location` | object | 用户的大致位置 |

**user_location 对象：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"approximate"` |
| `city` | string | 城市名（自由文本） |
| `region` | string | 地区名（自由文本） |

#### 2.4.3 web_search（2025-08-26 版本）

```json
{
  "type": "web_search",
  "search_context_size": "medium",
  "allowed_domains": ["pubmed.ncbi.nlm.nih.gov"],
  "user_location": {
    "type": "approximate",
    "city": "San Francisco",
    "region": "California"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"web_search"` 或 `"web_search_2025_08_26"` |
| `allowed_domains` | array | 允许的搜索域名。不提供则允许所有域名。子域名也被允许 |
| `search_context_size` | string | 上下文窗口使用量指导 |
| `user_location` | object | 用户的大致位置 |

#### 2.4.4 file_search — 文件搜索工具

```json
{
  "type": "file_search",
  "vector_store_ids": ["vs_abc123"],
  "max_num_results": 10,
  "ranking_options": {
    "ranker": "auto",
    "score_threshold": 0.5
  },
  "filters": {
    "type": "and",
    "filters": [
      {
        "type": "eq",
        "key": "category",
        "value": "finance"
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"file_search"` |
| `vector_store_ids` | array | 要搜索的向量存储 ID 列表 |
| `max_num_results` | integer | 最大返回结果数（1-50） |
| `ranking_options` | object | 搜索排名选项 |
| `filters` | object | 搜索过滤器 |

**ranking_options 对象：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `ranker` | string | 使用的排名器 |
| `score_threshold` | number | 分数阈值（0-1） |

**filters 类型：**

比较过滤器：

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 比较运算符：`"eq"` / `"ne"` / `"gt"` / `"gte"` / `"lt"` / `"lte"` / `"in"` / `"nin"` |
| `key` | string | 要比较的属性键 |
| `value` | string/number/boolean/array | 比较值 |

复合过滤器：

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"and"` 或 `"or"` |
| `filters` | array | 过滤器数组（可以是比较过滤器或复合过滤器） |

#### 2.4.5 computer_use_preview — 计算机使用工具

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
| `type` | string | 固定值 `"computer_use_preview"` |
| `display_width` | integer | 显示器宽度 |
| `display_height` | integer | 显示器高度 |
| `environment` | string | 计算机环境类型 |

#### 2.4.6 code_interpreter — 代码解释器工具

```json
{
  "type": "code_interpreter",
  "container": {
    "type": "auto",
    "file_ids": ["file-abc123"],
    "memory_limit": "512m"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"code_interpreter"` |
| `container` | string / object | 代码解释器容器 ID 或配置对象 |

**container 配置对象：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | `"auto"` |
| `file_ids` | array | 上传文件 ID 列表，供代码使用 |
| `memory_limit` | string | 容器内存限制 |

#### 2.4.7 image_generation — 图像生成工具

```json
{
  "type": "image_generation",
  "size": "1024x1024",
  "quality": "high",
  "output_format": "png",
  "output_compression": 100,
  "partial_images": 0,
  "background": "auto",
  "input_image_mask": {
    "image_url": "base64-data-url"
  }
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `type` | string | — | 固定值 `"image_generation"` |
| `size` | string | `"auto"` | 图像尺寸：`"1024x1024"` / `"1024x1536"` / `"1536x1024"` / `"auto"` |
| `quality` | string | `"auto"` | 图像质量：`"low"` / `"medium"` / `"high"` / `"auto"` |
| `output_format` | string | `"png"` | 输出格式：`"png"` / `"webp"` / `"jpeg"` |
| `output_compression` | integer | 100 | 压缩级别 |
| `partial_images` | integer | 0 | 流式模式下生成的部分图像数量（0-3） |
| `background` | string | `"auto"` | 背景类型：`"transparent"` / `"opaque"` / `"auto"` |
| `moderation` | string | `"auto"` | 内容审核级别 |
| `input_image_mask` | object | — | 编辑模式下的蒙版（含 `image_url` 和 `file_id`） |
| `style` | string | — | 仅 `gpt-image-1` 及以上版本支持，`"high"` / `"low"` |

#### 2.4.8 mcp — MCP 工具

```json
{
  "type": "mcp",
  "server_label": "deepwiki",
  "server_url": "https://mcp.deepwiki.com/mcp",
  "server_description": "DeepWiki MCP Server",
  "allowed_tools": ["read_wiki", "search_wiki"],
  "require_approval": "never",
  "headers": {
    "Authorization": "Bearer token123"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"mcp"` |
| `server_label` | string | 是 | MCP 服务器标签，用于在工具调用中标识 |
| `server_url` | string | 条件 | MCP 服务器 URL。与 `connector_id` 二选一 |
| `connector_id` | string | 条件 | 服务连接器 ID。与 `server_url` 二选一 |
| `server_description` | string | 否 | MCP 服务器描述 |
| `allowed_tools` | array / object | 否 | 允许的工具名称列表或过滤对象 |
| `require_approval` | string / object | 否 | 需要审批的工具策略，默认 `"always"` |
| `headers` | object | 否 | 发送到 MCP 服务器的 HTTP 头 |
| `oauth_token` | string | 否 | OAuth 访问令牌 |

**支持的 connector_id 值：**

| 连接器 | connector_id |
|--------|-------------|
| Dropbox | `connector_dropbox` |
| Gmail | `connector_gmail` |
| Google Calendar | `connector_googlecalendar` |
| Google Drive | `connector_googledrive` |
| Microsoft Teams | `connector_microsoftteams` |
| Outlook Calendar | `connector_outlookcalendar` |
| Outlook Email | `connector_outlookemail` |
| SharePoint | `connector_sharepoint` |

**allowed_tools 过滤对象：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 工具过滤器类型 |
| `tool_names` | array | 允许的工具名称列表 |
| `readOnlyHint` | boolean | 匹配只读工具 |

**require_approval 选项：**

- `"always"`：所有工具都需要审批
- `"never"`：所有工具都不需要审批
- 过滤对象：指定哪些工具需要审批

#### 2.4.9 shell — Shell 工具

```json
{
  "type": "shell"
}
```

允许模型在托管环境中执行 Shell 命令。

#### 2.4.10 local_shell — 本地 Shell 工具

```json
{
  "type": "local_shell"
}
```

允许模型在本地环境中执行 Shell 命令。

#### 2.4.11 custom — 自定义工具

```json
{
  "type": "custom",
  "name": "my_tool",
  "description": "My custom tool description",
  "input_format": {
    "type": "text"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"custom"` |
| `name` | string | 自定义工具名称 |
| `description` | string | 工具描述 |
| `input_format` | object | 输入格式。`{"type": "text"}` 或 `{"type": "grammar", "syntax": "lark"|"regex", "definition": "..."}` |

#### 2.4.12 apply_patch — 应用补丁工具

```json
{
  "type": "apply_patch"
}
```

允许使用 unified diff 创建、删除或更新文件。

### 2.5 工具选择（Tool Choice）

`tool_choice` 参数控制模型如何选择工具：

| 值 | 类型 | 说明 |
|----|------|------|
| `"auto"` | string | 模型自动选择生成消息或调用工具 |
| `"required"` | string | 模型必须调用一个或多个工具 |
| `"none"` | string | 模型不会调用任何工具，只生成消息 |

**特定工具选择（对象形式）：**

| 格式 | 说明 |
|------|------|
| `{"type": "function", "name": "get_weather"}` | 强制调用指定函数 |
| `{"type": "mcp", "server_label": "srv", "name": "tool"}` | 强制调用指定 MCP 工具 |
| `{"type": "custom", "name": "my_tool"}` | 强制调用指定自定义工具 |
| `{"type": "apply_patch"}` | 强制调用 apply_patch 工具 |
| `{"type": "shell"}` | 强制调用 shell 工具 |

**allowed_tools 约束（对象形式）：**

```json
{
  "type": "allowed_tools",
  "tools": [
    {"type": "file_search"},
    {"type": "web_search_preview"}
  ],
  "mode": "auto"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 固定值 `"allowed_tools"` |
| `tools` | array | 允许的内置工具定义列表 |
| `mode` | string | `"auto"` 允许生成消息或调用工具，`"required"` 必须调用工具 |

允许的内置工具类型：`file_search`、`web_search_preview`、`computer_use_preview`、`code_interpreter`、`image_generation`

### 2.6 推理配置（Reasoning）

```json
{
  "reasoning": {
    "effort": "high",
    "summary": "auto"
  }
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `effort` | string | `"medium"` | 推理力度约束 |
| `summary` | string | — | 推理摘要设置 |

**effort 支持的值：**

| 值 | 说明 |
|----|------|
| `none` | 不执行推理（gpt-5.1 默认） |
| `minimal` | 最小推理 |
| `low` | 低推理 |
| `medium` | 中等推理（gpt-5.1 之前模型的默认值） |
| `high` | 高推理（gpt-5-pro 的默认且唯一支持的值） |
| `xhigh` | 超高推理（gpt-5.1-codex-max 之后模型支持） |

**summary 支持的值：**

| 值 | 说明 |
|----|------|
| `"auto"` | 自动生成推理摘要 |
| `"concise"` | 简洁摘要（支持 computer-use-preview 和 gpt-5 之后模型） |
| `"detailed"` | 详细摘要 |

> 注意：`generate_summary` 参数已弃用，请使用 `summary` 替代。

### 2.7 文本输出格式（Text Format）

通过 `text.format` 参数控制输出格式：

```json
{
  "text": {
    "format": {
      "type": "json_schema",
      "name": "my_schema",
      "schema": {...},
      "strict": true
    },
    "verbosity": "medium"
  }
}
```

**格式类型：**

| 类型 | 格式对象 | 说明 |
|------|----------|------|
| `text` | `{"type": "text"}` | 默认文本格式（默认值） |
| `json_schema` | 见下方 | 结构化 JSON 输出（推荐） |
| `json_object` | `{"type": "json_object"}` | 旧版 JSON 模式（不推荐用于 gpt-4o 及更新模型） |

**json_schema 格式对象：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | 固定值 `"json_schema"` |
| `name` | string | 是 | 格式名称（a-z, A-Z, 0-9, _-，最长 64 字符） |
| `schema` | object | 是 | JSON Schema 对象 |
| `description` | string | 否 | 格式描述 |
| `strict` | boolean | 否 | 是否启用严格模式，默认 `false` |

**verbosity（详细程度）：**

| 值 | 说明 |
|----|------|
| `"low"` | 更简洁 |
| `"medium"` | 中等（默认） |
| `"high"` | 更详细 |

### 2.8 上下文管理（Context Management）

```json
{
  "context_management": {
    "type": "compaction",
    "compact_threshold": 80000
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string | 上下文管理条目类型，目前仅支持 `"compaction"` |
| `compact_threshold` | integer | 触发压缩的 Token 阈值 |

### 2.9 对话管理（Conversation）

```json
{
  "conversation": {
    "id": "conv_abc123"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 对话的唯一 ID |

对话对象中的项会预置到此响应请求的 `input_items` 前面。此响应完成后，输入项和输出项会自动添加到此对话中。

> 注意：`conversation` 不能与 `previous_response_id` 同时使用。

### 2.10 流式选项（Stream Options）

```json
{
  "stream": true,
  "stream_options": {
    "include_obfuscation": true
  }
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `include_obfuscation` | boolean | — | 是否启用流混淆。混淆在流式 delta 事件的 `obfuscation` 字段中添加随机字符，标准化载荷大小以缓解侧信道攻击 |

### 2.11 其他请求参数

**prompt — 提示模板引用：**

```json
{
  "prompt": {
    "id": "pmpt_abc123",
    "version": "1",
    "variables": {
      "topic": "AI safety",
      "image": {"type": "input_image", "image_url": "..."}
    }
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 提示模板 ID |
| `version` | string | 模板版本（可选） |
| `variables` | object | 变量映射，值可以是字符串或其他 Response 输入类型 |

**service_tier — 服务等级：**

| 值 | 说明 |
|----|------|
| `"auto"` | 使用项目设置中配置的服务等级（默认） |
| `"default"` | 标准定价和性能 |
| `"flex"` | Flex 服务等级 |
| `"priority"` | Priority 服务等级 |

**truncation — 截断策略：**

| 值 | 说明 |
|----|------|
| `"disabled"` | 如果输入超过上下文窗口大小，请求失败返回 400 错误（默认） |
| `"auto"` | 自动截断对话开头的内容以适应上下文窗口 |

---

## 3. 响应对象（Response Object）

### 3.1 完整响应结构

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 响应唯一标识符，格式 `resp_*` |
| `object` | string | 对象类型，固定值 `"response"` |
| `created_at` | integer | 创建时间（Unix 时间戳，秒） |
| `status` | string | 响应状态 |
| `completed_at` | integer / null | 完成时间（Unix 时间戳，秒），仅 `completed` 状态时存在 |
| `error` | object / null | 错误对象 |
| `incomplete_details` | object / null | 不完整详情 |
| `instructions` | string / null | 系统/开发者消息 |
| `max_output_tokens` | integer / null | 最大输出 Token 数 |
| `max_tool_calls` | integer / null | 最大工具调用次数 |
| `model` | string | 实际使用的模型 ID |
| `output` | array | 输出项数组 |
| `output_text` | string | （仅 SDK）聚合所有 `output_text` 项的文本内容 |
| `parallel_tool_calls` | boolean | 是否允许并行工具调用 |
| `previous_response_id` | string / null | 上一个响应 ID |
| `prompt` | object / null | 提示模板引用 |
| `prompt_cache_key` | string / null | 缓存键 |
| `prompt_cache_retention` | string / null | 缓存保留策略 |
| `reasoning` | object | 推理配置 |
| `safety_identifier` | string / null | 安全标识符 |
| `service_tier` | string / null | 实际使用的服务等级 |
| `store` | boolean | 是否已存储 |
| `temperature` | number / null | 使用的采样温度 |
| `text` | object | 文本格式配置 |
| `tool_choice` | string / object | 工具选择配置 |
| `tools` | array | 可用工具列表 |
| `top_p` | number / null | 使用的 top_p |
| `truncation` | string | 截断策略 |
| `usage` | object / null | Token 使用量统计 |
| `user` | string / null | 用户标识（已弃用） |
| `metadata` | object | 元数据 |
| `background` | boolean / null | 是否后台运行 |
| `conversation` | object / null | 对话对象 |

### 3.2 响应示例

**基本文本响应：**

```json
{
  "id": "resp_67ccd2bed1ec8190b14f964abc0542670bb6a6b452d3795b",
  "object": "response",
  "created_at": 1741476542,
  "status": "completed",
  "completed_at": 1741476543,
  "error": null,
  "incomplete_details": null,
  "instructions": null,
  "max_output_tokens": null,
  "model": "gpt-4.1-2025-04-14",
  "output": [
    {
      "type": "message",
      "id": "msg_67ccd2bf17f0819081ff3bb2cf6508e60bb6a6b452d3795b",
      "status": "completed",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "In a peaceful grove beneath a silver moon, a unicorn named Lumina discovered a hidden pool that reflected the stars.",
          "annotations": []
        }
      ]
    }
  ],
  "parallel_tool_calls": true,
  "previous_response_id": null,
  "reasoning": {
    "effort": null,
    "summary": null
  },
  "store": true,
  "temperature": 1.0,
  "text": {
    "format": {
      "type": "text"
    }
  },
  "tool_choice": "auto",
  "tools": [],
  "top_p": 1.0,
  "truncation": "disabled",
  "usage": {
    "input_tokens": 36,
    "input_tokens_details": {
      "cached_tokens": 0
    },
    "output_tokens": 87,
    "output_tokens_details": {
      "reasoning_tokens": 0
    },
    "total_tokens": 123
  },
  "user": null,
  "metadata": {}
}
```

### 3.3 状态值（Status）

| 状态 | 说明 |
|------|------|
| `completed` | 响应已完成 |
| `failed` | 响应生成失败 |
| `in_progress` | 响应正在生成中 |
| `cancelled` | 响应已被取消 |
| `queued` | 响应排队中（后台模式） |
| `incomplete` | 响应不完整 |

### 3.4 Usage 统计

```json
{
  "usage": {
    "input_tokens": 36,
    "input_tokens_details": {
      "cached_tokens": 0
    },
    "output_tokens": 87,
    "output_tokens_details": {
      "reasoning_tokens": 0
    },
    "total_tokens": 123
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `input_tokens` | integer | 输入 Token 数 |
| `input_tokens_details` | object | 输入 Token 明细 |
| `input_tokens_details.cached_tokens` | integer | 缓存命中的 Token 数 |
| `output_tokens` | integer | 输出 Token 数 |
| `output_tokens_details` | object | 输出 Token 明细 |
| `output_tokens_details.reasoning_tokens` | integer | 推理 Token 数 |
| `total_tokens` | integer | 总 Token 数 |

### 3.5 错误对象

```json
{
  "error": {
    "code": "rate_limit_exceeded",
    "message": "You have exceeded your rate limit."
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | string | 错误码 |
| `message` | string | 人类可读的错误描述 |

### 3.6 不完整详情

```json
{
  "incomplete_details": {
    "reason": "max_output_tokens"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `reason` | string | 不完整的原因，如 `"max_output_tokens"` |

---

## 4. 其他端点

### 4.1 获取响应 — GET /v1/responses/{response_id}

检索具有给定 ID 的模型响应。

**路径参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `response_id` | string | 要检索的响应 ID |

**查询参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `include` | array | 要包含的附加字段 |
| `include_obfuscation` | boolean | 是否启用流混淆 |
| `starting_after` | integer | 从指定序号后开始流式传输 |

**响应：** 返回完整的 Response 对象。

**请求示例：**

```bash
curl https://api.openai.com/v1/responses/resp_123 \
    -H "Authorization: Bearer $OPENAI_API_KEY"
```

### 4.2 删除响应 — DELETE /v1/responses/{response_id}

删除具有给定 ID 的模型响应。

**路径参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `response_id` | string | 要删除的响应 ID |

**响应：**

```json
{
  "id": "resp_6786a1bec27481909a17d673315b29f6",
  "object": "response",
  "deleted": true
}
```

### 4.3 取消响应 — POST /v1/responses/{response_id}/cancel

取消具有给定 ID 的模型响应。仅 `background` 参数设置为 `true` 创建的响应可以取消。

**路径参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `response_id` | string | 要取消的响应 ID |

**响应：** 返回 Response 对象，`status` 字段为 `"cancelled"`。注意取消的响应 `completed_at` 为 `null`，`usage` 为 `null`。

**请求示例：**

```bash
curl -X POST https://api.openai.com/v1/responses/resp_123/cancel \
    -H "Authorization: Bearer $OPENAI_API_KEY"
```

### 4.4 压缩对话 — POST /v1/responses/compact

对对话执行压缩处理。压缩返回加密的不透明项，底层逻辑可能随时间演变。

**请求体：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | 是 | 模型 ID |
| `input` | string / array | 是 | 输入项 |
| `instructions` | string | 否 | 系统消息 |
| `previous_response_id` | string | 否 | 上一个响应 ID |

**响应 — 压缩响应对象（response.compaction）：**

```json
{
  "id": "resp_001",
  "object": "response.compaction",
  "created_at": 1764967971,
  "output": [
    {
      "id": "msg_000",
      "type": "message",
      "status": "completed",
      "content": [
        {
          "type": "input_text",
          "text": "Create a simple landing page for a dog petting cafe."
        }
      ],
      "role": "user"
    },
    {
      "id": "cmp_001",
      "type": "compaction",
      "encrypted_content": "gAAAAABpM0Yj-...="
    }
  ],
  "usage": {
    "input_tokens": 139,
    "input_tokens_details": {
      "cached_tokens": 0
    },
    "output_tokens": 438,
    "output_tokens_details": {
      "reasoning_tokens": 64
    },
    "total_tokens": 577
  }
}
```

**压缩响应对象字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 压缩响应唯一标识符 |
| `object` | string | 固定值 `"response.compaction"` |
| `created_at` | integer | 创建时间戳 |
| `output` | array | 压缩后的输出项列表（包含原始消息和 compaction 项） |
| `usage` | object | Token 使用量 |

**使用方式：** 将 `compactedResponse.output` 作为下一次请求的 `input` 传入。

**请求示例：**

```bash
curl -X POST https://api.openai.com/v1/responses/compact \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $OPENAI_API_KEY" \
    -d '{
      "model": "gpt-5.1-codex-max",
      "input": [
        {"role": "user", "content": "Create a landing page."},
        {"id": "msg_001", "type": "message", "status": "completed",
         "content": [{"type": "output_text", "text": "...", "annotations": []}],
         "role": "assistant"}
      ]
    }'
```

### 4.5 列出输入项 — GET /v1/responses/{response_id}/input_items

返回给定响应的输入项列表。

**路径参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `response_id` | string | 响应 ID |

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `after` | string | — | 分页游标，列出此 ID 之后的项 |
| `include` | array | — | 要包含的附加字段 |
| `limit` | integer | 20 | 返回对象数量限制（1-100） |
| `order` | string | `"desc"` | 排序顺序：`"asc"` / `"desc"` |

**响应 — 输入项列表对象：**

```json
{
  "object": "list",
  "data": [
    {
      "id": "msg_abc123",
      "type": "message",
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "Tell me a three sentence bedtime story about a unicorn."
        }
      ]
    }
  ],
  "first_id": "msg_abc123",
  "last_id": "msg_abc123",
  "has_more": false
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `object` | string | 固定值 `"list"` |
| `data` | array | 输入项数组 |
| `first_id` | string | 列表中第一项的 ID |
| `last_id` | string | 列表中最后一项的 ID |
| `has_more` | boolean | 是否有更多项 |

### 4.6 计算输入 Token — POST /v1/responses/input_tokens

返回请求的输入 Token 数量，不实际生成响应。

**请求体：** 与创建响应相同的参数子集。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | 是 | 模型 ID |
| `input` | string / array | 是 | 输入项 |
| `instructions` | string | 否 | 系统消息 |
| `tools` | array | 否 | 工具定义 |
| `tool_choice` | string / object | 否 | 工具选择 |
| `previous_response_id` | string | 否 | 上一个响应 ID |
| `conversation` | object | 否 | 对话对象 |
| `parallel_tool_calls` | boolean | 否 | 是否并行工具调用 |
| `truncation` | string | 否 | 截断策略 |

**响应：**

```json
{
  "object": "response.input_tokens",
  "input_tokens": 11
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `object` | string | 固定值 `"response.input_tokens"` |
| `input_tokens` | integer | 输入 Token 数 |

**请求示例：**

```bash
curl -X POST https://api.openai.com/v1/responses/input_tokens \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $OPENAI_API_KEY" \
    -d '{
      "model": "gpt-4.1",
      "input": "Tell me a joke."
    }'
```

---

## 5. 与 Chat Completions API 的对比

| 特性 | Chat Completions API | Responses API |
|------|---------------------|---------------|
| 端点 | `POST /v1/chat/completions` | `POST /v1/responses` |
| 输入格式 | `messages` 数组 | `input` 字符串或数组 |
| 输出格式 | `choices` 数组 | `output` 数组 |
| 系统消息 | `messages` 中 `role: "system"` | `instructions` 参数 |
| 多轮对话 | 客户端维护完整消息历史 | `previous_response_id` 或 `conversation` 服务端管理 |
| 工具调用 | `tools` + `tool_choice` | `tools` + `tool_choice`（更多工具类型） |
| 内置工具 | 无 | Web 搜索、文件搜索、代码解释器等 |
| 流式响应 | SSE `data: {...}\n\n` + `data: [DONE]` | SSE 事件流 |
| 推理控制 | 无 | `reasoning.effort` + `reasoning.summary` |
| 结构化输出 | `response_format` | `text.format` |
| 后台模式 | 不支持 | `background: true` |
| 响应存储 | 不支持 | `store: true` + 响应检索 API |
| 上下文管理 | 不支持 | `context_management` + `compact` |
| MCP 集成 | 不支持 | `type: "mcp"` 工具 |
| 自定义工具 | 不支持 | `type: "custom"` 工具 |
| Shell 执行 | 不支持 | `type: "shell"` / `type: "local_shell"` |
| 文件操作 | 不支持 | `type: "apply_patch"` |
| 图像生成 | 不支持 | `type: "image_generation"` |
| 计算机控制 | 不支持 | `type: "computer_use_preview"` |
| Token 计数 | 不支持（需使用单独端点） | `POST /v1/responses/input_tokens` |

**关键转换映射（协议转换器参考）：**

| Chat Completions | Responses API |
|------------------|---------------|
| `messages[].role: "system"` | `instructions` |
| `messages[].role: "user"` + `content: "text"` | `{"type": "message", "role": "user", "content": "text"}` 或简写 `{"role": "user", "content": "text"}` |
| `messages[].role: "user"` + `content: [{"type": "text", ...}]` | `{"type": "message", "role": "user", "content": [{"type": "input_text", ...}]}` |
| `messages[].role: "user"` + `content: [{"type": "image_url", ...}]` | `{"type": "message", "role": "user", "content": [{"type": "input_image", ...}]}` |
| `messages[].role: "assistant"` | `{"type": "message", "role": "assistant", ...}` |
| `messages[].role: "tool"` | `{"type": "function_call_output", ...}` |
| `choices[0].message.content` | `output[].content[].text`（type 为 `output_text`） |
| `choices[0].message.tool_calls` | `output[]`（type 为 `function_call`） |
| `finish_reason: "stop"` | `status: "completed"` |
| `finish_reason: "tool_calls"` | `output` 中包含 `function_call` 项 |
| `usage.prompt_tokens` | `usage.input_tokens` |
| `usage.completion_tokens` | `usage.output_tokens` |
| `response_format` | `text.format` |
