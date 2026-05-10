# Google Gemini API 协议文档

> 基于官方 API Reference (https://ai.google.dev/api/generate-content) 整理
>
> 本文档用于 team-api 协议转换模块的开发参考，涵盖 Gemini GenerateContent API 的完整请求/响应规范、流式传输、上下文缓存、向量嵌入、Token 计数等。

---

## 目录

- [1. API 概览](#1-api-概览)
- [2. 认证方式](#2-认证方式)
- [3. 端点列表](#3-端点列表)
- [4. 请求体参数 (Request Body)](#4-请求体参数-request-body)
- [5. Content 结构](#5-content-结构)
- [6. Part 类型详解](#6-part-类型详解)
- [7. GenerationConfig 生成配置](#7-generationconfig-生成配置)
- [8. ThinkingConfig 思考配置](#8-thinkingconfig-思考配置)
- [9. SpeechConfig 语音配置](#9-speechconfig-语音配置)
- [10. Tool 工具系统](#10-tool-工具系统)
- [11. ToolConfig 工具配置](#11-toolconfig-工具配置)
- [12. SafetySettings 安全设置](#12-safetysettings-安全设置)
- [13. 响应格式 (Response)](#13-响应格式-response)
- [14. FinishReason 枚举](#14-finishreason-枚举)
- [15. 流式响应 (SSE Streaming)](#15-流式响应-sse-streaming)
- [16. 错误格式](#16-错误格式)
- [17. 上下文缓存 (Context Caching)](#17-上下文缓存-context-caching)
- [18. 向量嵌入 (Embeddings)](#18-向量嵌入-embeddings)
- [19. Token 计数 API](#19-token-计数-api)
- [20. 结构化输出 (Structured Output)](#20-结构化输出-structured-output)
- [21. 服务层级 (Service Tier)](#21-服务层级-service-tier)
- [22. 模型阶段 (Model Stage)](#22-模型阶段-model-stage)
- [23. 协议转换注意事项](#23-协议转换注意事项)

---

## 1. API 概览

| 属性 | 说明 |
|------|------|
| **协议类型** | REST (HTTP/JSON) |
| **基础 URL** | `https://generativelanguage.googleapis.com` |
| **API 版本** | `v1beta`（最新功能）/ `v1`（稳定版） |
| **核心功能** | 根据文本、图片、音频、视频等输入生成模型响应 |
| **Content-Type** | `application/json` |
| **特色能力** | 原生多模态输入/输出、思考模式(Thinking)、代码执行、搜索集成、MCP Server 集成、上下文缓存 |

### 基本请求示例

```bash
curl "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=$GEMINI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [
          {"text": "Hello, world!"}
        ]
      }
    ]
  }'
```

### 基本响应示例

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {"text": "Hello! How can I help you today?"}
        ],
        "role": "model"
      },
      "finishReason": "STOP",
      "index": 0
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 7,
    "candidatesTokenCount": 9,
    "totalTokenCount": 16
  },
  "modelVersion": "gemini-2.5-flash-001"
}
```

---

## 2. 认证方式

Gemini API 支持以下认证方式：

| 认证方式 | 说明 | 适用场景 |
|---------|------|---------|
| API Key (查询参数) | `?key=YOUR_API_KEY` | 开发调试、简单调用 |
| API Key (请求头) | `x-goog-api-key: YOUR_API_KEY` | 生产环境推荐 |
| OAuth 2.0 | `Authorization: Bearer YOUR_ACCESS_TOKEN` | Google Cloud 项目集成 |
| Service Account | 通过 Google Cloud IAM 认证 | 服务端应用 |

### 请求头示例

```http
# 方式一：请求头传递 API Key（推荐）
x-goog-api-key: AIzaSy...

# 方式二：查询参数传递 API Key
GET /v1beta/models/gemini-2.5-flash:generateContent?key=AIzaSy...

# 方式三：OAuth 2.0 Bearer Token
Authorization: Bearer ya29.a0AfH6...
```

---

## 3. 端点列表

### 3.1 内容生成端点

| 方法 | 路径 | 功能 | 流式支持 |
|------|------|------|---------|
| POST | `/v1beta/models/{model}:generateContent` | 同步生成内容 | 否 |
| POST | `/v1beta/models/{model}:streamGenerateContent?alt=sse` | 流式生成内容 | 是 (SSE) |
| POST | `/v1beta/models/{model}:countTokens` | 计算 Token 数量 | 否 |
| POST | `/v1beta/models/{model}:countTextTokens` | 计算文本 Token 数量 | 否 |
| POST | `/v1beta/models/{model}:computeTokens` | 计算 Tokens（通用） | 否 |
| POST | `/v1beta/models/{model}:embedContent` | 单条文本向量化 | 否 |
| POST | `/v1beta/models/{model}:batchEmbedContents` | 批量文本向量化 | 否 |

### 3.2 模型管理端点

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/v1beta/models` | 列出可用模型 |
| GET | `/v1beta/models/{model}` | 获取模型详情 |

### 3.3 上下文缓存端点

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/v1beta/cachedContents` | 创建缓存内容 |
| GET | `/v1beta/cachedContents/{name}` | 获取缓存内容 |
| PATCH | `/v1beta/cachedContents/{name}` | 更新缓存内容 |
| DELETE | `/v1beta/cachedContents/{name}` | 删除缓存内容 |
| GET | `/v1beta/cachedContents` | 列出缓存内容 |

### 3.4 URL 格式

```
# 同步生成
POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent

# 流式生成（注意 alt=sse 参数）
POST https://generativelanguage.googleapis.com/v1beta/models/{model}:streamGenerateContent?alt=sse

# 使用缓存
POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent
# 请求体中通过 cachedContent 字段引用缓存
```

---

## 4. 请求体参数 (Request Body)

### 4.1 完整参数一览

请求体是一个 JSON 对象，`GenerateContentRequest` 结构：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `contents` | Content[] | **是** | — | 对话内容列表，按时间顺序排列 |
| `tools` | Tool[] | 否 | — | 工具声明列表，如 Function Calling、Google Search 等 |
| `toolConfig` | ToolConfig | 否 | — | 工具配置，控制工具行为 |
| `safetySettings` | SafetySetting[] | 否 | — | 安全过滤设置 |
| `systemInstruction` | Content | 否 | — | 系统指令（开发者提示词） |
| `generationConfig` | GenerationConfig | 否 | — | 生成参数配置 |
| `cachedContent` | string | 否 | — | 引用已缓存内容的资源名称，如 `cachedContents/xxx` |

### 4.2 请求体示例

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {"text": "请分析这张图片"},
        {"inlineData": {"mimeType": "image/jpeg", "data": "base64EncodedImage..."}}
      ]
    },
    {
      "role": "model",
      "parts": [
        {"text": "这是一张风景照片..."}
      ]
    },
    {
      "role": "user",
      "parts": [
        {"text": "能更详细描述吗？"}
      ]
    }
  ],
  "systemInstruction": {
    "parts": [
      {"text": "你是一个专业的图像分析师。"}
    ]
  },
  "generationConfig": {
    "temperature": 0.7,
    "topP": 0.95,
    "topK": 40,
    "maxOutputTokens": 8192,
    "thinkingConfig": {
      "includeThoughts": true,
      "thinkingBudget": 8192
    }
  },
  "tools": [
    {"googleSearch": {}}
  ],
  "safetySettings": [
    {
      "category": "HARM_CATEGORY_HARASSMENT",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    }
  ]
}
```

---

## 5. Content 结构

Content 是 Gemini 消息的基本单元，与 OpenAI 的 message 和 Claude 的 content block 对应。

### 5.1 Content 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `parts` | Part[] | 内容部件列表，按顺序排列。每个 Part 可以是文本、图片、函数调用等 |
| `role` | string | 消息角色：`"user"`（用户）、`"model"`（模型） |

### 5.2 与其他协议的角色映射

| Gemini role | OpenAI role | Claude role | 说明 |
|-------------|-------------|-------------|------|
| `user` | `user` | `user` | 用户消息 |
| `model` | `assistant` | `assistant` | 模型回复 |
| — | `system` | — | Gemini 使用独立的 `systemInstruction` 字段 |
| `user`（function 回复） | `tool` | `user`（tool_result） | 函数执行结果 |

### 5.3 system 消息处理差异

Gemini **不使用** `contents` 数组中的 `role: "system"`，而是通过请求体顶层独立的 `systemInstruction` 字段传递系统提示词：

```json
{
  "systemInstruction": {
    "parts": [
      {"text": "你是一个有用的AI助手。"}
    ]
  },
  "contents": [
    {"role": "user", "parts": [{"text": "你好"}]}
  ]
}
```

---

## 6. Part 类型详解

Part 是 Gemini 内容的最小单元，支持多种类型。每个 Part 对象是一个**联合类型（union type）**，同一时间只应设置一个类型字段。

### 6.1 Part 类型总览

| 字段名 | 类型 | 说明 |
|--------|------|------|
| `text` | string | 纯文本内容 |
| `inlineData` | Blob | 内联的二进制数据（图片、音频、视频等） |
| `functionCall` | FunctionCall | 模型发起的函数调用 |
| `functionResponse` | FunctionResponse | 返回给模型的函数执行结果 |
| `fileData` | FileData | 引用已上传文件的引用 |
| `executableCode` | ExecutableCode | 可执行代码（代码执行工具生成） |
| `codeExecutionResult` | CodeExecutionResult | 代码执行结果 |
| `videoMetadata` | VideoMetadata | 视频元数据（配合 inlineData 或 fileData 使用） |
| `thought` | string | 思考内容（Thinking 模式） |
| `thoughtSignature` | string | 思考签名（用于验证思考过程完整性） |
| `toolCall` | ToolCall | 服务端工具调用（如 Google Search、代码执行） |
| `toolResponse` | ToolResponse | 服务端工具响应 |

### 6.2 文本 Part

```json
{
  "text": "这是一段文本内容"
}
```

### 6.3 内联数据 Part (inlineData / Blob)

用于发送图片、音频、视频等二进制数据，需 Base64 编码。

```json
{
  "inlineData": {
    "mimeType": "image/jpeg",
    "data": "/9j/4AAQSkZJRgABAQ..."
  }
}
```

**Blob 对象字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `mimeType` | string | **是** | MIME 类型 |
| `data` | string | **是** | Base64 编码的数据 |

**支持的 MIME 类型**：

| 类别 | MIME 类型示例 |
|------|-------------|
| 图片 | `image/jpeg`, `image/png`, `image/webp`, `image/gif`, `image/bmp`, `image/tiff` |
| 音频 | `audio/wav`, `audio/mp3`, `audio/aiff`, `audio/aac`, `audio/ogg`, `audio/flac` |
| 视频 | `video/mp4`, `video/mpeg`, `video/webm`, `video/x-matroska`, `video/x-flv` |
| 文档 | `application/pdf` |
| 文本 | `text/plain`, `text/html`, `text/css`, `text/csv`, `text/xml` |
| 代码 | `text/x-python`, `text/x-java`, `text/x-c`, `text/x-go`, `text/x-javascript` |

### 6.4 文件引用 Part (fileData)

引用通过 File API 上传的文件或 Google Cloud Storage URI。

```json
{
  "fileData": {
    "mimeType": "video/mp4",
    "fileUri": "gs://bucket-name/file.mp4"
  }
}
```

**FileData 对象字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `mimeType` | string | 否 | 文件 MIME 类型 |
| `fileUri` | string | **是** | 文件 URI，支持 `gs://` 格式或 File API 返回的 URI |

### 6.5 视频元数据 Part (videoMetadata)

配合 inlineData 或 fileData 使用，提供视频特定信息。

```json
{
  "videoMetadata": {
    "startOffset": "10s",
    "endOffset": "60s"
  }
}
```

**VideoMetadata 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `startOffset` | string (Duration) | 视频起始偏移量，格式如 `"10s"` |
| `endOffset` | string (Duration) | 视频结束偏移量，格式如 `"60s"` |

### 6.6 函数调用 Part (functionCall)

模型发起的函数调用请求。

```json
{
  "functionCall": {
    "name": "get_weather",
    "args": {
      "location": "Beijing",
      "unit": "celsius"
    }
  }
}
```

**FunctionCall 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 函数名称 |
| `args` | object | 函数参数（键值对） |

### 6.7 函数响应 Part (functionResponse)

返回函数执行结果给模型。

```json
{
  "functionResponse": {
    "name": "get_weather",
    "response": {
      "temperature": 22,
      "condition": "sunny",
      "unit": "celsius"
    }
  }
}
```

**FunctionResponse 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 函数名称（与 functionCall 的 name 对应） |
| `response` | object | 函数执行结果（任意 JSON 结构） |

### 6.8 可执行代码 Part (executableCode)

代码执行工具生成的可执行代码。

```json
{
  "executableCode": {
    "language": "PYTHON",
    "code": "print('Hello, World!')"
  }
}
```

**ExecutableCode 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `language` | string (enum) | 编程语言：`PYTHON` |
| `code` | string | 代码内容 |

### 6.9 代码执行结果 Part (codeExecutionResult)

代码执行的输出结果。

```json
{
  "codeExecutionResult": {
    "outcome": "OUTCOME_OK",
    "output": "Hello, World!"
  }
}
```

**CodeExecutionResult 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `outcome` | string (enum) | 执行结果状态：`OUTCOME_OK`、`OUTCOME_FAILED`、`OUTCOME_DEADLINE_EXCEEDED` |
| `output` | string | 执行输出 |

### 6.10 思考 Part (thought)

当启用 Thinking 模式（`thinkingConfig.includeThoughts: true`）时，模型内部推理过程。

```json
{
  "thought": "让我分析这个问题...首先需要考虑..."
}
```

### 6.11 思考签名 Part (thoughtSignature)

用于验证思考过程完整性的签名。某些模型在流式响应中会附带。

```json
{
  "thoughtSignature": "AQIDBA..."
}
```

### 6.12 服务端工具调用 Part (toolCall)

服务端工具（如 Google Search、代码执行）的调用信息。

```json
{
  "toolCall": {
    "toolType": "GOOGLE_SEARCH",
    "id": "tool-call-id-123"
  }
}
```

**ToolCall 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `toolType` | string (ToolType enum) | 工具类型 |
| `id` | string | 工具调用标识 |

### 6.13 服务端工具响应 Part (toolResponse)

服务端工具的执行结果。

```json
{
  "toolResponse": {
    "id": "tool-call-id-123",
    "result": "搜索结果内容..."
  }
}
```

**ToolResponse 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 与 toolCall 的 id 对应 |
| `result` | string | 工具执行结果 |

---

## 7. GenerationConfig 生成配置

GenerationConfig 控制模型的生成行为，是 Gemini 中最重要的配置对象之一。

### 7.1 完整字段一览

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `temperature` | number | 模型默认 | 采样温度。控制输出随机性，范围 0.0-2.0。值越低越确定，越高越随机 |
| `topP` | number | 模型默认 | 核采样（Nucleus Sampling）概率阈值。范围 0.0-1.0 |
| `topK` | integer | 模型默认 | Top-K 采样。仅从概率最高的 K 个 token 中采样 |
| `candidateCount` | integer | `1` | 生成的候选响应数量（目前仅支持 1） |
| `maxOutputTokens` | integer | 模型默认 | 最大输出 Token 数 |
| `stopSequences` | string[] | — | 停止序列列表，模型遇到任一序列时停止生成 |
| `responseMimeType` | string | `text/plain` | 响应 MIME 类型：`text/plain`、`application/json`、`text/x.enum` |
| `responseSchema` | Schema | — | 结构化输出的 JSON Schema（仅当 `responseMimeType` 为 `application/json` 时有效） |
| `responseJsonSchema` | JsonSchema | — | 增强的 JSON Schema 定义（含 ref 支持和严格模式） |
| `presencePenalty` | number | `0` | 存在惩罚。正值增加讨论新话题的概率，范围 -2.0 ~ 2.0 |
| `frequencyPenalty` | number | `0` | 频率惩罚。正值降低逐字重复的概率，范围 -2.0 ~ 2.0 |
| `seed` | integer | — | 随机种子。设置后可尽量保证相同输入产生相同输出 |
| `thinkingConfig` | ThinkingConfig | — | 思考模式配置（详见第 8 节） |
| `speechConfig` | SpeechConfig | — | 语音输出配置（详见第 9 节） |
| `audioConfig` | AudioConfig | — | 音频输出配置 |
| `imageConfig` | ImageConfig | — | 图片输出配置 |
| `mediaResolution` | string (enum) | — | 媒体分辨率：`MEDIA_RESOLUTION_LOW`、`MEDIA_RESOLUTION_MEDIUM`、`MEDIA_RESOLUTION_HIGH` |
| `routingConfig` | GenerationConfigRoutingConfig | — | 路由配置，控制请求在模型变体间的分发策略 |

### 7.2 与 OpenAI/Claude 参数映射

| Gemini 参数 | OpenAI Chat Completions | Claude Messages | 说明 |
|-------------|------------------------|----------------|------|
| `temperature` | `temperature` | `temperature` | 语义相同 |
| `topP` | `top_p` | `top_p` | 语义相同 |
| `topK` | — | `top_k` | OpenAI 不支持 |
| `maxOutputTokens` | `max_completion_tokens` | `max_tokens` | 语义相同 |
| `stopSequences` | `stop` | `stop_sequences` | 语义相同 |
| `candidateCount` | `n` | — | Gemini 目前仅支持 1 |
| `presencePenalty` | `presence_penalty` | — | Claude 不支持 |
| `frequencyPenalty` | `frequency_penalty` | — | Claude 不支持 |
| `seed` | `seed` | — | Claude 不支持 |
| `responseMimeType` | `response_format.type` | — | 结构化输出 |
| `responseSchema` | `response_format.json_schema` | — | Schema 定义 |
| `thinkingConfig` | — | `thinking.budget_tokens` | 思考模式 |

### 7.3 AudioConfig 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `audioConfig` | object | 音频输出配置（用于音频生成模型） |

### 7.4 ImageConfig 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `imageConfig` | object | 图片输出配置（用于图片生成模型） |

---

## 8. ThinkingConfig 思考配置

ThinkingConfig 控制模型的内部推理（思考）行为，是 Gemini 2.5 系列模型的核心特性。

### 8.1 ThinkingConfig 对象

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `includeThoughts` | boolean | 否 | `false` | 是否在响应中包含思考过程 |
| `thinkingBudget` | integer | 否 | 模型默认 | 思考 Token 预算，控制模型在思考上花费的最大 Token 数 |
| `thinkingLevel` | string (enum) | 否 | 模型默认 | 思考级别枚举 |

### 8.2 thinkingLevel 枚举值

| 值 | 说明 |
|----|------|
| `THINKING_LEVEL_UNSPECIFIED` | 未指定，使用模型默认 |
| `THINKING_LEVEL_LOW` | 低级别思考，快速但简略 |
| `THINKING_LEVEL_MEDIUM` | 中级别思考，平衡速度与深度 |
| `THINKING_LEVEL_HIGH` | 高级别思考，深度推理 |
| `THINKING_LEVEL_NONE` | 禁用思考 |

### 8.3 配置示例

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "includeThoughts": true,
      "thinkingBudget": 8192
    }
  }
}
```

### 8.4 思考模式响应示例

启用思考后，响应中会包含 `thought` 类型的 Part：

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {"thought": "让我分析这个问题...\n首先需要考虑数学公式..."},
          {"text": "答案是 42。"}
        ],
        "role": "model"
      }
    }
  ]
}
```

---

## 9. SpeechConfig 语音配置

用于配置文本转语音（TTS）输出的语音参数。

### 9.1 SpeechConfig 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `voiceConfig` | VoiceConfig | 单说话人语音配置 |
| `multiSpeakerVoiceConfig` | MultiSpeakerVoiceConfig | 多说话人语音配置 |
| `languageCode` | string | 语言代码，如 `"en-US"`、`"zh-CN"` |

### 9.2 VoiceConfig 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `prebuiltVoiceConfig` | PrebuiltVoiceConfig | 预置语音配置 |

### 9.3 PrebuiltVoiceConfig 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `voiceName` | string | 语音名称，如 `"Aoede"`、`"Charon"`、`"Puck"` 等 |

### 9.4 MultiSpeakerVoiceConfig 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `speakerVoiceConfigs` | SpeakerVoiceConfig[] | 各说话人的语音配置 |

### 9.5 配置示例

```json
{
  "generationConfig": {
    "speechConfig": {
      "voiceConfig": {
        "prebuiltVoiceConfig": {
          "voiceName": "Aoede"
        }
      },
      "languageCode": "zh-CN"
    }
  }
}
```

---

## 10. Tool 工具系统

Gemini 的 Tool 系统支持多种工具类型，包括自定义函数、Google 搜索、代码执行等。

### 10.1 Tool 对象

每个 Tool 是一个联合类型，可以包含以下任一类型：

| 字段 | 类型 | 说明 |
|------|------|------|
| `functionDeclarations` | FunctionDeclaration[] | 函数声明列表（Function Calling） |
| `googleSearch` | GoogleSearch | Google 搜索工具（即 Grounding） |
| `googleSearchRetrieval` | GoogleSearchRetrieval | Google 搜索检索（旧版，推荐使用 googleSearch） |
| `codeExecution` | CodeExecution | 代码执行工具 |
| `computerUse` | ComputerUse | 计算机操控工具 |
| `urlContext` | UrlContext | URL 上下文工具 |
| `fileSearch` | FileSearch | 文件搜索工具 |
| `mcpServers` | MCPServer[] | MCP（Model Context Protocol）服务端集成 |
| `googleMaps` | GoogleMaps | Google Maps 工具 |

### 10.2 FunctionDeclaration 函数声明

定义可供模型调用的函数。

```json
{
  "functionDeclarations": [
    {
      "name": "get_weather",
      "description": "获取指定城市的天气信息",
      "parameters": {
        "type": "OBJECT",
        "properties": {
          "location": {
            "type": "STRING",
            "description": "城市名称"
          },
          "unit": {
            "type": "STRING",
            "description": "温度单位",
            "enum": ["celsius", "fahrenheit"]
          }
        },
        "required": ["location"]
      }
    }
  ]
}
```

**FunctionDeclaration 字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | **是** | 函数名称，必须唯一 |
| `description` | string | **是** | 函数描述，模型据此决定何时调用 |
| `parameters` | Schema | 否 | 函数参数的 JSON Schema |
| `response` | Schema | 否 | 函数返回值的 Schema（部分模型支持） |

### 10.3 Schema 类型系统

Gemini 使用自定义 Schema 格式而非标准的 JSON Schema：

| 字段 | 类型 | 说明 |
|------|------|------|
| `type` | string (enum) | 类型枚举 |
| `format` | string | 格式说明（如 `"int64"`、`"double"`） |
| `description` | string | 字段描述 |
| `nullable` | boolean | 是否可为空 |
| `enum` | string[] | 枚举值列表 |
| `maxItems` | string (int64) | 数组最大元素数 |
| `minItems` | string (int64) | 数组最小元素数 |
| `properties` | map<string, Schema> | 对象属性 |
| `required` | string[] | 必填字段列表 |
| `items` | Schema | 数组元素类型 |
| `anyOf` | Schema[] | 联合类型 |

**type 枚举值**：

| 值 | 说明 |
|----|------|
| `TYPE_UNSPECIFIED` | 未指定 |
| `STRING` | 字符串 |
| `NUMBER` | 数字（浮点） |
| `INTEGER` | 整数 |
| `BOOLEAN` | 布尔值 |
| `ARRAY` | 数组 |
| `OBJECT` | 对象 |

### 10.4 GoogleSearch 工具

```json
{
  "tools": [
    {
      "googleSearch": {}
    }
  ]
}
```

GoogleSearch 目前为空对象，启用即表示使用默认配置。模型会自动在需要时触发搜索。

### 10.5 GoogleSearchRetrieval 工具（旧版）

```json
{
  "tools": [
    {
      "googleSearchRetrieval": {
        "disableAttribution": false
      }
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `disableAttribution` | boolean | 是否禁用搜索来源归因 |

### 10.6 CodeExecution 工具

```json
{
  "tools": [
    {
      "codeExecution": {}
    }
  ]
}
```

CodeExecution 目前为空对象。启用后模型会生成并执行代码。

### 10.7 ComputerUse 工具

```json
{
  "tools": [
    {
      "computerUse": {}
    }
  ]
}
```

用于计算机操控能力（类似 Claude 的 Computer Use）。

### 10.8 UrlContext 工具

```json
{
  "tools": [
    {
      "urlContext": {}
    }
  ]
}
```

允许模型获取和引用 URL 内容。

### 10.9 FileSearch 工具

```json
{
  "tools": [
    {
      "fileSearch": {}
    }
  ]
}
```

允许模型在用户上传的文件中进行搜索。

### 10.10 MCP Server 集成

```json
{
  "tools": [
    {
      "mcpServers": [
        {
          "name": "my-mcp-server",
          "streamableHttpTransport": {
            "url": "https://example.com/mcp"
          }
        }
      ]
    }
  ]
}
```

**MCPServer 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | MCP 服务器名称 |
| `streamableHttpTransport` | StreamableHttpTransport | HTTP 传输配置 |

**StreamableHttpTransport 对象字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `url` | string | MCP 服务器 URL |

### 10.11 ToolType 枚举

服务端工具的类型标识：

| 值 | 说明 |
|----|------|
| `TOOL_TYPE_UNSPECIFIED` | 未指定 |
| `GOOGLE_SEARCH` | Google 搜索 |
| `CODE_EXECUTION` | 代码执行 |
| `GOOGLE_MAPS` | Google Maps |

---

## 11. ToolConfig 工具配置

ToolConfig 控制工具的全局行为。

### 11.1 ToolConfig 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `functionCallingConfig` | FunctionCallingConfig | 函数调用配置 |
| `retrievalConfig` | RetrievalConfig | 检索配置（旧版搜索工具） |

### 11.2 FunctionCallingConfig 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `allowedFunctionNames` | string[] | 允许调用的函数名称列表。不设置则允许所有已声明的函数 |
| `mode` | string (enum) | 函数调用模式 |

**mode 枚举值**：

| 值 | 说明 |
|----|------|
| `AUTO` | 自动模式：模型自行决定是否调用函数（默认） |
| `ANY` | 任意模式：模型必须调用至少一个函数 |
| `NONE` | 禁用模式：模型不得调用任何函数 |
| `VALIDATED` | 验证模式：仅调用已验证可用的函数 |

### 11.3 配置示例

```json
{
  "toolConfig": {
    "functionCallingConfig": {
      "allowedFunctionNames": ["get_weather", "get_news"],
      "mode": "AUTO"
    }
  }
}
```

---

## 12. SafetySettings 安全设置

控制模型的内容安全过滤行为。

### 12.1 SafetySetting 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `category` | string (HarmCategory enum) | 安全类别 |
| `threshold` | string (HarmBlockThreshold enum) | 阻断阈值 |
| `method` | string (HarmBlockMethod enum) | 阻断方法 |

### 12.2 HarmCategory 安全类别

| 枚举值 | 说明 |
|--------|------|
| `HARM_CATEGORY_UNSPECIFIED` | 未指定 |
| `HARM_CATEGORY_HARASSMENT` | 骚扰 |
| `HARM_CATEGORY_HATE_SPEECH` | 仇恨言论 |
| `HARM_CATEGORY_SEXUALLY_EXPLICIT` | 色情内容 |
| `HARM_CATEGORY_DANGEROUS_CONTENT` | 危险内容 |
| `HARM_CATEGORY_CIVIC_INTEGRITY` | 公民诚信（选举虚假信息等） |

### 12.3 HarmBlockThreshold 阻断阈值

| 枚举值 | 说明 |
|--------|------|
| `HARM_BLOCK_THRESHOLD_UNSPECIFIED` | 未指定，使用默认设置 |
| `BLOCK_LOW_AND_ABOVE` | 阻断低风险及更高级别的内容 |
| `BLOCK_MEDIUM_AND_ABOVE` | 阻断中等风险及更高级别的内容（推荐默认值） |
| `BLOCK_ONLY_HIGH` | 仅阻断高风险内容 |
| `BLOCK_NONE` | 不阻断任何内容（仍会评估风险） |
| `OFF` | 关闭安全过滤（不评估也不阻断） |

### 12.4 HarmBlockMethod 阻断方法

| 枚举值 | 说明 |
|--------|------|
| `HARM_BLOCK_METHOD_UNSPECIFIED` | 未指定 |
| `SEVERITY` | 基于严重程度 |
| `PROBABILITY` | 基于概率 |

### 12.5 配置示例

```json
{
  "safetySettings": [
    {
      "category": "HARM_CATEGORY_HARASSMENT",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    },
    {
      "category": "HARM_CATEGORY_HATE_SPEECH",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    },
    {
      "category": "HARM_CATEGORY_SEXUALLY_EXPLICIT",
      "threshold": "BLOCK_ONLY_HIGH"
    },
    {
      "category": "HARM_CATEGORY_DANGEROUS_CONTENT",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    },
    {
      "category": "HARM_CATEGORY_CIVIC_INTEGRITY",
      "threshold": "BLOCK_NONE"
    }
  ]
}
```

---

## 13. 响应格式 (Response)

### 13.1 GenerateContentResponse 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `candidates` | Candidate[] | 候选响应列表 |
| `promptFeedback` | PromptFeedback | 输入提示的反馈信息 |
| `usageMetadata` | UsageMetadata | Token 使用量统计 |
| `modelVersion` | string | 模型版本标识，如 `"gemini-2.5-flash-001"` |
| `responseId` | string | 响应唯一标识 |

### 13.2 Candidate 候选对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `content` | Content | 响应内容（含 parts 和 role） |
| `finishReason` | string (FinishReason enum) | 结束原因 |
| `index` | integer | 候选索引（从 0 开始） |
| `citationMetadata` | CitationMetadata | 引用元数据 |
| `groundingMetadata` | GroundingMetadata | 搜索接地元数据 |
| `groundingAttributions` | GroundingAttribution[] | 搜索接地归因 |
| `tokenCount` | integer | Token 计数 |

### 13.3 CitationMetadata 引用元数据

| 字段 | 类型 | 说明 |
|------|------|------|
| `citationSources` | CitationSource[] | 引用来源列表 |

**CitationSource 对象**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `startIndex` | integer | 引用开始的字符索引 |
| `endIndex` | integer | 引用结束的字符索引 |
| `uri` | string | 引用来源 URI |
| `license` | string | 许可证信息 |

### 13.4 GroundingMetadata 搜索接地元数据

| 字段 | 类型 | 说明 |
|------|------|------|
| `groundingChunks` | GroundingChunk[] | 接地内容块 |
| `groundingSupports` | GroundingSupport[] | 接地支持信息 |
| `searchEntryPoint` | SearchEntryPoint | 搜索入口 |
| `retrievalMetadata` | RetrievalMetadata | 检索元数据 |
| `webSearchQueries` | string[] | Web 搜索查询列表 |

### 13.5 PromptFeedback 提示反馈

| 字段 | 类型 | 说明 |
|------|------|------|
| `blockReason` | string (BlockReason enum) | 阻断原因 |
| `safetyRatings` | SafetyRating[] | 安全评级列表 |

**BlockReason 枚举值**：

| 值 | 说明 |
|----|------|
| `BLOCK_REASON_UNSPECIFIED` | 未指定 |
| `SAFETY` | 因安全过滤被阻断 |
| `OTHER` | 其他原因 |
| `BLOCKLIST` | 因黑名单被阻断 |
| `PROHIBITED_CONTENT` | 禁止内容 |
| `MODEL_REJECT` | 模型拒绝 |

**SafetyRating 对象**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `category` | string (HarmCategory) | 安全类别 |
| `probability` | string | 概率级别：`NEGLIGIBLE`、`LOW`、`MEDIUM`、`HIGH` |
| `blocked` | boolean | 是否被阻断 |

### 13.6 UsageMetadata 使用量统计

| 字段 | 类型 | 说明 |
|------|------|------|
| `promptTokenCount` | integer | 输入 Token 数 |
| `candidatesTokenCount` | integer | 输出 Token 数 |
| `totalTokenCount` | integer | 总 Token 数 |
| `cachedContentTokenCount` | integer | 缓存命中 Token 数（使用上下文缓存时） |
| `thoughtsTokenCount` | integer | 思考 Token 数（启用思考模式时） |

### 13.7 完整响应示例

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "北京今天的天气是晴天，气温 22°C。"
          }
        ],
        "role": "model"
      },
      "finishReason": "STOP",
      "index": 0,
      "groundingMetadata": {
        "webSearchQueries": ["北京今天天气"],
        "groundingChunks": [
          {
            "web": {
              "uri": "https://example.com/weather/beijing",
              "title": "北京天气预报"
            }
          }
        ]
      }
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 15,
    "candidatesTokenCount": 20,
    "totalTokenCount": 35,
    "candidatesTokensDetails": [
      {
        "modality": "TEXT",
        "tokenCount": 20
      }
    ]
  },
  "modelVersion": "gemini-2.5-flash-001"
}
```

---

## 14. FinishReason 枚举

模型停止生成的原因，位于 Candidate 对象中。

| 枚举值 | 说明 | 对应 OpenAI finish_reason | 对应 Claude stop_reason |
|--------|------|--------------------------|------------------------|
| `FINISH_REASON_UNSPECIFIED` | 未指定 | — | — |
| `STOP` | 自然停止（遇到 EOS 或完成回复） | `stop` | `end_turn` |
| `MAX_TOKENS` | 达到最大输出 Token 限制 | `length` | `max_tokens` |
| `SAFETY` | 因安全过滤被阻断 | `content_filter` | — |
| `RECITATION` | 因引用/版权限制被阻断 | — | — |
| `LANGUAGE` | 因语言限制被阻断 | — | — |
| `OTHER` | 其他原因 | — | — |
| `BLOCKLIST` | 因黑名单被阻断 | — | — |
| `PROHIBITED_CONTENT` | 因禁止内容被阻断 | — | — |
| `SPII` | 因敏感个人身份信息被阻断 | — | — |
| `MALFORMED_FUNCTION_CALL` | 函数调用格式错误 | — | — |
| `IMAGE_SAFETY` | 因图片安全被阻断 | — | — |
| `UNEXPECTED_TOOL_CALL` | 非预期的工具调用 | — | — |
| `FAIRNESS_CIVIL_INTEGRITY` | 因公平性/公民诚信被阻断 | — | — |
| `CONTEXT_WINDOW_EXCEEDED` | 超出上下文窗口 | — | — |
| `CANDIDATE_SUMMARY` | 候选摘要（内部使用） | — | — |
| `TOOL_CALL_LIMIT_REACHED` | 达到工具调用次数上限 | — | — |
| `MISSING_THOUGHT_SIGNATURE` | 缺少思考签名 | — | — |
| `MALFORMED_RESPONSE` | 响应格式错误 | — | — |

---

## 15. 流式响应 (SSE Streaming)

### 15.1 流式端点

流式生成使用 `streamGenerateContent` 端点，**必须**添加 `alt=sse` 查询参数：

```
POST /v1beta/models/{model}:streamGenerateContent?alt=sse
```

### 15.2 SSE 格式特点

Gemini 的流式格式与 OpenAI/Claude 有显著差异：

| 特性 | Gemini | OpenAI | Claude |
|------|--------|--------|--------|
| 数据格式 | `data: {json}\n\n` | `data: {json}\n\n` | `event: xxx\ndata: {json}\n\n` |
| 结束标记 | **无** `[DONE]`，HTTP 连接关闭即结束 | `data: [DONE]` | `event: message_stop` |
| event 字段 | **不使用** | 不使用 | 使用 |
| 完整响应 | 每个 chunk 都是完整的 GenerateContentResponse | chunk 中只有增量 (delta) | 分事件类型 |

### 15.3 流式响应示例

```
data: {"candidates":[{"content":{"parts":[{"text":"Hello"}],"role":"model"},"index":0}],"usageMetadata":{"promptTokenCount":7,"totalTokenCount":7},"modelVersion":"gemini-2.5-flash-001"}

data: {"candidates":[{"content":{"parts":[{"text":"! How"}],"role":"model"},"index":0}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":2,"totalTokenCount":9},"modelVersion":"gemini-2.5-flash-001"}

data: {"candidates":[{"content":{"parts":[{"text":" can I help"}],"role":"model"},"index":0}],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":4,"totalTokenCount":11},"modelVersion":"gemini-2.5-flash-001"}

data: {"candidates":[{"content":{"parts":[{"text":" you today?"}],"role":"model"},"index":0},"finishReason":"STOP"],"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":7,"totalTokenCount":14},"modelVersion":"gemini-2.5-flash-001"}
```

### 15.4 流式响应解析注意事项

1. **每个 chunk 都是完整的 GenerateContentResponse**：不需要像 OpenAI 那样拼接 delta，但需要合并多个 chunk 中的 text Part 来重建完整响应
2. **无 `[DONE]` 标记**：流结束由 HTTP 连接关闭信号决定
3. **finishReason 出现在最后一个 chunk**：只有最后一个 chunk 的 candidate 会包含 `finishReason`
4. **usageMetadata 逐步累加**：每个 chunk 的 `usageMetadata` 反映截至当前的累计 Token 用量

### 15.5 流式 Function Calling 示例

```
data: {"candidates":[{"content":{"parts":[{"functionCall":{"name":"get_weather","args":{"location":"Beijing"}}}],"role":"model"},"index":0}],"usageMetadata":{"promptTokenCount":50,"totalTokenCount":50},"modelVersion":"gemini-2.5-flash-001"}

data: {"candidates":[{"content":{"parts":[]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":50,"candidatesTokenCount":10,"totalTokenCount":60},"modelVersion":"gemini-2.5-flash-001"}
```

### 15.6 流式思考模式示例

```
data: {"candidates":[{"content":{"parts":[{"thought":"让我分析这个问题..."}],"role":"model"},"index":0}],"usageMetadata":{"promptTokenCount":100,"totalTokenCount":100},"modelVersion":"gemini-2.5-flash-001"}

data: {"candidates":[{"content":{"parts":[{"thought":"首先需要考虑数学公式..."}],"role":"model"},"index":0}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":15,"totalTokenCount":115},"modelVersion":"gemini-2.5-flash-001"}

data: {"candidates":[{"content":{"parts":[{"text":"答案是 42。"}],"role":"model"},"index":0},"finishReason":"STOP"],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":20,"thoughtsTokenCount":15,"totalTokenCount":135},"modelVersion":"gemini-2.5-flash-001"}
```

---

## 16. 错误格式

### 16.1 错误响应结构

Gemini 使用 Google RPC 风格的错误格式：

```json
{
  "error": {
    "code": 400,
    "message": "Invalid JSON payload received. Unknown name \"models\": Cannot find field.",
    "status": "INVALID_ARGUMENT"
  }
}
```

### 16.2 错误字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `error.code` | integer | HTTP 状态码 |
| `error.message` | string | 错误描述信息 |
| `error.status` | string | Google RPC 状态码 |
| `error.details` | object[] | 错误详情（部分错误包含） |

### 16.3 常见 HTTP 状态码与 RPC 状态码映射

| HTTP 状态码 | RPC 状态码 | 说明 |
|------------|-----------|------|
| 400 | `INVALID_ARGUMENT` | 请求参数错误（字段缺失、格式不对、枚举值无效等） |
| 401 | `UNAUTHENTICATED` | 认证失败（API Key 无效或过期） |
| 403 | `PERMISSION_DENIED` | 权限不足（API Key 无权访问该资源） |
| 404 | `NOT_FOUND` | 资源不存在（模型名称错误等） |
| 429 | `RESOURCE_EXHAUSTED` | 请求频率超限或配额耗尽 |
| 429 | `RATE_LIMIT_EXCEEDED` | 速率限制 |
| 500 | `INTERNAL` | 服务器内部错误 |
| 503 | `UNAVAILABLE` | 服务暂时不可用 |
| 504 | `DEADLINE_EXCEEDED` | 请求超时 |

### 16.4 错误示例

**认证失败**：

```json
{
  "error": {
    "code": 401,
    "message": "API key not valid. Please pass a valid API key.",
    "status": "UNAUTHENTICATED"
  }
}
```

**模型不存在**：

```json
{
  "error": {
    "code": 404,
    "message": "Model `gemini-nonexistent` not found.",
    "status": "NOT_FOUND"
  }
}
```

**请求频率超限**：

```json
{
  "error": {
    "code": 429,
    "message": "Quota exceeded for quota metric 'Generate Requests Per Minute' and limit 'GenerateRequestsPerMinutePerProjectPerRegion' of service 'generativelanguage.googleapis.com'.",
    "status": "RESOURCE_EXHAUSTED"
  }
}
```

**安全过滤**：

```json
{
  "candidates": [
    {
      "content": {
        "parts": [],
        "role": "model"
      },
      "finishReason": "SAFETY",
      "index": 0
    }
  ],
  "promptFeedback": {
    "blockReason": "SAFETY",
    "safetyRatings": [
      {
        "category": "HARM_CATEGORY_DANGEROUS_CONTENT",
        "probability": "HIGH",
        "blocked": true
      }
    ]
  }
}
```

### 16.5 与其他协议的错误格式对比

| 特性 | Gemini | OpenAI | Claude |
|------|--------|--------|--------|
| 顶层字段 | `error` | `error` | 无顶层 error |
| 错误码字段 | `error.code` (HTTP) | `error.type` (字符串) | `error.type` |
| 消息字段 | `error.message` | `error.message` | `error.message` |
| 状态标识 | `error.status` (RPC) | `error.code` (字符串) | — |
| HTTP 状态码 | 是 | 是 | 是 |
| 包装格式 | `{"error": {...}}` | `{"error": {...}}` | `{"type": "error", "error": {...}}` |

---

## 17. 上下文缓存 (Context Caching)

上下文缓存允许预存储大量上下文内容（如长文档、系统指令），在后续请求中引用，避免重复处理，降低延迟和费用。

### 17.1 CachedContent 对象

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 缓存资源名称（系统生成），格式：`cachedContents/{id}` |
| `model` | string | 模型名称，格式：`models/{model}` |
| `displayName` | string | 可选的显示名称 |
| `contents` | Content[] | 缓存的对话内容 |
| `systemInstruction` | Content | 缓存的系统指令 |
| `tools` | Tool[] | 缓存的工具声明 |
| `toolConfig` | ToolConfig | 缓存的工具配置 |
| `ttl` | string (Duration) | 缓存存活时间，格式如 `"3600s"`（1小时） |
| `expireTime` | string (Timestamp) | 过期时间（RFC 3339 格式） |
| `createTime` | string (Timestamp) | 创建时间（系统生成） |
| `updateTime` | string (Timestamp) | 更新时间（系统生成） |
| `usageMetadata` | CachedContentUsageMetadata | 使用量元数据 |

### 17.2 创建缓存

```
POST /v1beta/cachedContents
```

```json
{
  "model": "models/gemini-2.5-flash",
  "contents": [
    {
      "role": "user",
      "parts": [
        {"text": "以下是一份非常长的技术文档内容...（省略数万字）"}
      ]
    }
  ],
  "systemInstruction": {
    "parts": [
      {"text": "你是一个专业技术文档分析助手。请基于提供的文档内容回答问题。"}
    ]
  },
  "ttl": "3600s"
}
```

**创建响应**：

```json
{
  "name": "cachedContents/abc123def456",
  "model": "models/gemini-2.5-flash",
  "createTime": "2025-01-15T10:00:00.000Z",
  "updateTime": "2025-01-15T10:00:00.000Z",
  "expireTime": "2025-01-15T11:00:00.000Z",
  "usageMetadata": {
    "totalTokenCount": 50000
  }
}
```

### 17.3 使用缓存

在 generateContent 请求中通过 `cachedContent` 字段引用缓存：

```json
{
  "cachedContent": "cachedContents/abc123def456",
  "contents": [
    {
      "role": "user",
      "parts": [
        {"text": "请总结文档的第三章"}
      ]
    }
  ]
}
```

**注意**：使用缓存时，请求体中的 `contents` 是追加到缓存内容之后的新消息。缓存的 `systemInstruction`、`tools` 等也会被继承。

### 17.4 获取缓存

```
GET /v1beta/cachedContents/{name}
```

### 17.5 更新缓存

```
PATCH /v1beta/cachedContents/{name}
```

可更新字段：`ttl`、`expireTime`

```json
{
  "ttl": "7200s"
}
```

### 17.6 删除缓存

```
DELETE /v1beta/cachedContents/{name}
```

### 17.7 列出缓存

```
GET /v1beta/cachedContents
```

### 17.8 CachedContentUsageMetadata

| 字段 | 类型 | 说明 |
|------|------|------|
| `totalTokenCount` | integer | 缓存内容的总 Token 数 |

### 17.9 缓存费用优势

使用缓存的 Token 价格低于普通输入 Token 价格。响应中的 `usageMetadata.cachedContentTokenCount` 表示本次请求中从缓存命中的 Token 数。

---

## 18. 向量嵌入 (Embeddings)

### 18.1 单条嵌入

```
POST /v1beta/models/{model}:embedContent
```

**请求体**：

```json
{
  "model": "models/text-embedding-004",
  "content": {
    "parts": [
      {"text": "What is the meaning of life?"}
    ]
  },
  "taskType": "RETRIEVAL_QUERY",
  "title": "Life Question",
  "outputDimensionality": 256
}
```

**EmbedContentRequest 字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | **是** | 嵌入模型名称 |
| `content` | Content | **是** | 待嵌入的内容 |
| `taskType` | string (TaskType enum) | 否 | 任务类型 |
| `title` | string | 否 | 内容标题（仅 RETRIEVAL_DOCUMENT 类型有效） |
| `outputDimensionality` | integer | 否 | 输出向量维度 |

**TaskType 枚举**：

| 值 | 说明 |
|----|------|
| `RETRIEVAL_QUERY` | 检索查询 |
| `RETRIEVAL_DOCUMENT` | 检索文档 |
| `SEMANTIC_SIMILARITY` | 语义相似度 |
| `CLASSIFICATION` | 分类 |
| `CLUSTERING` | 聚类 |
| `QUESTION_ANSWERING` | 问答 |
| `FACT_VERIFICATION` | 事实验证 |
| `CODE_RETRIEVAL_QUERY` | 代码检索查询 |

**响应**：

```json
{
  "embedding": {
    "values": [0.013168513, -0.008721383, 0.043494854, ...]
  }
}
```

### 18.2 批量嵌入

```
POST /v1beta/models/{model}:batchEmbedContents
```

**请求体**：

```json
{
  "requests": [
    {
      "model": "models/text-embedding-004",
      "content": {
        "parts": [{"text": "Hello world"}]
      }
    },
    {
      "model": "models/text-embedding-004",
      "content": {
        "parts": [{"text": "Goodbye world"}]
      }
    }
  ]
}
```

**响应**：

```json
{
  "embeddings": [
    {"values": [0.013168513, -0.008721383, ...]},
    {"values": [0.005394286, -0.011432873, ...]}
  ]
}
```

### 18.3 可用嵌入模型

| 模型 | 最大输入 Token | 输出维度 | 说明 |
|------|---------------|---------|------|
| `text-embedding-004` | 2,048 | 768（可降维到 256） | 推荐通用模型 |
| `embedding-001` | 2,048 | 768 | 旧版模型 |

---

## 19. Token 计数 API

### 19.1 计数端点

```
POST /v1beta/models/{model}:countTokens
```

### 19.2 请求体

请求体格式与 GenerateContentRequest 相同，支持 `contents`、`systemInstruction`、`tools` 等字段：

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {"text": "Hello, how are you?"}
      ]
    }
  ],
  "systemInstruction": {
    "parts": [
      {"text": "You are a helpful assistant."}
    ]
  },
  "tools": [
    {
      "functionDeclarations": [
        {
          "name": "get_weather",
          "description": "Get weather",
          "parameters": {
            "type": "OBJECT",
            "properties": {
              "location": {"type": "STRING"}
            }
          }
        }
      ]
    }
  ]
}
```

### 19.3 响应

```json
{
  "totalTokens": 42
}
```

**CountTokensResponse 字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `totalTokens` | integer | 输入内容的总 Token 数 |
| `cachedContentTokenCount` | integer | 缓存命中的 Token 数（引用缓存时） |

---

## 20. 结构化输出 (Structured Output)

### 20.1 通过 responseMimeType 指定

最简单的方式是设置 `responseMimeType` 为 `application/json`：

```json
{
  "generationConfig": {
    "responseMimeType": "application/json"
  }
}
```

模型会输出 JSON 格式的响应，但不保证特定的 Schema。

### 20.2 通过 responseSchema 指定

配合 `responseMimeType: "application/json"` 使用 `responseSchema` 定义输出结构：

```json
{
  "generationConfig": {
    "responseMimeType": "application/json",
    "responseSchema": {
      "type": "OBJECT",
      "properties": {
        "name": {"type": "STRING", "description": "人物姓名"},
        "age": {"type": "INTEGER", "description": "年龄"},
        "hobbies": {
          "type": "ARRAY",
          "items": {"type": "STRING"}
        }
      },
      "required": ["name", "age"]
    }
  }
}
```

### 20.3 通过 responseJsonSchema 指定

增强版 Schema 定义，支持 `$ref` 引用和严格模式：

```json
{
  "generationConfig": {
    "responseMimeType": "application/json",
    "responseJsonSchema": {
      "schema": {
        "type": "OBJECT",
        "properties": {
          "name": {"type": "STRING"},
          "address": {
            "$ref": "#/$defs/Address"
          }
        },
        "$defs": {
          "Address": {
            "type": "OBJECT",
            "properties": {
              "city": {"type": "STRING"},
              "country": {"type": "STRING"}
            }
          }
        }
      }
    }
  }
}
```

### 20.4 与 OpenAI Structured Output 对比

| 特性 | Gemini | OpenAI |
|------|--------|--------|
| 方式一 | `responseMimeType: "application/json"` | `response_format: {"type": "json_object"}` |
| 方式二 | `responseMimeType` + `responseSchema` | `response_format: {"type": "json_schema", "json_schema": {...}}` |
| Schema 格式 | Gemini 自定义 Schema 格式 | 标准 JSON Schema |
| 枚举类型 | `type: "STRING"` + `enum: [...]` | `"type": "string"` + `"enum": [...]` |

---

## 21. 服务层级 (Service Tier)

Gemini API 支持不同的服务层级，影响请求处理优先级和速率限制。

### 21.1 服务层级类型

| 层级 | 说明 |
|------|------|
| `SERVICE_TIER_UNSPECIFIED` | 未指定，使用默认 |
| `STANDARD` | 标准层级，共享资源池 |
| `FLEX` | 弹性层级，延迟可能较高但成本更低 |
| `PRIORITY` | 优先层级，更高的速率限制和更低的延迟 |

### 21.2 配置方式

通过 `generationConfig.routingConfig` 指定：

```json
{
  "generationConfig": {
    "routingConfig": {
      "autoMode": "AUTO",
      "manualMode": "PRIORITY"
    }
  }
}
```

---

## 22. 模型阶段 (Model Stage)

Google 模型有不同的生命周期阶段。

### 22.1 阶段枚举

| 阶段 | 说明 |
|------|------|
| `UNSTABLE_EXPERIMENTAL` | 不稳定的实验阶段，API 可能随时变更 |
| `STABLE_EXPERIMENTAL` | 稳定的实验阶段，API 相对稳定 |
| `PREVIEW` | 预览阶段，功能基本确定 |
| `GA` | 正式发布 (General Availability)，推荐生产使用 |
| `RETIRED` | 已退役，不再可用 |

### 22.2 主流模型列表

| 模型 | 说明 |
|------|------|
| `gemini-2.5-flash` | Gemini 2.5 Flash（推荐，支持 Thinking） |
| `gemini-2.5-pro` | Gemini 2.5 Pro（最强推理能力） |
| `gemini-2.0-flash` | Gemini 2.0 Flash |
| `gemini-2.0-flash-lite` | Gemini 2.0 Flash Lite（轻量版） |
| `gemini-1.5-pro` | Gemini 1.5 Pro（支持超长上下文） |
| `gemini-1.5-flash` | Gemini 1.5 Flash |
| `text-embedding-004` | 文本嵌入模型 |

---

## 23. 协议转换注意事项

### 23.1 消息格式转换

| 转换方向 | 关键差异 |
|---------|---------|
| **OpenAI → Gemini** | `messages` → `contents`；`role: "system"` → `systemInstruction`；`role: "assistant"` → `role: "model"`；`role: "tool"` → `role: "user"` + `functionResponse` |
| **Claude → Gemini** | `messages` → `contents`；`system` → `systemInstruction`；content block 数组 → parts 数组 |
| **Gemini → OpenAI** | `contents` → `messages`；`systemInstruction` → `role: "system"` message；`role: "model"` → `role: "assistant"`；`functionCall` → `tool_calls`；`functionResponse` → `role: "tool"` |
| **Gemini → Claude** | `contents` → `messages`；`systemInstruction` → `system`；parts 数组 → content blocks |

### 23.2 工具调用转换

**OpenAI Tool Calling → Gemini Function Calling**：

```
OpenAI:
  tools[].type = "function"
  tools[].function.name → functionDeclarations[].name
  tools[].function.description → functionDeclarations[].description
  tools[].function.parameters → functionDeclarations[].parameters（需转换 Schema 格式）

  响应：
  message.tool_calls[].id → 需自行生成映射
  message.tool_calls[].function.name → functionCall.name
  message.tool_calls[].function.arguments (JSON string) → functionCall.args (JSON object)

  工具结果：
  role: "tool", tool_call_id → role: "user", functionResponse.name
  content (string) → functionResponse.response (JSON object)
```

### 23.3 Schema 格式转换

Gemini 的 Schema 格式与标准 JSON Schema 有差异：

| 标准 JSON Schema | Gemini Schema | 说明 |
|-----------------|---------------|------|
| `"type": "string"` | `"type": "STRING"` | 类型名全大写 |
| `"type": "number"` | `"type": "NUMBER"` | — |
| `"type": "integer"` | `"type": "INTEGER"` | — |
| `"type": "boolean"` | `"type": "BOOLEAN"` | — |
| `"type": "array"` | `"type": "ARRAY"` | — |
| `"type": "object"` | `"type": "OBJECT"` | — |
| `"enum": [...]` | `"enum": [...]` | 相同 |
| 不支持 `anyOf` | `"anyOf": [...]` | Gemini 扩展 |
| `"nullable": true` | `"nullable": true` | 相同 |

### 23.4 流式响应转换

**Gemini SSE → OpenAI SSE**：

1. 解析 Gemini 的每个 `data:` chunk 为 GenerateContentResponse
2. 提取 `candidates[0].content.parts` 中的文本
3. 转换为 OpenAI 的 delta 格式：
   ```json
   {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}
   ```
4. 最后一个 chunk（含 `finishReason`）添加 `finish_reason` 映射
5. 结束时发送 `data: [DONE]`（Gemini 原生无此标记，需转换层添加）

**Gemini SSE → Claude SSE**：

1. 解析 Gemini chunk
2. 首个 chunk 转换为 `event: message_start`
3. 后续文本 chunk 转换为 `event: content_block_delta` + `{"type":"text_delta","text":"..."}`
4. 思考 chunk 转换为 `event: content_block_delta` + `{"type":"thinking_delta","thinking":"..."}`
5. 最后一个 chunk 转换为 `event: message_delta` + `{"delta":{"stop_reason":"end_turn"}}`
6. 发送 `event: message_stop`

### 23.5 使用量统计转换

| Gemini | OpenAI | Claude |
|--------|--------|--------|
| `usageMetadata.promptTokenCount` | `usage.prompt_tokens` | `usage.input_tokens` |
| `usageMetadata.candidatesTokenCount` | `usage.completion_tokens` | `usage.output_tokens` |
| `usageMetadata.totalTokenCount` | `usage.total_tokens` | —（需计算） |
| `usageMetadata.cachedContentTokenCount` | —（OpenAI 无对应） | `usage.cache_read_input_tokens` |
| `usageMetadata.thoughtsTokenCount` | —（OpenAI 无对应） | `usage.cache_creation_input_tokens`（概念不同） |

### 23.6 错误格式转换

**Gemini → OpenAI 错误格式**：

```json
// Gemini 原始错误
{"error": {"code": 400, "message": "...", "status": "INVALID_ARGUMENT"}}

// 转换为 OpenAI 格式
{"error": {"type": "invalid_request_error", "message": "...", "param": null, "code": null}}
```

**Gemini → Claude 错误格式**：

```json
// 转换为 Claude 格式
{"type": "error", "error": {"type": "invalid_request_error", "message": "..."}}
```

**错误类型映射**：

| Gemini error.status | OpenAI error.type | Claude error.type |
|--------------------|--------------------|--------------------|
| `UNAUTHENTICATED` | `authentication_error` | `authentication_error` |
| `PERMISSION_DENIED` | `permission_error` | `permission_error` |
| `INVALID_ARGUMENT` | `invalid_request_error` | `invalid_request_error` |
| `RESOURCE_EXHAUSTED` | `rate_limit_error` | `rate_limit_error` |
| `INTERNAL` | `internal_error` | `api_error` |
| `UNAVAILABLE` | `server_error` | `api_error` |
| `NOT_FOUND` | `invalid_request_error` | `not_found_error` |
