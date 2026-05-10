# 图像生成 API 协议文档

> 基于 OpenAI Images API 官方文档 (https://platform.openai.com/docs/api-reference/images) 整理
>
> 本文档用于 team-api 协议转换模块的开发参考，涵盖图像生成（Generations）、图像编辑（Edits）、图像变体（Variations）三个端点的完整请求/响应规范，以及各供应商适配器的协议差异。

---

## 目录

- [1. API 概览](#1-api-概览)
- [2. 认证方式](#2-认证方式)
- [3. 图像生成 (Create Image)](#3-图像生成-create-image)
- [4. 图像编辑 (Edit Image)](#4-图像编辑-edit-image)
- [5. 图像变体 (Create Image Variation)](#5-图像变体-create-image-variation)
- [6. 响应格式](#6-响应格式)
- [7. 流式响应 (GPT Image 模型)](#7-流式响应-gpt-image-模型)
- [8. 各模型参数差异对比](#8-各模型参数差异对比)
- [9. 错误格式](#9-错误格式)
- [10. 供应商适配器协议差异](#10-供应商适配器协议差异)
- [11. 协议转换注意事项](#11-协议转换注意事项)

---

## 1. API 概览

OpenAI 图像 API 提供三个端点，支持 DALL-E 系列和 GPT Image 系列模型：

| 端点 | 方法 | 路径 | 功能 | 支持模型 |
|------|------|------|------|----------|
| 图像生成 | POST | `/v1/images/generations` | 根据文本提示生成图像 | `dall-e-2`, `dall-e-3`, `gpt-image-1`, `gpt-image-1-mini`, `gpt-image-1.5` |
| 图像编辑 | POST | `/v1/images/edits` | 编辑现有图像（支持多图） | `dall-e-2`, `gpt-image-1` |
| 图像变体 | POST | `/v1/images/variations` | 基于现有图像生成变体 | `dall-e-2` |

### 基本请求示例（图像生成）

```bash
curl https://api.openai.com/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
    "model": "gpt-image-1",
    "prompt": "A cute baby sea otter",
    "n": 1,
    "size": "1024x1024"
  }'
```

### 基本请求示例（图像编辑，multipart/form-data）

```bash
curl https://api.openai.com/v1/images/edits \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -F image="@otter.png" \
  -F prompt="A cute baby sea otter wearing a beret" \
  -F model="gpt-image-1"
```

---

## 2. 认证方式

请求头中携带 Bearer Token，与 Chat Completions API 相同：

```
Authorization: Bearer sk-xxxxxxxxxxxxxxxx
```

| 认证方式 | 说明 |
|---------|------|
| API Key | `Authorization: Bearer sk-xxx`，在平台 API Keys 页面创建 |
| Organization（可选） | `OpenAI-Organization: org-xxx`，指定组织 |
| Project（可选） | `OpenAI-Project: proj-xxx`，指定项目 |

---

## 3. 图像生成 (Create Image)

**端点**：`POST /v1/images/generations`

**Content-Type**：`application/json`

### 3.1 完整参数表

| 参数名 | 类型 | 必填 | 默认值 | 适用模型 | 说明 |
|--------|------|------|--------|----------|------|
| `prompt` | string | **是** | — | 全部 | 图像描述文本。GPT Image 最长 32000 字符，dall-e-2 最长 1000 字符，dall-e-3 最长 4000 字符 |
| `model` | string | **是** | `dall-e-2` | — | 模型 ID：`dall-e-2`、`dall-e-3`、`gpt-image-1`、`gpt-image-1-mini`、`gpt-image-1.5` |
| `n` | integer | 否 | `1` | 全部 | 生成图像数量。dall-e-2/dall-e-3: 1；GPT Image: 1-10 |
| `size` | string | 否 | `1024x1024` | 全部 | 图像尺寸，各模型支持值不同，详见下方 |
| `quality` | string | 否 | `auto` | GPT Image; dall-e-3 | 图像质量。GPT Image: `auto`/`high`/`medium`/`low`；dall-e-3: `standard`/`hd` |
| `response_format` | string | 否 | `url` | 全部 | 返回格式：`url` 或 `b64_json`。GPT Image 默认 `b64_json` |
| `background` | string | 否 | `auto` | GPT Image | 背景透明度：`transparent`、`opaque`、`auto`。仅支持 PNG/WebP 输出格式 |
| `output_format` | string | 否 | `png` | GPT Image | 输出图像格式：`png`、`jpeg`、`webp` |
| `output_compression` | integer | 否 | — | GPT Image | 输出压缩率（0-100），仅 `jpeg` 和 `webp` 格式有效。100 = 无压缩 |
| `partial_images` | integer | 否 | `0` | GPT Image | 流式中间图数量（0-3）。设为非 0 时自动启用流式模式 |
| `stream` | boolean | 否 | `false` | GPT Image | 是否启用流式响应。也可通过 `partial_images > 0` 隐式启用 |
| `style` | string | 否 | `vivid` | dall-e-3 | 图像风格：`vivid`（生动）或 `natural`（自然） |
| `moderation` | string | 否 | `auto` | GPT Image | 内容审核严格度：`auto` 或 `low` |
| `user` | string | 否 | — | 全部 | 终端用户标识符，用于监控滥用 |

### 3.2 各模型支持的尺寸

**dall-e-2**：

| 尺寸 |
|------|
| `256x256` |
| `512x512` |
| `1024x1024` |

**dall-e-3**：

| 尺寸 |
|------|
| `1024x1024` |
| `1024x1792`（纵向） |
| `1792x1024`（横向） |

**gpt-image-1 / gpt-image-1-mini / gpt-image-1.5**：

| 尺寸 |
|------|
| `1024x1024`（正方形） |
| `1536x1024`（横向） |
| `1024x1536`（纵向） |

### 3.3 请求示例

**DALL-E 3 高质量生成**：

```json
{
  "model": "dall-e-3",
  "prompt": "A white Siamese cat wearing a tiny top hat, digital art",
  "n": 1,
  "size": "1792x1024",
  "quality": "hd",
  "style": "vivid",
  "response_format": "url"
}
```

**GPT Image 透明背景 PNG**：

```json
{
  "model": "gpt-image-1",
  "prompt": "A red logo of a rocket ship, minimalist, clean design",
  "n": 1,
  "size": "1024x1024",
  "quality": "high",
  "background": "transparent",
  "output_format": "png"
}
```

**GPT Image 流式生成（含中间图）**：

```json
{
  "model": "gpt-image-1",
  "prompt": "A sunset over a mountain lake",
  "n": 1,
  "size": "1024x1024",
  "partial_images": 2
}
```

---

## 4. 图像编辑 (Edit Image)

**端点**：`POST /v1/images/edits`

**Content-Type**：`multipart/form-data` 或 `application/json`（GPT Image 模型支持 JSON）

### 4.1 完整参数表

| 参数名 | 类型 | 必填 | 默认值 | 适用模型 | 说明 |
|--------|------|------|--------|----------|------|
| `image` | file / array | **是** | — | 全部 | 源图像文件。dall-e-2: 单个 PNG（正方形，<4MB）；GPT Image: 最多 16 张图（PNG/JPEG/WebP，每张 <20MB） |
| `prompt` | string | **是** | — | 全部 | 编辑指令文本 |
| `model` | string | 否 | `dall-e-2` | — | 模型 ID：`dall-e-2` 或 `gpt-image-1` |
| `mask` | file | 否 | — | dall-e-2 | 遮罩图像（PNG，与源图同尺寸）。透明区域表示需要编辑的部分 |
| `n` | integer | 否 | `1` | 全部 | 生成图像数量。dall-e-2: 1；GPT Image: 1-10 |
| `size` | string | 否 | `1024x1024` | 全部 | 输出尺寸。dall-e-2: 256x256/512x512/1024x1024；GPT Image: 同生成端点 |
| `response_format` | string | 否 | `url` | 全部 | 返回格式：`url` 或 `b64_json` |
| `quality` | string | 否 | `auto` | GPT Image | 图像质量：`auto`/`high`/`medium`/`low` |
| `background` | string | 否 | `auto` | GPT Image | 背景透明度：`transparent`/`opaque`/`auto` |
| `output_format` | string | 否 | `png` | GPT Image | 输出格式：`png`/`jpeg`/`webp` |
| `output_compression` | integer | 否 | — | GPT Image | 压缩率 0-100 |
| `partial_images` | integer | 否 | `0` | GPT Image | 流式中间图数量 0-3 |
| `stream` | boolean | 否 | `false` | GPT Image | 是否流式 |
| `user` | string | 否 | — | 全部 | 终端用户标识 |

### 4.2 GPT Image 的 JSON 请求格式

GPT Image 模型支持以 JSON 传递图像 URL 或 base64 数据（而非 multipart/form-data）：

```json
{
  "model": "gpt-image-1",
  "prompt": "Add a red bow tie to the cat",
  "image": [
    {
      "type": "image_url",
      "url": "https://example.com/cat.png"
    }
  ],
  "n": 1,
  "size": "1024x1024"
}
```

或使用 base64 内联：

```json
{
  "model": "gpt-image-1",
  "prompt": "Add a red bow tie to the cat",
  "image": [
    {
      "type": "image_url",
      "url": "data:image/png;base64,iVBORw0KGgo..."
    }
  ]
}
```

### 4.3 DALL-E 2 的遮罩编辑

DALL-E 2 的编辑使用 mask 参数标记需要重新生成的区域。mask 必须满足：
- 格式为 PNG
- 尺寸与源图相同
- 透明（alpha = 0）区域表示需要编辑的部分
- 非透明区域保留原样

```bash
curl https://api.openai.com/v1/images/edits \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -F image="@original.png" \
  -F mask="@mask.png" \
  -F prompt="Replace the background with a beach" \
  -F model="dall-e-2"
```

---

## 5. 图像变体 (Create Image Variation)

**端点**：`POST /v1/images/variations`

**Content-Type**：`multipart/form-data`

**仅支持 `dall-e-2` 模型**。

### 5.1 完整参数表

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `image` | file | **是** | — | 源图像（PNG，正方形，<4MB） |
| `model` | string | 否 | `dall-e-2` | 必须为 `dall-e-2` |
| `n` | integer | 否 | `1` | 生成变体数量（1-10） |
| `size` | string | 否 | `1024x1024` | 输出尺寸：`256x256`/`512x512`/`1024x1024` |
| `response_format` | string | 否 | `url` | 返回格式：`url` 或 `b64_json` |
| `user` | string | 否 | — | 终端用户标识 |

### 5.2 请求示例

```bash
curl https://api.openai.com/v1/images/variations \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -F image="@otter.png" \
  -F n=2 \
  -F size="1024x1024"
```

---

## 6. 响应格式

### 6.1 非流式响应（所有模型通用）

```json
{
  "created": 1706734328,
  "data": [
    {
      "url": "https://oaidalleapiprodscus.blob.core.windows.net/private/...",
      "revised_prompt": "A white Siamese cat wearing a tiny top hat, rendered in digital art style with vibrant colors"
    }
  ]
}
```

或 base64 格式（`response_format: "b64_json"`）：

```json
{
  "created": 1706734328,
  "data": [
    {
      "b64_json": "iVBORw0KGgoAAAANSUhEUgAA..."
    }
  ]
}
```

### 6.2 GPT Image 增强响应

GPT Image 模型在响应中包含额外的元数据字段：

```json
{
  "created": 1706734328,
  "data": [
    {
      "b64_json": "iVBORw0KGgoAAAANSUhEUgAA..."
    }
  ],
  "background": "transparent",
  "output_format": "png",
  "size": "1024x1024",
  "quality": "high",
  "usage": {
    "total_tokens": 100,
    "input_tokens": 50,
    "output_tokens": 50,
    "input_tokens_details": {
      "text_tokens": 20,
      "image_tokens": 30
    }
  }
}
```

### 6.3 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `created` | integer | 创建时间戳（Unix 时间） |
| `data` | array | 图像数据数组 |
| `data[].url` | string | 图像下载 URL（`response_format: "url"` 时返回，有效期 1 小时） |
| `data[].b64_json` | string | 图像 Base64 编码（`response_format: "b64_json"` 时返回） |
| `data[].revised_prompt` | string | 模型修改后的 prompt（dall-e-3 和 GPT Image 可能返回） |
| `background` | string | 输出背景（GPT Image 专属） |
| `output_format` | string | 输出格式（GPT Image 专属） |
| `size` | string | 输出尺寸（GPT Image 专属） |
| `quality` | string | 输出质量（GPT Image 专属） |
| `usage` | object | Token 使用量（GPT Image 专属） |

### 6.4 Usage 字段说明（GPT Image）

| 字段 | 类型 | 说明 |
|------|------|------|
| `usage.total_tokens` | integer | 总 token 数 |
| `usage.input_tokens` | integer | 输入 token 数 |
| `usage.output_tokens` | integer | 输出 token 数（图像 token） |
| `usage.input_tokens_details.text_tokens` | integer | 文本输入 token 数 |
| `usage.input_tokens_details.image_tokens` | integer | 图像输入 token 数（编辑场景） |

---

## 7. 流式响应 (GPT Image 模型)

仅 GPT Image 模型（`gpt-image-1`、`gpt-image-1-mini`、`gpt-image-1.5`）支持流式响应。通过设置 `stream: true` 或 `partial_images > 0` 启用。

**Content-Type**：`text/event-stream`

### 7.1 流式事件格式

流式响应通过 SSE 返回，每个事件包含一个 JSON 对象：

```
data: {"type": "generation.partial", "data": [{"b64_json": "...", "index": 0}]}

data: {"type": "generation.partial", "data": [{"b64_json": "...", "index": 0}]}

data: {"type": "generation.completed", "data": [{"b64_json": "...", "index": 0}]}
```

### 7.2 事件类型

| 事件类型 | 说明 |
|---------|------|
| `generation.partial` | 中间过程图，数量取决于 `partial_images` 参数 |
| `generation.completed` | 最终完成的图像 |

### 7.3 流式请求示例

```json
{
  "model": "gpt-image-1",
  "prompt": "A beautiful mountain landscape at sunset",
  "n": 1,
  "size": "1024x1024",
  "partial_images": 2,
  "stream": true
}
```

### 7.4 流式响应完整示例

```
data: {"type":"generation.partial","data":[{"b64_json":"iVBORw0KGgo...partial1","index":0}],"usage":{"total_tokens":0,"input_tokens":50,"output_tokens":0,"input_tokens_details":{"text_tokens":50,"image_tokens":0}}}

data: {"type":"generation.partial","data":[{"b64_json":"iVBORw0KGgo...partial2","index":0}],"usage":{"total_tokens":0,"input_tokens":50,"output_tokens":0,"input_tokens_details":{"text_tokens":50,"image_tokens":0}}}

data: {"type":"generation.completed","data":[{"b64_json":"iVBORw0KGgo...final","index":0}],"usage":{"total_tokens":100,"input_tokens":50,"output_tokens":50,"input_tokens_details":{"text_tokens":50,"image_tokens":0}}}
```

---

## 8. 各模型参数差异对比

| 参数 | dall-e-2 | dall-e-3 | gpt-image-1 / mini / 1.5 |
|------|----------|----------|--------------------------|
| **生成端点** | 支持 | 支持 | 支持 |
| **编辑端点** | 支持（mask） | 不支持 | 支持（多图，无 mask） |
| **变体端点** | 支持 | 不支持 | 不支持 |
| **prompt 最大长度** | 1000 字符 | 4000 字符 | 32000 字符 |
| **n（生成数量）** | 1 | 1 | 1-10 |
| **尺寸** | 256/512/1024 正方形 | 1024 正方形 + 横纵 1792 | 1024 正方形 + 1536 横纵 |
| **quality** | 不支持 | `standard`/`hd` | `auto`/`high`/`medium`/`low` |
| **style** | 不支持 | `vivid`/`natural` | 不支持 |
| **background** | 不支持 | 不支持 | `transparent`/`opaque`/`auto` |
| **output_format** | 不支持 | 不支持 | `png`/`jpeg`/`webp` |
| **output_compression** | 不支持 | 不支持 | 0-100 |
| **流式响应** | 不支持 | 不支持 | 支持（`partial_images`） |
| **moderation** | 不支持 | 不支持 | `auto`/`low` |
| **response_format 默认值** | `url` | `url` | `b64_json` |
| **Usage 统计** | 不返回 | 不返回 | 返回 |

---

## 9. 错误格式

图像 API 使用与 Chat Completions 相同的 OpenAI 错误格式：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid size for model dall-e-3",
    "param": "size",
    "code": null
  }
}
```

### 常见错误

| HTTP 状态码 | error.type | 说明 |
|------------|-----------|------|
| 400 | `invalid_request_error` | 参数错误（不支持的尺寸、无效的 prompt 等） |
| 401 | `authentication_error` | API Key 无效或缺失 |
| 403 | `permission_error` | 账号无权使用该模型 |
| 429 | `rate_limit_error` | 请求频率超限 |
| 500 | `server_error` | 服务器内部错误 |

### 特殊错误场景

| 场景 | 说明 |
|------|------|
| prompt 被安全系统拒绝 | `"type": "invalid_request_error"`, `"message": "Your prompt was rejected by the safety system"` |
| 图像过大 | `"message": "Image file is too large. Maximum file size is 4MB."` |
| 遮罩尺寸不匹配 | `"message": "Mask must have the same dimensions as the image."` |

---

## 10. 供应商适配器协议差异

team-api Relay 层已实现以下供应商的图像生成适配器。各供应商使用不同的上游端点和参数格式：

### 10.1 端点映射表

| 供应商 | RelayModeImagesGenerations 对应的上游端点 | 说明 |
|--------|------------------------------------------|------|
| **OpenAI** | `POST {baseURL}/v1/images/generations` | 原生 OpenAI 格式 |
| **Ali（阿里云百炼）** | `POST {baseURL}/api/v1/services/aigc/text2image/image-synthesis` | DashScope 原生端点 |
| **Baidu V2（百度文心）** | `POST {baseURL}/v2/images/generations` | 百度兼容格式 |
| **Zhipu（智谱 AI）** | `POST {baseURL}/api/paas/v4/images/generations` | 智谱 CogView 端点 |
| **SiliconFlow** | `POST {baseURL}/v1/images/generations` | 兼容 OpenAI 格式 |
| **xAI（Grok）** | `POST {baseURL}/v1/images/generations` | 兼容 OpenAI 格式 |
| **Mistral** | `POST {baseURL}/v1/images/generations` | 兼容 OpenAI 格式 |
| **Cloudflare（Workers AI）** | `POST {baseURL}/accounts/{account_id}/ai/models/@cf/.../images/generations` | Cloudflare AI 端点 |
| **Replicate** | `POST {baseURL}/v1/images/generations` | 兼容 OpenAI 格式 |
| **Jimeng（即梦）** | `POST {baseURL}/v2/images/generations` | 字节跳动即梦专属端点，仅支持图像生成 |

### 10.2 参数转换注意事项

| 供应商 | 参数差异 |
|--------|---------|
| **Ali** | 请求体需要转换为 DashScope 格式（`input.prompt`、`parameters.size` 等），响应需要从 DashScope 格式转回 OpenAI 格式 |
| **SiliconFlow** | `size` 字段需重命名为 `image_size` |
| **Jimeng** | 仅支持 `RelayModeImagesGenerations`，不支持其他模式 |
| **Replicate** | 仅支持 `RelayModeImagesGenerations`，不支持其他模式 |

---

## 11. 协议转换注意事项

### 11.1 当前实现状态

team-api 中图像生成的处理链路：

```
客户端请求 POST /v1/images/generations
  → Path2RelayMode 识别为 RelayModeImagesGenerations
    → HandleImagesGenerations 入口
      → RelayHandler 统一调度
        → 渠道适配器 GetRequestURL（构建上游 URL）
        → 渠道适配器 ConvertRequest（转换请求参数）
        → 渠道适配器 DoRequest（发送上游请求）
        → 渠道适配器 DoResponse（处理上游响应）
```

### 11.2 DTO 定义（当前）

当前 `relay/dto/usage.go` 中的图像相关结构体：

```go
// ImageRequest 图像生成请求
type ImageRequest struct {
    Model          string `json:"model"`
    Prompt         string `json:"prompt"`
    N              *int   `json:"n,omitempty"`
    Size           string `json:"size,omitempty"`
    Quality        string `json:"quality,omitempty"`
    ResponseFormat string `json:"response_format,omitempty"`
    Style          string `json:"style,omitempty"`
    User           string `json:"user,omitempty"`
}

// ImageResponse 图像生成响应
type ImageResponse struct {
    Created int64       `json:"created"`
    Data    []ImageData `json:"data"`
}

// ImageData 单个图像数据
type ImageData struct {
    URL           string `json:"url,omitempty"`
    B64JSON       string `json:"b64_json,omitempty"`
    RevisedPrompt string `json:"revised_prompt,omitempty"`
}
```

### 11.3 待完善项

1. **GPT Image 新参数未覆盖**：当前 `ImageRequest` 缺少 `background`、`output_format`、`output_compression`、`partial_images`、`stream`、`moderation` 字段
2. **GPT Image 增强响应未覆盖**：当前 `ImageResponse` 缺少 `background`、`output_format`、`size`、`quality`、`usage` 字段
3. **图像编辑端点未注册**：`/v1/images/edits` 路径尚未在 `Path2RelayMode` 中注册
4. **图像变体端点未注册**：`/v1/images/variations` 路径尚未在 `Path2RelayMode` 中注册
5. **流式图像响应未实现**：GPT Image 的 SSE 流式响应需要专门的流处理器
6. **Usage 统计缺失**：当前 `handleImageResponse` 返回空的 `Usage{}`，GPT Image 模型应返回实际的 token 用量
7. **计费缺失**：DALL-E 和 GPT Image 的定价模型不同（DALL-E 按次计费，GPT Image 按 token 计费），需要区分

### 11.4 DTO 扩展建议

为支持 GPT Image 的完整功能，建议扩展 DTO：

```go
// ImageRequest 图像生成请求（扩展版）
type ImageRequest struct {
    Model             string `json:"model"`
    Prompt            string `json:"prompt"`
    N                 *int   `json:"n,omitempty"`
    Size              string `json:"size,omitempty"`
    Quality           string `json:"quality,omitempty"`
    ResponseFormat    string `json:"response_format,omitempty"`
    Style             string `json:"style,omitempty"`
    Background        string `json:"background,omitempty"`        // GPT Image: transparent/opaque/auto
    OutputFormat      string `json:"output_format,omitempty"`     // GPT Image: png/jpeg/webp
    OutputCompression *int   `json:"output_compression,omitempty"` // GPT Image: 0-100
    PartialImages     *int   `json:"partial_images,omitempty"`    // GPT Image: 0-3
    Stream            *bool  `json:"stream,omitempty"`            // GPT Image: 流式
    Moderation        string `json:"moderation,omitempty"`        // GPT Image: auto/low
    User              string `json:"user,omitempty"`
}

// ImageResponse 图像生成响应（扩展版）
type ImageResponse struct {
    Created      int64         `json:"created"`
    Data         []ImageData   `json:"data"`
    Background   string        `json:"background,omitempty"`
    OutputFormat string        `json:"output_format,omitempty"`
    Size         string        `json:"size,omitempty"`
    Quality      string        `json:"quality,omitempty"`
    Usage        *ImageUsage   `json:"usage,omitempty"`
}

// ImageUsage GPT Image token 用量
type ImageUsage struct {
    TotalTokens      int                `json:"total_tokens"`
    InputTokens      int                `json:"input_tokens"`
    OutputTokens     int                `json:"output_tokens"`
    InputTokenDetails *ImageTokenDetails `json:"input_tokens_details,omitempty"`
}

// ImageTokenDetails GPT Image 输入 token 细分
type ImageTokenDetails struct {
    TextTokens  int `json:"text_tokens"`
    ImageTokens int `json:"image_tokens"`
}
```

### 11.5 新增 RelayMode 建议

为支持图像编辑和变体端点，建议在 `relay/constant/relay_mode.go` 中新增：

```go
RelayModeImagesEdits       // /v1/images/edits
RelayModeImagesVariations  // /v1/images/variations
```

并在 `Path2RelayMode` 中添加路径匹配：

```go
case strings.HasSuffix(path, "/images/edits"):
    result = RelayModeImagesEdits
case strings.HasSuffix(path, "/images/variations"):
    result = RelayModeImagesVariations
```

### 11.6 计费差异

| 模型 | 计费方式 | 说明 |
|------|---------|------|
| `dall-e-2` | 按次计费 | 根据尺寸和数量定价，与 token 无关 |
| `dall-e-3` | 按次计费 | quality=hd 价格高于 standard |
| `gpt-image-1` | 按 token 计费 | 使用 `usage` 字段中的 token 数计费 |
| `gpt-image-1-mini` | 按 token 计费 | 价格低于 gpt-image-1 |
| `gpt-image-1.5` | 按 token 计费 | 价格低于 gpt-image-1 |

Relay 层的 `BillingProvider` 需要根据模型类型选择不同的计费策略：DALL-E 模型使用固定价格计费，GPT Image 模型使用 token 计费。当前 `handleImageResponse` 返回空 Usage，需要区分处理。
