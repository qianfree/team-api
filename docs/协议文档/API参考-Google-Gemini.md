# Google Gemini API 完整参考文档

> 基于 Google AI for Developers 官方文档整理（最后更新: 2026-04-19）
> 本文档用于 team-api 网关的协议转换模块开发参考

---

## 目录

1. [API 端点概览](#1-api-端点概览)
2. [generateContent 请求体](#2-generatecontent-请求体)
3. [Content 与 Part 类型](#3-content-与-part-类型)
4. [GenerationConfig 完整字段](#4-generationconfig-完整字段)
5. [Tool 系统](#5-tool-系统)
6. [ThinkingConfig](#6-thinkingconfig)
7. [SafetySettings](#7-safetysettings)
8. [GenerateContentResponse 响应格式](#8-generatecontentresponse-响应格式)
9. [流式响应格式 (streamGenerateContent)](#9-流式响应格式-streamgeneratecontent)
10. [Context Caching API](#10-context-caching-api)
11. [File API](#11-file-api)
12. [Token 计数端点](#12-token-计数端点)
13. [Embeddings 端点](#13-embeddings-端点)
14. [OpenAI 兼容端点](#14-openai-兼容端点)
15. [错误格式](#15-错误格式)
16. [可用模型列表](#16-可用模型列表)
17. [枚举值速查表](#17-枚举值速查表)

---

## 1. API 端点概览

### 核心生成端点

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `https://generativelanguage.googleapis.com/v1beta/{model=models/*}:generateContent` | 非流式生成 |
| POST | `https://generativelanguage.googleapis.com/v1beta/{model=models/*}:streamGenerateContent` | 流式生成（SSE） |

### Token 计数端点

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `https://generativelanguage.googleapis.com/v1beta/{model=models/*}:countTokens` | Token 计数 |

### Embeddings 端点

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `https://generativelanguage.googleapis.com/v1beta/{model=models/*}:embedContent` | 单条嵌入 |
| POST | `https://generativelanguage.googleapis.com/v1beta/{model=models/*}:batchEmbedContents` | 批量嵌入 |
| POST | `https://generativelanguage.googleapis.com/v1beta/{batch.model=models/*}:asyncBatchEmbedContent` | 异步批量嵌入 |

### Context Caching 端点

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `https://generativelanguage.googleapis.com/v1beta/cachedContents` | 创建缓存 |
| GET | `https://generativelanguage.googleapis.com/v1beta/cachedContents` | 列出缓存 |
| GET | `https://generativelanguage.googleapis.com/v1beta/{name=cachedContents/*}` | 获取缓存 |
| PATCH | `https://generativelanguage.googleapis.com/v1beta/{cachedContent.name=cachedContents/*}` | 更新缓存（仅过期时间） |
| DELETE | `https://generativelanguage.googleapis.com/v1beta/{name=cachedContents/*}` | 删除缓存 |

### File API 端点

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `https://generativelanguage.googleapis.com/upload/v1beta/files` | 上传文件（resumable） |
| POST | `https://generativelanguage.googleapis.com/v1beta/files` | 上传文件（metadata only） |
| GET | `https://generativelanguage.googleapis.com/v1beta/{name=files/*}` | 获取文件信息 |
| GET | `https://generativelanguage.googleapis.com/v1beta/files` | 列出文件 |
| DELETE | `https://generativelanguage.googleapis.com/v1beta/{name=files/*}` | 删除文件 |
| POST | `https://generativelanguage.googleapis.com/v1beta/files:register` | 注册 GCS 文件 |

### OpenAI 兼容端点

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `https://generativelanguage.googleapis.com/v1beta/openai/chat/completions` | Chat Completions |
| POST | `https://generativelanguage.googleapis.com/v1beta/openai/images/generations` | 图像生成 |
| POST | `https://generativelanguage.googleapis.com/v1beta/openai/videos` | 视频生成 |
| GET | `https://generativelanguage.googleapis.com/v1beta/openai/videos/{id}` | 查询视频状态 |
| POST | `https://generativelanguage.googleapis.com/v1beta/openai/embeddings` | 嵌入 |
| GET | `https://generativelanguage.googleapis.com/v1beta/openai/models` | 模型列表 |
| GET | `https://generativelanguage.googleapis.com/v1beta/openai/models/{model}` | 模型详情 |

### Stable v1 端点

v1 端点使用稳定模型，格式为 `https://generativelanguage.googleapis.com/v1/...`，其余路径结构与 v1beta 相同。

---

## 2. generateContent 请求体

```json
{
  "contents": [{ "role": "user", "parts": [{ "text": "Hello" }] }],
  "tools": [],
  "toolConfig": {},
  "safetySettings": [],
  "systemInstruction": { "role": "user", "parts": [{ "text": "You are a helpful assistant." }] },
  "generationConfig": {},
  "cachedContent": "cachedContents/{id}",
  "serviceTier": "STANDARD",
  "store": false
}
```

### 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `contents[]` | `object(Content)` | **是** | 对话内容数组。单轮查询为单个实例，多轮对话为完整历史 |
| `tools[]` | `object(Tool)` | 否 | 可用工具列表（函数声明、代码执行、搜索等） |
| `toolConfig` | `object(ToolConfig)` | 否 | 工具配置 |
| `safetySettings[]` | `object(SafetySetting)` | 否 | 安全过滤设置，每个类别最多一个 |
| `systemInstruction` | `object(Content)` | 否 | 系统指令（developer prompt） |
| `generationConfig` | `object(GenerationConfig)` | 否 | 生成参数配置 |
| `cachedContent` | `string` | 否 | 已缓存的上下文名称，格式: `cachedContents/{id}` |
| `serviceTier` | `enum(ServiceTier)` | 否 | 服务层级 |
| `store` | `boolean` | 否 | 配置请求的日志记录行为 |

---

## 3. Content 与 Part 类型

### Content 结构

```json
{
  "role": "user",
  "parts": [
    { "text": "Hello" },
    { "inlineData": { "mimeType": "image/png", "data": "base64..." } },
    { "functionCall": { "name": "get_weather", "args": { "city": "NYC" } } }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `role` | `string` | `"user"` 或 `"model"` |
| `parts[]` | `object(Part)` | 消息各部分，可混合多种 MIME 类型 |

### Part 类型（联合类型，只能包含以下之一）

#### text — 文本内容
```json
{ "text": "Hello, how are you?" }
```

#### inlineData (Blob) — 内联二进制媒体
```json
{
  "inlineData": {
    "mimeType": "image/png",
    "data": "iVBORw0KGgo..."  // base64 编码
  }
}
```

#### functionCall — 模型预测的函数调用
```json
{
  "functionCall": {
    "id": "call_abc123",       // 可选，唯一标识符
    "name": "get_weather",      // 必填，函数名
    "args": { "city": "NYC" }   // 可选，JSON 对象参数
  }
}
```

#### functionResponse — 函数调用结果
```json
{
  "functionResponse": {
    "id": "call_abc123",
    "name": "get_weather",
    "response": { "temperature": 72, "condition": "sunny" },
    "parts": [],
    "willContinue": false,
    "scheduling": "WHEN_IDLE"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `string` | 可选，匹配对应 functionCall 的 id |
| `name` | `string` | 必填，函数名 |
| `response` | `object(Struct)` | 必填，JSON 对象格式的函数响应 |
| `parts[]` | `FunctionResponsePart[]` | 可选，函数响应的多媒体部分 |
| `willContinue` | `boolean` | 仅 NON_BLOCKING 函数适用，表示后续还有更多响应 |
| `scheduling` | `enum(Scheduling)` | 调度方式：`SILENT`/`WHEN_IDLE`/`INTERRUPT` |

#### fileData — URI 引用的文件
```json
{
  "fileData": {
    "mimeType": "video/mp4",
    "fileUri": "https://generativelanguage.googleapis.com/v1beta/files/abc-123"
  }
}
```

#### executableCode — 模型生成的可执行代码
```json
{
  "executableCode": {
    "id": "code_1",
    "language": "PYTHON",
    "code": "print('Hello, World!')"
  }
}
```

#### codeExecutionResult — 代码执行结果
```json
{
  "codeExecutionResult": {
    "id": "code_1",
    "outcome": "OUTCOME_OK",
    "output": "Hello, World!\n"
  }
}
```

| outcome 值 | 说明 |
|------------|------|
| `OUTCOME_OK` | 执行成功，output 包含 stdout |
| `OUTCOME_FAILED` | 执行失败，output 包含 stderr 和 stdout |
| `OUTCOME_DEADLINE_EXCEEDED` | 超时被取消，可能有部分 output |

#### toolCall — 服务端工具调用
```json
{
  "toolCall": {
    "id": "tc_001",
    "toolType": "GOOGLE_SEARCH_WEB",
    "args": { "query": "latest AI news" }
  }
}
```

ToolType 枚举值：
- `TOOL_TYPE_UNSPECIFIED`
- `GOOGLE_SEARCH_WEB` — Google 网页搜索
- `GOOGLE_SEARCH_IMAGE` — Google 图片搜索
- `URL_CONTEXT` — URL 上下文检索
- `GOOGLE_MAPS` — Google 地图
- `FILE_SEARCH` — 文件搜索

#### toolResponse — 服务端工具响应
```json
{
  "toolResponse": {
    "id": "tc_001",
    "toolType": "GOOGLE_SEARCH_WEB",
    "response": { "results": [...] }
  }
}
```

### Part 的附加元数据字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `thought` | `boolean` | 是否为思考内容 |
| `thoughtSignature` | `string(bytes)` | 思考签名，用于后续请求复用（base64） |
| `partMetadata` | `object(Struct)` | 自定义元数据 |
| `mediaResolution` | `object(MediaResolution)` | 输入媒体的分辨率设置 |
| `videoMetadata` | `object(VideoMetadata)` | 视频元数据（startOffset, endOffset, fps） |

---

## 4. GenerationConfig 完整字段

```json
{
  "stopSequences": ["\n"],
  "responseMimeType": "application/json",
  "responseSchema": {},
  "responseJsonSchema": {},
  "responseModalities": ["TEXT"],
  "candidateCount": 1,
  "maxOutputTokens": 8192,
  "temperature": 0.7,
  "topP": 0.95,
  "topK": 40,
  "seed": 42,
  "presencePenalty": 0.0,
  "frequencyPenalty": 0.0,
  "responseLogprobs": false,
  "logprobs": 0,
  "enableEnhancedCivicAnswers": false,
  "speechConfig": {},
  "thinkingConfig": {},
  "imageConfig": {},
  "mediaResolution": "MEDIA_RESOLUTION_UNSPECIFIED"
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `stopSequences[]` | `string[]` | — | 最多 5 个停止序列 |
| `responseMimeType` | `string` | `"text/plain"` | 输出 MIME 类型：`text/plain` / `application/json` / `text/x.enum` |
| `responseSchema` | `object(Schema)` | — | OpenAPI 子集的输出 schema，需配合 `application/json` |
| `responseJsonSchema` | `Value` | — | JSON Schema 格式的输出 schema，与 responseSchema 互斥 |
| `responseModalities[]` | `enum(Modality)[]` | `["TEXT"]` | 请求的响应模态：`TEXT`/`IMAGE`/`AUDIO` |
| `candidateCount` | `integer` | `1` | 候选响应数量 |
| `maxOutputTokens` | `integer` | 因模型而异 | 最大输出 token 数 |
| `temperature` | `number` | 因模型而异 | 随机性控制，范围 [0.0, 2.0] |
| `topP` | `number` | 因模型而异 | 核采样概率阈值 |
| `topK` | `integer` | 因模型而异 | 最大候选 token 数 |
| `seed` | `integer` | 随机 | 解码种子 |
| `presencePenalty` | `number` | — | 存在惩罚（二值开关） |
| `frequencyPenalty` | `number` | — | 频率惩罚（按使用次数累加） |
| `responseLogprobs` | `boolean` | `false` | 是否返回 logprobs |
| `logprobs` | `integer` | `0` | 返回 top logprobs 数量，范围 [0, 20] |
| `enableEnhancedCivicAnswers` | `boolean` | `false` | 增强公民答案 |
| `speechConfig` | `object(SpeechConfig)` | — | 语音生成配置 |
| `thinkingConfig` | `object(ThinkingConfig)` | — | 思考功能配置 |
| `imageConfig` | `object(ImageConfig)` | — | 图像生成配置 |
| `mediaResolution` | `enum(MediaResolution)` | — | 输入媒体分辨率 |

### responseJsonSchema 支持的 JSON Schema 属性

`$id`, `$defs`, `$ref`, `$anchor`, `type`, `format`, `title`, `description`, `enum`, `items`, `prefixItems`, `minItems`, `maxItems`, `minimum`, `maximum`, `anyOf`, `oneOf`（等同于 anyOf）, `properties`, `additionalProperties`, `required`。还支持非标准的 `propertyOrdering`。

### SpeechConfig

```json
{
  "voiceConfig": {
    "prebuiltVoiceConfig": { "voiceName": "Aoede" }
  },
  "multiSpeakerVoiceConfig": {
    "speakerVoiceConfigs": [
      { "speaker": "Speaker1", "voiceConfig": { "prebuiltVoiceConfig": { "voiceName": "Aoede" } } }
    ]
  },
  "languageCode": "en-US"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `voiceConfig` | `object(VoiceConfig)` | 单声音配置 |
| `multiSpeakerVoiceConfig` | `object(MultiSpeakerVoiceConfig)` | 多说话人配置，与 voiceConfig 互斥 |
| `languageCode` | `string` | BCP-47 语言代码 |

支持的语言代码：`de-DE`, `en-AU`, `en-GB`, `en-IN`, `en-US`, `es-US`, `fr-FR`, `hi-IN`, `pt-BR`, `ar-XA`, `es-ES`, `fr-CA`, `id-ID`, `it-IT`, `ja-JP`, `tr-TR`, `vi-VN`, `bn-IN`, `gu-IN`, `kn-IN`, `ml-IN`, `mr-IN`, `ta-IN`, `te-IN`, `nl-NL`, `ko-KR`, `cmn-CN`, `pl-PL`, `ru-RU`, `th-TH`

### ImageConfig

```json
{
  "aspectRatio": "16:9",
  "imageSize": "1K"
}
```

| 字段 | 说明 |
|------|------|
| `aspectRatio` | 支持：`1:1`, `1:4`, `4:1`, `1:8`, `8:1`, `2:3`, `3:2`, `3:4`, `4:3`, `4:5`, `5:4`, `9:16`, `16:9`, `21:9` |
| `imageSize` | 支持：`512`, `1K`, `2K`, `4K`，默认 `1K` |

### MediaResolution 枚举

| 值 | 说明 |
|---|------|
| `MEDIA_RESOLUTION_UNSPECIFIED` | 未设置 |
| `MEDIA_RESOLUTION_LOW` | 低分辨率（64 tokens） |
| `MEDIA_RESOLUTION_MEDIUM` | 中分辨率（256 tokens） |
| `MEDIA_RESOLUTION_HIGH` | 高分辨率（zoomed reframing with 256 tokens） |

---

## 5. Tool 系统

### Tool 类型概览

```json
{
  "tools": [
    { "functionDeclarations": [...] },
    { "googleSearchRetrieval": { ... } },
    { "codeExecution": {} },
    { "googleSearch": { ... } },
    { "computerUse": { ... } },
    { "urlContext": {} },
    { "fileSearch": { ... } },
    { "mcpServers": [...] },
    { "googleMaps": { ... } }
  ]
}
```

每个 Tool 对象可包含以下字段之一（互斥）：

### 5.1 functionDeclarations — 自定义函数调用

```json
{
  "functionDeclarations": [
    {
      "name": "get_weather",
      "description": "Get the current weather in a given location",
      "behavior": "BLOCKING",
      "parameters": {
        "type": "OBJECT",
        "properties": {
          "location": { "type": "STRING", "description": "The city and state" },
          "unit": { "type": "STRING", "enum": ["celsius", "fahrenheit"] }
        },
        "required": ["location"]
      },
      "parametersJsonSchema": { ... },
      "response": { "type": "OBJECT", "properties": { ... } },
      "responseJsonSchema": { ... }
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | `string` | 必填，函数名，最长 128 字符 |
| `description` | `string` | 必填，函数描述 |
| `behavior` | `enum(Behavior)` | `BLOCKING`（默认）/ `NON_BLOCKING` |
| `parameters` | `object(Schema)` | OpenAPI 3.03 格式的参数定义 |
| `parametersJsonSchema` | `Value` | JSON Schema 格式的参数定义，与 parameters 互斥 |
| `response` | `object(Schema)` | 函数输出 schema |
| `responseJsonSchema` | `Value` | JSON Schema 格式的输出定义，与 response 互斥 |

### Schema 结构（OpenAPI 子集）

```json
{
  "type": "OBJECT",
  "format": "",
  "title": "",
  "description": "",
  "nullable": false,
  "enum": ["EAST", "NORTH"],
  "maxItems": "10",
  "minItems": "1",
  "properties": { ... },
  "required": ["name"],
  "minProperties": "0",
  "maxProperties": "10",
  "minLength": "1",
  "maxLength": "100",
  "pattern": "^[a-z]+$",
  "example": {},
  "anyOf": [ ... ],
  "propertyOrdering": ["name", "age"],
  "default": {},
  "items": { ... },
  "minimum": 0,
  "maximum": 100
}
```

Schema Type 枚举：`TYPE_UNSPECIFIED` / `STRING` / `NUMBER` / `INTEGER` / `BOOLEAN` / `ARRAY` / `OBJECT` / `NULL`

### 5.2 codeExecution — 代码执行工具

```json
{ "codeExecution": {} }
```

无额外字段。启用后模型自动生成 `ExecutableCode`（PYTHON >= 3.10）和 `CodeExecutionResult`。

### 5.3 googleSearch — Google 搜索工具（新版）

```json
{
  "googleSearch": {
    "timeRangeFilter": {
      "startTime": "2025-01-01T00:00:00Z",
      "endTime": "2025-12-31T23:59:59Z"
    },
    "searchTypes": {
      "webSearch": {},
      "imageSearch": {}
    }
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `timeRangeFilter` | `object(Interval)` | 时间范围过滤，start 和 end 必须同时设置 |
| `searchTypes` | `object(SearchTypes)` | 搜索类型：`webSearch` / `imageSearch`，不设置则默认 web 搜索 |

### 5.4 googleSearchRetrieval — Google 搜索检索（旧版）

```json
{
  "googleSearchRetrieval": {
    "dynamicRetrievalConfig": {
      "mode": "MODE_DYNAMIC",
      "dynamicThreshold": 0.3
    }
  }
}
```

| 字段 | 说明 |
|------|------|
| `mode` | `MODE_UNSPECIFIED`（始终触发）/ `MODE_DYNAMIC`（系统判断） |
| `dynamicThreshold` | 动态检索阈值 [0, 1] |

### 5.5 computerUse — 计算机使用工具

```json
{
  "computerUse": {
    "environment": "ENVIRONMENT_BROWSER",
    "excludedPredefinedFunctions": ["screenshot"]
  }
}
```

| 字段 | 说明 |
|------|------|
| `environment` | 目前仅支持 `ENVIRONMENT_BROWSER` |
| `excludedPredefinedFunctions[]` | 排除的预定义函数列表 |

### 5.6 urlContext — URL 上下文检索

```json
{ "urlContext": {} }
```

无额外字段。

### 5.7 fileSearch — 文件搜索工具

```json
{
  "fileSearch": {
    "fileSearchStoreNames": ["fileSearchStores/my-store-123"],
    "metadataFilter": "category = 'tech'",
    "topK": 5
  }
}
```

### 5.8 mcpServers — MCP 服务器集成

```json
{
  "mcpServers": [
    {
      "name": "my-mcp-server",
      "streamableHttpTransport": {
        "url": "https://api.example.com/mcp",
        "headers": { "Authorization": "Bearer token" },
        "timeout": "30s",
        "sseReadTimeout": "60s",
        "terminateOnClose": true
      }
    }
  ]
}
```

### 5.9 googleMaps — Google 地图工具

```json
{
  "googleMaps": {
    "enableWidget": true
  }
}
```

### ToolConfig

```json
{
  "toolConfig": {
    "functionCallingConfig": {
      "mode": "AUTO",
      "allowedFunctionNames": ["get_weather"]
    },
    "retrievalConfig": {
      "latLng": { "latitude": 40.7128, "longitude": -74.0060 },
      "languageCode": "en"
    },
    "includeServerSideToolInvocations": false
  }
}
```

#### FunctionCallingConfig Mode 枚举

| 值 | 说明 |
|---|------|
| `AUTO` | 默认，模型自行决定是否调用函数 |
| `ANY` | 强制模型调用函数，限制在 allowedFunctionNames 内 |
| `NONE` | 禁止函数调用 |
| `VALIDATED` | 模型决定是否调用，但使用约束解码验证函数调用 |

---

## 6. ThinkingConfig

```json
{
  "generationConfig": {
    "thinkingConfig": {
      "includeThoughts": true,
      "thinkingBudget": 8192,
      "thinkingLevel": "MEDIUM"
    }
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `includeThoughts` | `boolean` | 是否在响应中包含思考内容 |
| `thinkingBudget` | `integer` | 思考 token 数量上限 |
| `thinkingLevel` | `enum(ThinkingLevel)` | 思考深度等级 |

### ThinkingLevel 枚举

| 值 | 说明 |
|---|------|
| `THINKING_LEVEL_UNSPECIFIED` | 默认值 |
| `MINIMAL` | 几乎不思考 |
| `LOW` | 低度思考 |
| `MEDIUM` | 中度思考 |
| `HIGH` | 高度思考（默认） |

### OpenAI reasoning_effort 到 Gemini 的映射

| reasoning_effort (OpenAI) | Gemini 3.1 Pro thinking_level | Gemini 3.1 Flash-Lite thinking_level | Gemini 3 Flash thinking_level | Gemini 2.5 thinking_budget |
|---|---|---|---|---|
| `minimal` | `low` | `minimal` | `minimal` | `1,024` |
| `low` | `low` | `low` | `low` | `1,024` |
| `medium` | `medium` | `medium` | `medium` | `8,192` |
| `high` | `high` | `high` | `high` | `24,576` |

注意：`reasoning_effort` 和 `thinking_level`/`thinking_budget` 不能同时使用。Gemini 2.5 Pro 和 Gemini 3 系列无法关闭 thinking。

---

## 7. SafetySettings

### SafetySetting 结构

```json
{
  "safetySettings": [
    {
      "category": "HARM_CATEGORY_HARASSMENT",
      "threshold": "BLOCK_MEDIUM_AND_ABOVE"
    }
  ]
}
```

### HarmCategory 枚举

#### Gemini 系列（推荐使用）

| 值 | 说明 |
|---|------|
| `HARM_CATEGORY_HARASSMENT` | 骚扰内容 |
| `HARM_CATEGORY_HATE_SPEECH` | 仇恨言论 |
| `HARM_CATEGORY_SEXUALLY_EXPLICIT` | 色情内容 |
| `HARM_CATEGORY_DANGEROUS_CONTENT` | 危险内容 |
| `HARM_CATEGORY_CIVIC_INTEGRITY` | 危害公民诚信的内容（已弃用，用 enableEnhancedCivicAnswers 替代） |

#### PaLM 系列（旧版）

| 值 | 说明 |
|---|------|
| `HARM_CATEGORY_DEROGATORY` | 针对身份/受保护属性的负面评论 |
| `HARM_CATEGORY_TOXICITY` | 粗鲁、不敬或亵渎内容 |
| `HARM_CATEGORY_VIOLENCE` | 暴力描述 |
| `HARM_CATEGORY_SEXUAL` | 性行为引用或淫秽内容 |
| `HARM_CATEGORY_MEDICAL` | 未经核实的医疗建议 |
| `HARM_CATEGORY_DANGEROUS` | 有害行为推广 |

### HarmBlockThreshold 枚举

| 值 | 说明 |
|---|------|
| `HARM_BLOCK_THRESHOLD_UNSPECIFIED` | 未指定 |
| `BLOCK_LOW_AND_ABOVE` | 允许 NEGLIGIBLE |
| `BLOCK_MEDIUM_AND_ABOVE` | 允许 NEGLIGIBLE 和 LOW |
| `BLOCK_ONLY_HIGH` | 允许 NEGLIGIBLE、LOW 和 MEDIUM |
| `BLOCK_NONE` | 允许所有内容 |
| `OFF` | 关闭安全过滤 |

### HarmProbability 枚举

| 值 | 说明 |
|---|------|
| `HARM_PROBABILITY_UNSPECIFIED` | 未指定 |
| `NEGLIGIBLE` | 安全可能性极低 |
| `LOW` | 安全可能性低 |
| `MEDIUM` | 安全可能性中等 |
| `HIGH` | 安全可能性高 |

---

## 8. GenerateContentResponse 响应格式

```json
{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [{ "text": "Hello! How can I help you?" }]
      },
      "finishReason": "STOP",
      "safetyRatings": [
        { "category": "HARM_CATEGORY_HARASSMENT", "probability": "NEGLIGIBLE", "blocked": false }
      ],
      "citationMetadata": {
        "citationSources": [
          { "startIndex": 0, "endIndex": 100, "uri": "https://example.com", "license": "MIT" }
        ]
      },
      "tokenCount": 42,
      "groundingAttributions": [],
      "groundingMetadata": {},
      "avgLogprobs": -0.123,
      "logprobsResult": {},
      "urlContextMetadata": {},
      "index": 0,
      "finishMessage": ""
    }
  ],
  "promptFeedback": {
    "blockReason": "BLOCK_REASON_UNSPECIFIED",
    "safetyRatings": []
  },
  "usageMetadata": {},
  "modelVersion": "gemini-3-flash-preview-2025-09-01",
  "responseId": "resp_abc123",
  "modelStatus": {
    "modelStage": "STABLE",
    "retirementTime": "",
    "message": ""
  }
}
```

### Candidate 字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `content` | `object(Content)` | 模型生成的内容 |
| `finishReason` | `enum(FinishReason)` | 停止生成的原因 |
| `safetyRatings[]` | `object(SafetyRating)` | 安全评级列表 |
| `citationMetadata` | `object(CitationMetadata)` | 引用信息 |
| `tokenCount` | `integer` | 候选 token 数 |
| `groundingAttributions[]` | `object(GroundingAttribution)` | 归因信息 |
| `groundingMetadata` | `object(GroundingMetadata)` | Grounding 元数据 |
| `avgLogprobs` | `number` | 平均 log 概率 |
| `logprobsResult` | `object(LogprobsResult)` | Logprobs 结果 |
| `urlContextMetadata` | `object(UrlContextMetadata)` | URL 上下文元数据 |
| `index` | `integer` | 候选索引 |
| `finishMessage` | `string` | 停止原因详情 |

### FinishReason 枚举

| 值 | 说明 |
|---|------|
| `FINISH_REASON_UNSPECIFIED` | 默认值 |
| `STOP` | 自然停止或命中停止序列 |
| `MAX_TOKENS` | 达到最大 token 限制 |
| `SAFETY` | 安全原因被标记 |
| `RECITATION` | 引用（版权）原因被标记 |
| `LANGUAGE` | 不支持的语言被标记 |
| `OTHER` | 未知原因 |
| `BLOCKLIST` | 包含禁止术语 |
| `PROHIBITED_CONTENT` | 包含禁止内容 |
| `SPII` | 可能包含敏感个人信息 |
| `MALFORMED_FUNCTION_CALL` | 函数调用格式无效 |
| `IMAGE_SAFETY` | 生成的图片包含安全违规 |
| `IMAGE_PROHIBITED_CONTENT` | 图片包含其他禁止内容 |
| `IMAGE_OTHER` | 图片生成的其他问题 |
| `NO_IMAGE` | 预期生成图片但未生成 |
| `IMAGE_RECITATION` | 图片引用问题 |
| `UNEXPECTED_TOOL_CALL` | 模型生成了工具调用但未启用工具 |
| `TOO_MANY_TOOL_CALLS` | 连续工具调用过多 |
| `MISSING_THOUGHT_SIGNATURE` | 缺少思考签名 |
| `MALFORMED_RESPONSE` | 响应格式错误 |

### UsageMetadata

```json
{
  "promptTokenCount": 100,
  "cachedContentTokenCount": 50,
  "candidatesTokenCount": 200,
  "toolUsePromptTokenCount": 30,
  "thoughtsTokenCount": 500,
  "totalTokenCount": 880,
  "promptTokensDetails": [
    { "modality": "TEXT", "tokenCount": 80 },
    { "modality": "IMAGE", "tokenCount": 20 }
  ],
  "cacheTokensDetails": [],
  "candidatesTokensDetails": [],
  "toolUsePromptTokensDetails": []
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `promptTokenCount` | `integer` | 提示 token 数（含缓存） |
| `cachedContentTokenCount` | `integer` | 缓存部分的 token 数 |
| `candidatesTokenCount` | `integer` | 所有候选的总 token 数 |
| `toolUsePromptTokenCount` | `integer` | 工具使用提示的 token 数 |
| `thoughtsTokenCount` | `integer` | 思考 token 数（thinking models） |
| `totalTokenCount` | `integer` | 总 token 数（提示 + 响应） |
| `promptTokensDetails[]` | `ModalityTokenCount[]` | 按模态的输入 token 详情 |
| `cacheTokensDetails[]` | `ModalityTokenCount[]` | 按模态的缓存 token 详情 |
| `candidatesTokensDetails[]` | `ModalityTokenCount[]` | 按模态的输出 token 详情 |
| `toolUsePromptTokensDetails[]` | `ModalityTokenCount[]` | 按模态的工具使用 token 详情 |

ModalityTokenCount 结构：`{ "modality": "TEXT", "tokenCount": 100 }`

### GroundingMetadata

```json
{
  "groundingChunks": [
    { "web": { "uri": "https://example.com", "title": "Example" } },
    { "image": { "sourceUri": "https://...", "imageUri": "https://...", "title": "...", "domain": "example.com" } },
    { "retrievedContext": { "uri": "...", "title": "...", "text": "...", "fileSearchStore": "fileSearchStores/123" } },
    { "maps": { "uri": "...", "title": "...", "text": "...", "placeId": "places/ChIJ...", "placeAnswerSources": { "reviewSnippets": [...] } } }
  ],
  "groundingSupports": [
    {
      "groundingChunkIndices": [0, 2],
      "confidenceScores": [0.95, 0.88],
      "renderedParts": [0],
      "segment": { "partIndex": 0, "startIndex": 0, "endIndex": 100, "text": "..." }
    }
  ],
  "webSearchQueries": ["latest AI news"],
  "imageSearchQueries": ["cute cats"],
  "searchEntryPoint": {
    "renderedContent": "<div>...</div>",
    "sdkBlob": "base64..."
  },
  "retrievalMetadata": {
    "googleSearchDynamicRetrievalScore": 0.85
  },
  "googleMapsWidgetContextToken": "token_string"
}
```

### LogprobsResult

```json
{
  "topCandidates": [
    { "candidates": [
      { "token": "Hello", "tokenId": 1234, "logProbability": -0.01 },
      { "token": "Hi", "tokenId": 5678, "logProbability": -3.5 }
    ]}
  ],
  "chosenCandidates": [
    { "token": "Hello", "tokenId": 1234, "logProbability": -0.01 }
  ],
  "logProbabilitySum": -15.3
}
```

### PromptFeedback

```json
{
  "blockReason": "SAFETY",
  "safetyRatings": [
    { "category": "HARM_CATEGORY_HARASSMENT", "probability": "NEGLIGIBLE", "blocked": false }
  ]
}
```

### BlockReason 枚举

| 值 | 说明 |
|---|------|
| `BLOCK_REASON_UNSPECIFIED` | 默认值 |
| `SAFETY` | 安全原因被阻止 |
| `OTHER` | 未知原因 |
| `BLOCKLIST` | 术语黑名单 |
| `PROHIBITED_CONTENT` | 禁止内容 |
| `IMAGE_SAFETY` | 不安全的图片生成内容 |

---

## 9. 流式响应格式 (streamGenerateContent)

### 请求方式

```
POST https://generativelanguage.googleapis.com/v1beta/{model=models/*}:streamGenerateContent?alt=sse
```

使用 `?alt=sse` 查询参数启用 SSE 流式输出。

### SSE 响应格式

每个事件是一个完整的 `GenerateContentResponse` JSON：

```
data: {"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":1,"totalTokenCount":11}}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"!"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2,"totalTokenCount":12}}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":""}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":2,"totalTokenCount":12}}
```

注意：Gemini 流式响应**没有** `[DONE]` 标记（不同于 OpenAI 的 SSE 格式）。流在 HTTP 连接关闭时结束。

### Grounding 流式注意

当使用 grounding 时，`groundingChunks` 仅包含**尚未出现在先前响应中的** grounding 元数据块。客户端需要自行累积所有响应中的 grounding chunks。

---

## 10. Context Caching API

### CachedContent 资源

```json
{
  "name": "cachedContents/abc123",
  "displayName": "My Cache",
  "model": "models/gemini-2.5-flash",
  "contents": [{ "role": "user", "parts": [{ "text": "Long document..." }] }],
  "tools": [],
  "systemInstruction": { "role": "user", "parts": [{ "text": "System prompt" }] },
  "toolConfig": {},
  "createTime": "2025-01-01T00:00:00Z",
  "updateTime": "2025-01-01T00:00:00Z",
  "usageMetadata": { "totalTokenCount": 50000 },
  "expireTime": "2025-01-08T00:00:00Z",
  "ttl": "604800s"
}
```

### 创建缓存

```
POST /v1beta/cachedContents
```

请求体为 CachedContent 对象，必填字段：
- `model` — 必填，格式 `models/{model}`
- `contents` — 可选，要缓存的内容
- `expiration` — 必须设置 `expireTime` 或 `ttl` 之一

### 更新缓存

```
PATCH /v1beta/{cachedContent.name=cachedContents/*}?updateMask=expireTime,ttl
```

仅可更新过期时间（`expireTime` 或 `ttl`）。

### 使用缓存

在 generateContent 请求中设置 `cachedContent` 字段：
```json
{
  "cachedContent": "cachedContents/abc123",
  "contents": [{ "role": "user", "parts": [{ "text": "Follow-up question" }] }]
}
```

---

## 11. File API

### 上传文件

使用 resumable upload：
```
POST https://generativelanguage.googleapis.com/upload/v1beta/files
```

或 metadata only：
```
POST https://generativelanguage.googleapis.com/v1beta/files
```

### File 资源

```json
{
  "name": "files/abc-123",
  "displayName": "My Image",
  "mimeType": "image/png",
  "sizeBytes": "123456",
  "createTime": "2025-01-01T00:00:00Z",
  "updateTime": "2025-01-01T00:00:00Z",
  "expirationTime": "2025-01-08T00:00:00Z",
  "sha256Hash": "base64...",
  "uri": "https://generativelanguage.googleapis.com/v1beta/files/abc-123",
  "downloadUri": "https://...",
  "state": "ACTIVE",
  "source": "UPLOADED",
  "error": { "code": 0, "message": "" },
  "videoMetadata": { "videoDuration": "120s" }
}
```

### State 枚举

| 值 | 说明 |
|---|------|
| `STATE_UNSPECIFIED` | 默认 |
| `PROCESSING` | 正在处理 |
| `ACTIVE` | 可用 |
| `FAILED` | 处理失败 |

### Source 枚举

| 值 | 说明 |
|---|------|
| `SOURCE_UNSPECIFIED` | 未指定 |
| `UPLOADED` | 用户上传 |
| `GENERATED` | Google 生成 |
| `REGISTERED` | 注册的 GCS 文件 |

### 列出文件

```
GET /v1beta/files?pageSize=10&pageToken=xxx
```

响应：`{ "files": [...], "nextPageToken": "xxx" }`

### 注册 GCS 文件

```
POST /v1beta/files:register
{ "uris": ["gs://bucket/object"] }
```

响应：`{ "files": [...] }`

---

## 12. Token 计数端点

```
POST /v1beta/{model=models/*}:countTokens
```

### 请求体

```json
{
  "contents": [{ "role": "user", "parts": [{ "text": "Hello" }] }]
}
```

或使用完整 generateContent 请求（两者互斥）：

```json
{
  "generateContentRequest": {
    "contents": [...],
    "systemInstruction": {...},
    "tools": [...],
    "generationConfig": {...}
  }
}
```

### 响应体

```json
{
  "totalTokens": 42,
  "cachedContentTokenCount": 10,
  "promptTokensDetails": [
    { "modality": "TEXT", "tokenCount": 32 },
    { "modality": "IMAGE", "tokenCount": 10 }
  ],
  "cacheTokensDetails": []
}
```

---

## 13. Embeddings 端点

### 单条嵌入

```
POST /v1beta/{model=models/*}:embedContent
```

```json
{
  "content": { "parts": [{ "text": "Hello world" }] },
  "taskType": "RETRIEVAL_DOCUMENT",
  "title": "My Document",
  "outputDimensionality": 256
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `content` | `object(Content)` | 必填，要嵌入的内容 |
| `taskType` | `enum(TaskType)` | 可选，任务类型 |
| `title` | `string` | 可选，仅适用于 RETRIEVAL_DOCUMENT |
| `outputDimensionality` | `integer` | 可选，输出维度截断 |

响应：
```json
{
  "embedding": {
    "values": [0.1, 0.2, ...],
    "shape": [768]
  }
}
```

### 批量嵌入

```
POST /v1beta/{model=models/*}:batchEmbedContents
```

```json
{
  "requests": [
    {
      "model": "models/gemini-embedding-001",
      "content": { "parts": [{ "text": "Hello" }] },
      "taskType": "RETRIEVAL_QUERY"
    }
  ]
}
```

响应：`{ "embeddings": [{ "values": [...], "shape": [768] }] }`

### TaskType 枚举

| 值 | 说明 |
|---|------|
| `TASK_TYPE_UNSPECIFIED` | 默认 |
| `RETRIEVAL_QUERY` | 搜索/检索场景的查询 |
| `RETRIEVAL_DOCUMENT` | 被搜索的文档 |
| `SEMANTIC_SIMILARITY` | 语义相似度 |
| `CLASSIFICATION` | 文本分类 |
| `CLUSTERING` | 聚类 |
| `QUESTION_ANSWERING` | 问答 |
| `FACT_VERIFICATION` | 事实验证 |
| `CODE_RETRIEVAL_QUERY` | 代码检索 |

---

## 14. OpenAI 兼容端点

Base URL: `https://generativelanguage.googleapis.com/v1beta/openai/`

认证方式: `Authorization: Bearer $GEMINI_API_KEY`

### Chat Completions

```
POST /v1beta/openai/chat/completions
```

请求/响应格式与 OpenAI Chat Completions API 完全兼容。支持：
- Thinking（`reasoning_effort`）
- Streaming
- Function calling
- Image understanding
- Audio understanding
- Structured output
- Flex/Priority inference（`service_tier`）

### Images

```
POST /v1beta/openai/images/generations
```

支持的参数：`prompt`, `model`, `n`, `size`, `response_format`。
可用模型：`gemini-2.5-flash-image`, `gemini-3-pro-image-preview`。

### Videos

```
POST /v1beta/openai/videos
```

创建视频生成任务（长时运行操作），返回 operation ID。
可用模型：`veo-3.1-generate-preview`。

```
GET /v1beta/openai/videos/{id}
```

轮询视频生成状态。

### Embeddings

```
POST /v1beta/openai/embeddings
```

可用模型：`gemini-embedding-2-preview`, `gemini-embedding-001`。

### Models

```
GET /v1beta/openai/models
GET /v1beta/openai/models/{model}
```

### extra_body 字段（Gemini 专有参数）

通过 `extra_body` 传递 Gemini 特有的参数：

| 参数 | 类型 | 适用端点 | 说明 |
|------|------|---------|------|
| `cached_content` | Text | Chat | 内容缓存 |
| `thinking_config` | Object | Chat | ThinkingConfig 配置 |
| `aspect_ratio` | Text | Images | 输出宽高比 |
| `generation_config` | Object | Images | Gemini 生成配置 |
| `safety_settings` | List | Images | 安全过滤设置 |
| `tools` | List | Images | 启用 grounding |
| `aspect_ratio` | Text | Video | 视频宽高比 |
| `resolution` | Text | Video | 输出分辨率（720p/1080p/4K） |
| `duration_seconds` | Integer | Video | 生成时长（4/6/8） |
| `frame_rate` | Text | Video | 帧率 |
| `input_reference` | Text | Video | 参考输入 |
| `extend_video_id` | Text | Video | 视频扩展 ID |
| `negative_prompt` | Text | Video | 排除项 |
| `seed` | Integer | Video | 确定性生成 |
| `style` | Text | Video | 视觉风格（cinematic/creative） |
| `person_generation` | Text | Video | 人物生成控制 |
| `reference_images` | List | Video | 参考图片（最多 3 张） |
| `image` | Text | Video | 首帧图片 |
| `last_frame` | Object | Video | 末帧图片 |

### Flex 和 Priority 推理

通过 `service_tier` 参数指定：
- `"standard"` — 标准层级（默认）
- `"flex"` — 灵活层级（更低成本，可能更高延迟）
- `"priority"` — 优先层级（更低延迟）

---

## 15. 错误格式

### REST API 错误（Google RPC Status 格式）

所有 Gemini REST API 端点使用 Google RPC Status 错误格式：

```json
{
  "error": {
    "code": 400,
    "message": "API key not valid. Please pass a valid API key.",
    "status": "INVALID_ARGUMENT",
    "details": [
      {
        "@type": "type.googleapis.com/google.rpc.ErrorInfo",
        "reason": "API_KEY_INVALID",
        "domain": "googleapis.com",
        "metadata": { "service": "generativelanguage.googleapis.com" }
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | `integer` | HTTP 状态码 |
| `message` | `string` | 面向开发者的英文错误信息 |
| `status` | `string` | gRPC 状态码字符串 |
| `details[]` | `object[]` | 错误详情列表 |

### 常见 HTTP 状态码

| 状态码 | 含义 |
|--------|------|
| 400 | 请求参数错误（INVALID_ARGUMENT） |
| 401 | 认证失败（UNAUTHENTICATED） |
| 403 | 权限不足（PERMISSION_DENIED） |
| 404 | 资源不存在（NOT_FOUND） |
| 429 | 频率限制（RESOURCE_EXHAUSTED） |
| 500 | 内部错误（INTERNAL） |
| 503 | 服务不可用（UNAVAILABLE） |

### 内容过滤错误（promptFeedback.blockReason）

当提示被阻止时，响应中 `promptFeedback.blockReason` 不为空，且 `candidates` 为空数组。此时应检查 `promptFeedback.safetyRatings` 了解具体被阻止的类别。

---

## 16. 可用模型列表

### 当前活跃模型

#### Gemini 3 系列

| 模型 ID | 说明 |
|---------|------|
| `gemini-3-flash-preview` | 最佳性价比推理模型 |
| `gemini-3-pro-image-preview` | 带图像生成的专业模型 |

#### Gemini 2.5 系列

| 模型 ID | 说明 | 输入限制 | 输出限制 |
|---------|------|---------|---------|
| `gemini-2.5-flash` | 高性能推理模型 | 1,048,576 tokens | 65,536 tokens |
| `gemini-2.5-flash-lite` | 轻量级推理模型 | 1,048,576 tokens | 65,536 tokens |
| `gemini-2.5-pro` | 最先进的复杂任务模型 | 1,048,576 tokens | 65,536 tokens |
| `gemini-2.5-flash-image` | 图像生成模型 | — | — |
| `gemini-2.5-pro-tts-preview` | 高保真语音合成 | — | — |

#### Nano Banana 系列（图像生成）

| 模型 ID | 说明 |
|---------|------|
| `nano-banana` | 原生图像生成与编辑 |
| `nano-banana-2-preview` | 高效率大规模视觉创作 |
| `nano-banana-pro-preview` | 专业 4K 视觉设计引擎 |

#### Veo 系列（视频生成）

| 模型 ID | 说明 |
|---------|------|
| `veo-3.1-generate-preview` | 电影级视频生成 |
| `veo-3.1-lite-preview` | 低成本视频生成 |

#### Lyria 系列（音乐生成）

| 模型 ID | 说明 |
|---------|------|
| `lyria-3-pro-preview` | 全长歌曲生成 |
| `lyria-3-clip-preview` | 短音乐片段生成（30s） |
| `lyria-realtime-experimental` | 实时音乐生成 |

#### Imagen 系列

| 模型 ID | 说明 |
|---------|------|
| `imagen-4` | 文本到图像，支持 2K 分辨率 |

#### 特殊用途模型

| 模型 ID | 说明 |
|---------|------|
| `computer-use-preview` | 计算机使用（浏览器自动化） |
| `gemini-deep-research-preview` | 深度研究代理 |
| `gemini-embedding-2-preview` | 多模态嵌入模型 |
| `gemini-embedding-001` | 文本嵌入（768 维） |
| `text-embedding-004` | 文本嵌入（768 维） |
| `gemini-robotics-er-1.6-preview` | 机器人具身推理 |

#### 已弃用模型

| 模型 ID | 说明 |
|---------|------|
| `gemini-2.0-flash` | 已弃用 |
| `gemini-1.5-flash` | 输入 1,048,576 / 输出 8,192 |
| `gemini-1.5-flash-8b` | 输入 1,048,576 / 输出 8,192 |
| `gemini-1.5-pro` | 输入 2,097,152 / 输出 8,192 |
| `gemini-1.0-pro` | 已弃用 |
| `aqa` | 输入 7,168 / 输出 1,024 |

### 模型版本命名规则

| 后缀 | 说明 | 示例 |
|------|------|------|
| 无后缀 | 稳定版 | `gemini-2.5-flash` |
| `-preview-*` | 预览版，可用于生产 | `gemini-2.5-flash-preview-09-2025` |
| `-latest` | 最新版本别名 | `gemini-flash-latest` |
| `-experimental-*` | 实验版，不推荐生产使用 | — |

### ModelStage 枚举

| 值 | 说明 |
|---|------|
| `MODEL_STAGE_UNSPECIFIED` | 未指定 |
| `UNSTABLE_EXPERIMENTAL` | 频繁调整的实验阶段 |
| `EXPERIMENTAL` | 实验阶段 |
| `PREVIEW` | 预览阶段（比实验更成熟） |
| `STABLE` | 稳定阶段（可用于生产） |
| `LEGACY` | 遗留阶段（即将弃用） |
| `DEPRECATED` | 已弃用（不可使用） |
| `RETIRED` | 已退役（不可使用） |

### ServiceTier 枚举

| 值 | 说明 |
|---|------|
| `unspecified` | 默认，等同于 standard |
| `standard` | 标准层级 |
| `flex` | 灵活层级（低成本，高延迟） |
| `priority` | 优先层级（低延迟） |

---

## 17. 枚举值速查表

### FinishReason 完整列表（21 值）

| Gemini | OpenAI 映射 | Claude 映射 |
|--------|------------|-------------|
| `STOP` | `stop` | `end_turn` |
| `MAX_TOKENS` | `length` | `max_tokens` |
| `SAFETY` | `content_filter` | — |
| `RECITATION` | — | — |
| `LANGUAGE` | — | — |
| `OTHER` | — | — |
| `BLOCKLIST` | `content_filter` | — |
| `PROHIBITED_CONTENT` | `content_filter` | — |
| `SPII` | — | — |
| `MALFORMED_FUNCTION_CALL` | — | — |
| `IMAGE_SAFETY` | `content_filter` | — |
| `IMAGE_PROHIBITED_CONTENT` | `content_filter` | — |
| `IMAGE_OTHER` | — | — |
| `NO_IMAGE` | — | — |
| `IMAGE_RECITATION` | — | — |
| `UNEXPECTED_TOOL_CALL` | — | — |
| `TOO_MANY_TOOL_CALLS` | — | — |
| `MISSING_THOUGHT_SIGNATURE` | — | — |
| `MALFORMED_RESPONSE` | — | — |

### FunctionCallingConfig Mode

| Gemini | OpenAI tool_choice 映射 |
|--------|------------------------|
| `AUTO` | `"auto"` |
| `ANY` | `"required"` 或 `{"type": "function", "function": {"name": "..."}}` |
| `NONE` | `"none"` |
| `VALIDATED` | `"auto"`（带约束验证） |

### Content Part Modality（用于 ModalityTokenCount）

| 值 | 说明 |
|---|------|
| `TEXT` | 纯文本 |
| `IMAGE` | 图像 |
| `VIDEO` | 视频 |
| `AUDIO` | 音频 |
| `DOCUMENT` | 文档（如 PDF） |

### Response Modality（用于 responseModalities）

| 值 | 说明 |
|---|------|
| `TEXT` | 返回文本 |
| `IMAGE` | 返回图像 |
| `AUDIO` | 返回音频 |

### UrlRetrievalStatus

| 值 | 说明 |
|---|------|
| `URL_RETRIEVAL_STATUS_UNSPECIFIED` | 默认 |
| `URL_RETRIEVAL_STATUS_SUCCESS` | 成功 |
| `URL_RETRIEVAL_STATUS_ERROR` | 失败 |
| `URL_RETRIEVAL_STATUS_PAYWALL` | 内容付费墙 |
| `URL_RETRIEVAL_STATUS_UNSAFE` | 内容不安全 |

### Code Language

| 值 | 说明 |
|---|------|
| `LANGUAGE_UNSPECIFIED` | 未指定 |
| `PYTHON` | Python >= 3.10 |

### Function Behavior

| 值 | 说明 |
|---|------|
| `UNSPECIFIED` | 未指定 |
| `BLOCKING` | 阻塞等待响应 |
| `NON_BLOCKING` | 非阻塞，异步响应 |

### Scheduling

| 值 | 说明 |
|---|------|
| `SCHEDULING_UNSPECIFIED` | 默认 |
| `SILENT` | 仅添加到上下文，不触发生成 |
| `WHEN_IDLE` | 空闲时触发生成 |
| `INTERRUPT` | 中断当前生成并触发 |
