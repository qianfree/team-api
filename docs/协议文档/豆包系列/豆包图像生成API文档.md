# 豆包图像生成 API 文档

> 火山引擎方舟（Volcengine Ark）平台 — 豆包图像生成模型 API 完整参考
>
> 基于火山引擎方舟平台官方文档整理，覆盖 Seedream 文生图、Seedance 图生图、Seed Thinking 等豆包图像生成模型的完整 API 规范。
>
> 官方文档入口：https://www.volcengine.com/docs/6791/1399418

---

## 目录

- [1. 概述](#1-概述)
- [2. 认证方式](#2-认证方式)
- [3. API 端点列表](#3-api-端点列表)
- [4. 图像生成（文生图）接口详细参数](#4-图像生成文生图接口详细参数)
- [5. 图像编辑（图生图）接口详细参数](#5-图像编辑图生图接口详细参数)
- [6. 支持模型与参数差异](#6-支持模型与参数差异)
- [7. 响应格式](#7-响应格式)
- [8. 错误码与错误处理](#8-错误码与错误处理)
- [9. 请求与响应示例](#9-请求与响应示例)
- [10. OpenAI 兼容接口说明](#10-openai-兼容接口说明)
- [11. team-api 适配器实现说明](#11-team-api-适配器实现说明)

---

## 1. 概述

### 1.1 平台简介

豆包（Doubao）是字节跳动推出的 AI 大模型品牌，其图像生成能力通过**火山引擎方舟（Volcengine Ark）**平台提供 API 服务。方舟平台是火山引擎的一站式大模型推理服务平台，支持文本生成、图像生成、语音合成等多种 AI 能力。

豆包图像生成目前提供以下核心模型系列：

| 模型系列 | 模型名 | 功能定位 |
|---------|--------|---------|
| Seedream | `doubao-seedream-*` | 文生图（Text-to-Image），根据文本描述生成图像 |
| Seedance | `doubao-seedance-*` | 图生图（Image-to-Image），基于参考图+文本描述生成新图像 |
| Seed Thinking | `doubao-seed-*-thinking-*` | 带推理增强的图像生成，先思考再生成 |

### 1.2 API 基本信息总览

| 项目 | 值 |
|------|------|
| 平台名称 | 火山引擎方舟（Volcengine Ark） |
| 基础 URL | `https://ark.cn-beijing.volces.com` |
| API 版本 | v3 |
| 协议风格 | RESTful JSON |
| 兼容性 | 兼容 OpenAI API 格式 |
| 文档 ID | 6791（火山引擎文档中心） |

### 1.3 核心特性

- **OpenAI 兼容**：方舟平台的图像生成接口完全兼容 OpenAI Images API 格式，客户端可直接使用 OpenAI SDK 调用
- **多模型支持**：支持 Seedream（文生图）、Seedance（图生图）等多种图像生成模型
- **多种输出格式**：支持 URL 返回和 Base64 编码返回
- **多尺寸支持**：支持正方形、横向、纵向等多种尺寸
- **流式生成**：部分模型支持流式返回中间结果
- **Bot 模型**：支持 `bot-` 前缀的 Bot 模型，使用专用端点

---

## 2. 认证方式

### 2.1 API Key 认证

火山引擎方舟平台使用 Bearer Token 认证方式，在请求头中携带 API Key：

```http
Authorization: Bearer <your-api-key>
Content-Type: application/json
```

### 2.2 API Key 获取

1. 登录火山引擎控制台：https://console.volcengine.com/ark
2. 进入方舟平台，在「API Key 管理」中创建新的 API Key
3. 创建接入点（Endpoint），获取模型对应的 Endpoint ID
4. API Key 格式示例：`ark-xxxxxxxxxxxxxxxxxxxxxxxx`

### 2.3 认证参数表

| 参数 | 位置 | 格式 | 必填 | 说明 |
|------|------|------|------|------|
| `Authorization` | Header | `Bearer <api-key>` | 是 | API Key 认证令牌 |
| `Content-Type` | Header | `application/json` | 是 | 请求体格式 |

### 2.4 认证错误响应

当 API Key 无效或缺失时，返回以下错误：

```json
{
  "error": {
    "type": "authentication_error",
    "message": "Incorrect API key provided",
    "code": "invalid_api_key"
  }
}
```

HTTP 状态码：`401 Unauthorized`

### 2.5 注意事项

- API Key 必须妥善保管，不可暴露在客户端代码中
- 每个 API Key 关联一个方舟平台账户，所有调用产生的费用由该账户承担
- API Key 支持在方舟控制台进行轮换和吊销
- 若使用 team-api 网关，API Key 存储在渠道配置中，格式为 `ark-xxxxx`

---

## 3. API 端点列表

### 3.1 方舟平台原生端点

| 方法 | 端点路径 | 功能 | 支持模型 | 请求格式 |
|------|---------|------|---------|---------|
| POST | `/api/v3/images/generations` | 文生图 + 图生图 | Seedream, Seedance | JSON |
| POST | `/api/v3/chat/completions` | 文本对话 | Doubao-pro, Doubao-lite 等 | JSON |
| POST | `/api/v3/embeddings` | 文本向量化 | Doubao-embedding | JSON |
| POST | `/api/v3/bots/chat/completions` | Bot 对话 | bot-* 模型 | JSON |
| POST | `/api/v3/rerank` | 重排序 | 支持的模型 | JSON |
| POST | `/api/v3/responses` | Responses API | 支持的模型 | JSON |

### 3.2 图像生成端点详解

方舟平台的图像生成统一使用 `/api/v3/images/generations` 端点，**文生图和图生图共用同一端点**。

**基础 URL**：`https://ark.cn-beijing.volces.com`

**完整端点 URL**：`https://ark.cn-beijing.volces.com/api/v3/images/generations`

### 3.3 端点功能矩阵

| 功能 | 端点 | 支持的模型系列 | 说明 |
|------|------|--------------|------|
| 文生图（Text-to-Image） | `/api/v3/images/generations` | Seedream | 仅提供 prompt，模型根据文本生成图像 |
| 图生图（Image-to-Image） | `/api/v3/images/generations` | Seedance | 提供 prompt + 参考图像，模型结合两者生成新图像 |
| 带推理图像生成 | `/api/v3/images/generations` | Seed Thinking | 带思维链的图像生成，先推理再出图 |

---

## 4. 图像生成（文生图）接口详细参数

### 4.1 接口基本信息

| 项目 | 值 |
|------|------|
| 端点 | `POST /api/v3/images/generations` |
| Content-Type | `application/json` |
| 认证方式 | Bearer Token |
| 响应格式 | JSON |

### 4.2 完整请求参数表

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | **是** | — | 模型 ID 或 Endpoint ID。例如 `doubao-seedream-4-0-250828`，也支持使用方舟平台的接入点 ID（`ep-xxxxxxxx`） |
| `prompt` | string | **是** | — | 图像描述文本，即文生图的提示词。支持中英文描述，建议使用详细具体的描述以获得更好的生成效果。Seedream 模型最长支持约 4000 字符 |
| `n` | integer | 否 | `1` | 生成图像数量。通常支持 1-4 张。部分模型仅支持 1 张 |
| `size` | string | 否 | `1024x1024` | 输出图像尺寸，格式为 `{width}x{height}`。支持的尺寸取决于模型，详见下方尺寸表 |
| `response_format` | string | 否 | `url` | 图像返回格式：`url`（返回临时下载链接）或 `b64_json`（返回 Base64 编码） |
| `seed` | integer | 否 | 随机 | 随机种子。相同种子 + 相同参数可复现近似结果 |
| `guidance_scale` | float | 否 | 模型默认 | 引导比例（CFG Scale），控制图像与提示词的一致性。值越高越严格遵循提示词，值越低创意自由度越高 |
| `user` | string | 否 | — | 终端用户标识符，用于监控和滥用检测 |

### 4.3 Seedream 模型支持的尺寸

**doubao-seedream-4-0-250828（Seedream 4.0）**：

| 尺寸 | 宽高比 | 说明 |
|------|--------|------|
| `1024x1024` | 1:1 | 正方形 |
| `1536x1024` | 3:2 | 横向 |
| `1024x1536` | 2:3 | 纵向 |
| `1280x720` | 16:9 | 宽屏横向 |
| `720x1280` | 9:16 | 宽屏纵向（手机竖屏） |

**doubao-seedream-3-0-t2i-250415（Seedream 3.0 文生图）**：

| 尺寸 | 宽高比 | 说明 |
|------|--------|------|
| `1024x1024` | 1:1 | 正方形 |
| `1024x1792` | 9:16 | 纵向 |
| `1792x1024` | 16:9 | 横向 |

### 4.4 文生图请求示例

**基本文生图请求**：

```json
{
  "model": "doubao-seedream-4-0-250828",
  "prompt": "一只橘色的猫咪坐在窗台上，阳光从窗外照射进来，温暖的氛围，高清摄影风格",
  "n": 1,
  "size": "1024x1024",
  "response_format": "url"
}
```

**使用 Endpoint ID 的请求**：

```json
{
  "model": "ep-20250415xxxxx-xxxxx",
  "prompt": "A beautiful sunset over a mountain lake, reflection in the water, photorealistic",
  "n": 1,
  "size": "1536x1024",
  "response_format": "b64_json",
  "seed": 42
}
```

**多图生成请求**：

```json
{
  "model": "doubao-seedream-4-0-250828",
  "prompt": "四个季节的风景：春天樱花、夏天海滩、秋天枫叶、冬天雪景",
  "n": 4,
  "size": "1024x1024",
  "response_format": "url"
}
```

### 4.5 Seedream 模型 prompt 技巧

| 技巧 | 示例 | 说明 |
|------|------|------|
| 具体描述主体 | "一只金色的拉布拉多犬在草地上奔跑" | 明确描述主体、动作、场景 |
| 指定风格 | "水彩画风格"、"赛博朋克风格"、"日本动漫风格" | 在 prompt 末尾添加风格关键词 |
| 指定视角 | "俯视角度"、"特写镜头"、"广角全景" | 控制拍摄/生成视角 |
| 指定光照 | "自然光"、"黄金时段"、"霓虹灯光" | 描述光照条件 |
| 指定色彩 | "暖色调"、"冷色调"、"莫兰迪色系" | 控制整体色彩倾向 |
| 中文支持 | 直接使用中文描述 | Seedream 原生支持中文 prompt |

---

## 5. 图像编辑（图生图）接口详细参数

### 5.1 接口基本信息

| 项目 | 值 |
|------|------|
| 端点 | `POST /api/v3/images/generations` |
| Content-Type | `application/json` |
| 认证方式 | Bearer Token |
| 说明 | 图生图与文生图共用同一端点，通过传入参考图像区分 |

### 5.2 完整请求参数表

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | **是** | — | 图生图模型 ID，如 `doubao-seedance-1-0-pro-250528` |
| `prompt` | string | **是** | — | 编辑指令文本，描述期望的编辑效果 |
| `image` | string 或 array | **是** | — | 参考图像。支持传入图像 URL 或 Base64 编码的图像数据。部分模型支持多张参考图（最多 16 张） |
| `n` | integer | 否 | `1` | 生成图像数量 |
| `size` | string | 否 | `1024x1024` | 输出图像尺寸 |
| `response_format` | string | 否 | `url` | 返回格式：`url` 或 `b64_json` |
| `strength` | float | 否 | 模型默认 | 编辑强度（0.0-1.0）。值越低越接近原图，值越高变化越大 |
| `guidance_scale` | float | 否 | 模型默认 | 引导比例，控制编辑结果与提示词的一致性 |
| `seed` | integer | 否 | 随机 | 随机种子 |
| `user` | string | 否 | — | 终端用户标识 |

### 5.3 参考图像格式

参考图像（`image` 字段）支持以下格式：

**方式一：图像 URL**

```json
{
  "image": "https://example.com/photo.jpg"
}
```

**方式二：Base64 编码**

```json
{
  "image": "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQ..."
}
```

**方式三：多张参考图（数组格式）**

```json
{
  "image": [
    {
      "type": "image_url",
      "url": "https://example.com/photo1.jpg"
    },
    {
      "type": "image_url",
      "url": "data:image/png;base64,iVBORw0KGgo..."
    }
  ]
}
```

### 5.4 Seedance 模型支持的尺寸

**doubao-seedance-1-0-pro-250528（Seedance 1.0 Pro）**：

| 尺寸 | 宽高比 | 说明 |
|------|--------|------|
| `1024x1024` | 1:1 | 正方形 |
| `1536x1024` | 3:2 | 横向 |
| `1024x1536` | 2:3 | 纵向 |

### 5.5 图生图请求示例

**基本图生图请求**：

```json
{
  "model": "doubao-seedance-1-0-pro-250528",
  "prompt": "将背景替换为海滩日落场景，保持人物不变",
  "image": "https://example.com/portrait.jpg",
  "n": 1,
  "size": "1024x1024",
  "response_format": "url",
  "strength": 0.7
}
```

**使用 Base64 参考图的请求**：

```json
{
  "model": "doubao-seedance-1-0-pro-250528",
  "prompt": "Add a red scarf to the person in the image, winter style",
  "image": "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQ...",
  "size": "1536x1024",
  "response_format": "b64_json"
}
```

**多参考图请求**：

```json
{
  "model": "doubao-seedance-1-0-pro-250528",
  "prompt": "融合两张图片的风格，生成一幅新的艺术作品",
  "image": [
    {
      "type": "image_url",
      "url": "https://example.com/style1.jpg"
    },
    {
      "type": "image_url",
      "url": "https://example.com/style2.jpg"
    }
  ],
  "n": 1,
  "size": "1024x1024"
}
```

### 5.6 图生图使用场景

| 场景 | prompt 示例 | 说明 |
|------|-----------|------|
| 风格转换 | "将这张照片转换为梵高风格" | 保留内容，改变艺术风格 |
| 背景替换 | "将背景替换为星空" | 改变场景背景 |
| 细节编辑 | "给人物添加一顶红色帽子" | 局部修改添加元素 |
| 图像修复 | "修复图片左侧的划痕" | 修复图像缺陷 |
| 色彩调整 | "将整体色调调整为暖色调" | 改变色彩风格 |

---

## 6. 支持模型与参数差异

### 6.1 模型列表

| 模型 ID | 模型系列 | 功能 | 发布日期（版本号） | 说明 |
|---------|---------|------|-----------------|------|
| `doubao-seedream-4-0-250828` | Seedream 4.0 | 文生图 | 2025-08-28 | 最新版本文生图模型，画质和文字渲染能力大幅提升 |
| `seedream-4-0-250828` | Seedream 4.0（短名） | 文生图 | 2025-08-28 | 同上，短名版本 |
| `doubao-seedream-3-0-t2i-250415` | Seedream 3.0 | 文生图 | 2025-04-15 | 第三代文生图模型 |
| `doubao-seedance-1-0-pro-250528` | Seedance 1.0 Pro | 图生图 | 2025-05-28 | 专业版图生图模型，支持多参考图 |
| `seedance-1-0-pro-250528` | Seedance 1.0 Pro（短名） | 图生图 | 2025-05-28 | 同上，短名版本 |
| `doubao-seed-1-6-thinking-250715` | Seed 1.6 Thinking | 带推理图像生成 | 2025-07-15 | 带思维链的图像生成模型 |
| `seed-1-6-thinking-250715` | Seed 1.6 Thinking（短名） | 带推理图像生成 | 2025-07-15 | 同上，短名版本 |

### 6.2 模型命名规则

豆包图像生成模型遵循统一的命名规范：

```
doubao-{系列名}-{版本号}-{功能标识}-{发布日期}
```

| 部分 | 说明 | 示例 |
|------|------|------|
| `doubao-` | 品牌前缀（可选，短名不含此前缀） | `doubao-seedream-4-0-250828` |
| `seedream` | 模型系列：文生图 | seedream = see + dream |
| `seedance` | 模型系列：图生图 | seedance = see + dance |
| `seed` | 模型系列：通用 | 包含 thinking 能力 |
| `4-0` | 主版本号.次版本号 | 4.0 版本 |
| `t2i` | 功能标识：Text-to-Image | 仅文生图模型 |
| `pro` | 功能标识：专业版 | 图生图专业版 |
| `thinking` | 功能标识：推理增强 | 带思维链的模型 |
| `250828` | 发布日期 YYMMDD | 2025年8月28日 |

### 6.3 各模型参数差异对比

| 参数 | Seedream 4.0 | Seedream 3.0 | Seedance 1.0 Pro | Seed 1.6 Thinking |
|------|-------------|-------------|-----------------|-------------------|
| **文生图** | 支持 | 支持 | 不支持 | 支持 |
| **图生图** | 不支持 | 不支持 | 支持 | 支持 |
| **prompt** | 必填 | 必填 | 必填 | 必填 |
| **image 参数** | 不支持 | 不支持 | 必填 | 可选 |
| **n（数量）** | 1-4 | 1 | 1-4 | 1 |
| **尺寸 1024x1024** | 支持 | 支持 | 支持 | 支持 |
| **尺寸 1536x1024** | 支持 | 不支持 | 支持 | 支持 |
| **尺寸 1024x1536** | 支持 | 不支持 | 支持 | 支持 |
| **尺寸 1792x1024** | 不支持 | 支持 | 不支持 | 不支持 |
| **尺寸 1280x720** | 支持 | 不支持 | 不支持 | 支持 |
| **尺寸 720x1280** | 支持 | 不支持 | 不支持 | 支持 |
| **response_format** | url/b64_json | url/b64_json | url/b64_json | url/b64_json |
| **seed** | 支持 | 支持 | 支持 | 支持 |
| **guidance_scale** | 支持 | 支持 | 支持 | 支持 |
| **strength** | 不支持 | 不支持 | 支持 | 支持 |
| **流式响应** | 不支持 | 不支持 | 不支持 | 不支持 |

### 6.4 模型性能特征

| 模型 | 典型响应时间 | 图像质量 | 中文字体渲染 | 适用场景 |
|------|------------|---------|-------------|---------|
| Seedream 4.0 | 5-15 秒 | 极高 | 优秀 | 高质量文生图、商业海报、带文字的图片 |
| Seedream 3.0 | 3-10 秒 | 高 | 良好 | 通用文生图 |
| Seedance 1.0 Pro | 5-20 秒 | 极高 | 良好 | 图像编辑、风格迁移、细节修改 |
| Seed 1.6 Thinking | 10-30 秒 | 极高 | 优秀 | 复杂场景生成、需要推理的图像 |

---

## 7. 响应格式

### 7.1 成功响应结构

#### URL 格式响应（response_format: "url"）

```json
{
  "created": 1714051200,
  "data": [
    {
      "url": "https://ark-cn-beijing.volces.com/api/v3/files/xxxxxxxx"
    }
  ]
}
```

#### Base64 格式响应（response_format: "b64_json"）

```json
{
  "created": 1714051200,
  "data": [
    {
      "b64_json": "iVBORw0KGgoAAAANSUhEUgAA..."
    }
  ]
}
```

### 7.2 响应字段说明

| 字段 | 类型 | 必返回 | 说明 |
|------|------|--------|------|
| `created` | integer | 是 | 响应创建时间，Unix 时间戳（秒） |
| `data` | array | 是 | 图像数据数组，长度等于请求中的 `n` |
| `data[].url` | string | 条件 | 图像下载 URL（当 `response_format: "url"` 时返回）。URL 有效期为 1 小时 |
| `data[].b64_json` | string | 条件 | 图像的 Base64 编码字符串（当 `response_format: "b64_json"` 时返回） |
| `data[].revised_prompt` | string | 否 | 模型修改后的 prompt（部分模型可能返回） |

### 7.3 多图响应示例

当请求 `n > 1` 时，`data` 数组包含多个元素：

```json
{
  "created": 1714051200,
  "data": [
    {
      "url": "https://ark-cn-beijing.volces.com/api/v3/files/image-001"
    },
    {
      "url": "https://ark-cn-beijing.volces.com/api/v3/files/image-002"
    },
    {
      "url": "https://ark-cn-beijing.volces.com/api/v3/files/image-003"
    },
    {
      "url": "https://ark-cn-beijing.volces.com/api/v3/files/image-004"
    }
  ]
}
```

### 7.4 URL 有效期

- 通过 `response_format: "url"` 返回的图像 URL 有效期为 **1 小时**
- 过期后需重新调用 API 获取新 URL
- 建议在获取 URL 后及时下载并存储到自有对象存储（S3/OSS/COS）
- 若需持久化使用，推荐使用 `b64_json` 格式或配合对象存储服务

### 7.5 图像格式与大小

| 属性 | 说明 |
|------|------|
| 输出格式 | PNG（默认） |
| 色彩空间 | RGB（不支持 CMYK） |
| 位深度 | 8-bit |
| 典型文件大小 | 512x512 约 200-500KB；1024x1024 约 500KB-2MB；1536x1024 约 800KB-3MB |

---

## 8. 错误码与错误处理

### 8.1 错误响应格式

方舟平台的图像生成 API 使用 OpenAI 兼容的错误格式：

```json
{
  "error": {
    "type": "<error_type>",
    "message": "<错误描述>",
    "param": "<相关参数（可选）>",
    "code": "<错误代码（可选）>"
  }
}
```

### 8.2 HTTP 状态码与错误类型

| HTTP 状态码 | error.type | 说明 | 可能原因 |
|------------|-----------|------|---------|
| 400 | `invalid_request_error` | 请求参数错误 | 缺少必填字段、参数格式不正确、不支持的尺寸等 |
| 401 | `authentication_error` | 认证失败 | API Key 无效、缺失或已过期 |
| 402 | `insufficient_quota` | 额度不足 | 账户余额不足或配额已用完 |
| 403 | `permission_error` | 权限不足 | 无权使用指定模型 |
| 404 | `not_found_error` | 资源不存在 | 模型 ID 或 Endpoint ID 不存在 |
| 429 | `rate_limit_error` | 请求频率超限 | 超过 API 调用频率限制 |
| 500 | `server_error` | 服务器内部错误 | 方舟平台内部异常 |
| 503 | `service_unavailable` | 服务不可用 | 模型服务暂时不可用 |

### 8.3 常见业务错误

#### 8.3.1 认证相关错误

**API Key 无效**：

```json
{
  "error": {
    "type": "authentication_error",
    "message": "Incorrect API key provided: ark-xxxx...xxxx. You can find your API key at https://console.volcengine.com/ark.",
    "code": "invalid_api_key"
  }
}
```

**API Key 缺失**：

```json
{
  "error": {
    "type": "authentication_error",
    "message": "You didn't provide an API key. You need to provide your API key in the Authorization header.",
    "code": "missing_api_key"
  }
}
```

#### 8.3.2 参数相关错误

**不支持的尺寸**：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid size: 2048x2048. Supported sizes for doubao-seedream-4-0-250828 are: 1024x1024, 1536x1024, 1024x1536, 1280x720, 720x1280",
    "param": "size",
    "code": "invalid_size"
  }
}
```

**模型不存在**：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Model not found: doubao-seedream-xxx",
    "param": "model",
    "code": "model_not_found"
  }
}
```

**prompt 过长**：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Prompt length exceeds maximum limit of 4000 characters",
    "param": "prompt",
    "code": "prompt_too_long"
  }
}
```

**缺少必填参数**：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Required parameter 'prompt' is missing",
    "param": "prompt",
    "code": "missing_required_param"
  }
}
```

#### 8.3.3 限流相关错误

**请求频率超限**：

```json
{
  "error": {
    "type": "rate_limit_error",
    "message": "Rate limit reached. Please retry after 60 seconds.",
    "code": "rate_limit_exceeded"
  }
}
```

响应头会包含限流信息：

```http
HTTP/1.1 429 Too Many Requests
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1714051260
Retry-After: 60
```

#### 8.3.4 内容审核错误

**Prompt 被安全系统拒绝**：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Your prompt was rejected by our safety system. It may contain content that is not allowed.",
    "code": "content_filtered"
  }
}
```

#### 8.3.5 服务端错误

**模型服务不可用**：

```json
{
  "error": {
    "type": "server_error",
    "message": "The model service is temporarily unavailable. Please try again later.",
    "code": "service_unavailable"
  }
}
```

### 8.4 错误处理最佳实践

| 策略 | 说明 |
|------|------|
| **重试机制** | 对 429（限流）和 500/503（服务端错误）实现指数退避重试，建议最大重试 3 次 |
| **超时设置** | 图像生成请求建议设置 60-120 秒超时 |
| **错误日志** | 记录完整的 error.type、error.code 和 error.message 用于问题排查 |
| **降级策略** | 当主模型不可用时，可切换到备用模型（如从 Seedream 4.0 降级到 3.0） |
| **参数校验** | 在客户端预先校验参数，避免不必要的 API 调用失败 |

---

## 9. 请求与响应示例

### 9.1 cURL 示例

#### 9.1.1 文生图（基本）

```bash
curl -X POST https://ark.cn-beijing.volces.com/api/v3/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ARK_API_KEY" \
  -d '{
    "model": "doubao-seedream-4-0-250828",
    "prompt": "一只可爱的猫咪在雨中撑着小伞",
    "n": 1,
    "size": "1024x1024",
    "response_format": "url"
  }'
```

**响应**：

```json
{
  "created": 1714051200,
  "data": [
    {
      "url": "https://ark-cn-beijing.volces.com/api/v3/files/fg-abc123def456"
    }
  ]
}
```

#### 9.1.2 文生图（Base64 返回）

```bash
curl -X POST https://ark.cn-beijing.volces.com/api/v3/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ARK_API_KEY" \
  -d '{
    "model": "doubao-seedream-4-0-250828",
    "prompt": "A futuristic city skyline at night, neon lights, cyberpunk style, ultra detailed",
    "n": 1,
    "size": "1536x1024",
    "response_format": "b64_json"
  }'
```

**响应**：

```json
{
  "created": 1714051200,
  "data": [
    {
      "b64_json": "iVBORw0KGgoAAAANSUhEUgAAB9AAAASwCAYAAAD..."
    }
  ]
}
```

#### 9.1.3 图生图

```bash
curl -X POST https://ark.cn-beijing.volces.com/api/v3/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ARK_API_KEY" \
  -d '{
    "model": "doubao-seedance-1-0-pro-250528",
    "prompt": "将这张照片变成油画风格",
    "image": "https://example.com/landscape.jpg",
    "n": 1,
    "size": "1024x1024",
    "response_format": "url",
    "strength": 0.6
  }'
```

**响应**：

```json
{
  "created": 1714051200,
  "data": [
    {
      "url": "https://ark-cn-beijing.volces.com/api/v3/files/fg-xyz789ghi012"
    }
  ]
}
```

#### 9.1.4 使用 Endpoint ID

方舟平台支持通过接入点 ID（Endpoint ID）指定模型：

```bash
curl -X POST https://ark.cn-beijing.volces.com/api/v3/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ARK_API_KEY" \
  -d '{
    "model": "ep-20250415xxxxx-xxxxx",
    "prompt": "中国水墨画风格的山水，远处有飞鸟",
    "n": 1,
    "size": "1024x1536",
    "response_format": "url",
    "seed": 12345
  }'
```

### 9.2 Python SDK 示例

#### 9.2.1 使用 OpenAI SDK（推荐）

由于方舟平台兼容 OpenAI API 格式，可以直接使用 OpenAI Python SDK：

```python
from openai import OpenAI

# 创建客户端，指向方舟平台
client = OpenAI(
    api_key="ark-xxxxxxxxxxxxxxxx",
    base_url="https://ark.cn-beijing.volces.com/api/v3"
)

# 文生图
response = client.images.generate(
    model="doubao-seedream-4-0-250828",
    prompt="一只在樱花树下读书的女孩，水彩画风格",
    n=1,
    size="1024x1024"
)

# 获取生成的图像 URL
image_url = response.data[0].url
print(f"图像 URL: {image_url}")
```

#### 9.2.2 使用 Base64 格式

```python
import base64
from openai import OpenAI

client = OpenAI(
    api_key="ark-xxxxxxxxxxxxxxxx",
    base_url="https://ark.cn-beijing.volces.com/api/v3"
)

response = client.images.generate(
    model="doubao-seedream-4-0-250828",
    prompt="A majestic dragon flying over a medieval castle at sunset",
    n=1,
    size="1536x1024",
    response_format="b64_json"
)

# 解码并保存图像
image_data = base64.b64decode(response.data[0].b64_json)
with open("dragon.png", "wb") as f:
    f.write(image_data)
print("图像已保存为 dragon.png")
```

#### 9.2.3 图生图示例

```python
from openai import OpenAI

client = OpenAI(
    api_key="ark-xxxxxxxxxxxxxxxx",
    base_url="https://ark.cn-beijing.volces.com/api/v3"
)

# 图生图请求
response = client.images.generate(
    model="doubao-seedance-1-0-pro-250528",
    prompt="Convert this photo to Studio Ghibli animation style",
    image="https://example.com/photo.jpg",
    n=1,
    size="1024x1024"
)

print(f"生成图像 URL: {response.data[0].url}")
```

### 9.3 Go HTTP 请求示例

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type ImageRequest struct {
    Model         string `json:"model"`
    Prompt        string `json:"prompt"`
    N             int    `json:"n"`
    Size          string `json:"size"`
    ResponseFormat string `json:"response_format"`
}

type ImageResponse struct {
    Created int64       `json:"created"`
    Data    []ImageData `json:"data"`
}

type ImageData struct {
    URL     string `json:"url,omitempty"`
    B64JSON string `json:"b64_json,omitempty"`
}

func main() {
    reqBody := ImageRequest{
        Model:         "doubao-seedream-4-0-250828",
        Prompt:        "一只橘猫在秋天的落叶中玩耍",
        N:             1,
        Size:          "1024x1024",
        ResponseFormat: "url",
    }

    body, _ := json.Marshal(reqBody)

    req, _ := http.NewRequest("POST",
        "https://ark.cn-beijing.volces.com/api/v3/images/generations",
        bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer ark-xxxxxxxxxxxxxxxx")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    respBody, _ := io.ReadAll(resp.Body)

    var imgResp ImageResponse
    json.Unmarshal(respBody, &imgResp)

    fmt.Printf("图像 URL: %s\n", imgResp.Data[0].URL)
}
```

### 9.4 Node.js 示例

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  apiKey: 'ark-xxxxxxxxxxxxxxxx',
  baseURL: 'https://ark.cn-beijing.volces.com/api/v3',
});

async function generateImage() {
  const response = await client.images.generate({
    model: 'doubao-seedream-4-0-250828',
    prompt: 'A serene Japanese garden with cherry blossoms in full bloom',
    n: 1,
    size: '1024x1024',
  });

  console.log('图像 URL:', response.data[0].url);
}

generateImage();
```

---

## 10. OpenAI 兼容接口说明

### 10.1 兼容性概述

火山引擎方舟平台的图像生成 API **完全兼容 OpenAI Images API 格式**，这意味着：

- **请求格式**：与 OpenAI `/v1/images/generations` 完全相同
- **响应格式**：与 OpenAI 响应结构一致（`created` + `data` 数组）
- **错误格式**：与 OpenAI 错误结构一致（`error.type` + `error.message`）
- **SDK 兼容**：可直接使用 OpenAI 官方 SDK，只需修改 `base_url` 和 `api_key`

### 10.2 端点映射

| OpenAI 格式 | 方舟平台格式 | 说明 |
|------------|------------|------|
| `POST /v1/images/generations` | `POST /api/v3/images/generations` | 文生图/图生图 |
| `POST /v1/chat/completions` | `POST /api/v3/chat/completions` | 文本对话 |
| `POST /v1/embeddings` | `POST /api/v3/embeddings` | 文本向量化 |
| `POST /v1/audio/speech` | `POST /api/v3/audio/speech` | 语音合成（方舟平台使用 WebSocket 方式） |

### 10.3 使用 OpenAI SDK 的配置差异

| 配置项 | OpenAI 原生 | 方舟平台 |
|--------|-----------|---------|
| `base_url` | `https://api.openai.com/v1` | `https://ark.cn-beijing.volces.com/api/v3` |
| `api_key` | `sk-xxxxx` | `ark-xxxxx` |
| `model` | `dall-e-3`, `gpt-image-1` | `doubao-seedream-4-0-250828` 等 |

### 10.4 team-api 网关集成

team-api 作为 API 网关，接收 OpenAI 格式的请求，转发至方舟平台：

```
客户端 → POST /v1/images/generations (OpenAI 格式)
  → team-api Relay 层
    → Volcengine Adaptor
      → GET RequestURL: {baseURL}/api/v3/images/generations
      → SetupRequestHeader: Bearer {apiKey}
      → ConvertRequest: 模型名映射 + OpenAI 格式直通
      → DoRequest: 发送 HTTP 请求
      → DoResponse: 委托 OpenAI 适配器处理响应
```

### 10.5 不支持的 OpenAI 参数

以下 OpenAI Images API 参数在方舟平台图像生成中**不支持或行为不同**：

| 参数 | OpenAI 支持 | 方舟平台支持 | 说明 |
|------|-----------|------------|------|
| `quality` | 支持 | 不支持 | 方舟平台不区分图像质量等级 |
| `style` | 支持（dall-e-3） | 不支持 | 方舟平台不区分 vivid/natural 风格 |
| `background` | 支持（gpt-image-1） | 不支持 | 方舟平台不支持透明背景 |
| `output_format` | 支持（gpt-image-1） | 不支持 | 方舟平台默认输出 PNG |
| `output_compression` | 支持（gpt-image-1） | 不支持 | 不支持压缩率设置 |
| `partial_images` | 支持（gpt-image-1） | 不支持 | 不支持流式中间图 |
| `stream` | 支持（gpt-image-1） | 不支持 | 不支持 SSE 流式响应 |
| `moderation` | 支持（gpt-image-1） | 不支持 | 不支持审核严格度设置 |
| `mask` | 支持（dall-e-2 编辑） | 不支持 | 不支持遮罩编辑 |

### 10.6 方舟平台特有参数

方舟平台支持以下 OpenAI 标准中没有的参数：

| 参数 | 说明 |
|------|------|
| `seed` | 随机种子，用于可复现生成 |
| `guidance_scale` | 引导比例（CFG Scale） |
| `strength` | 图生图编辑强度 |
| `image` | 参考图像（图生图模式） |

---

## 11. team-api 适配器实现说明

### 11.1 当前实现状态

team-api 已实现火山引擎（Volcengine）适配器，位于 `relay/channel/volcengine/` 目录。

#### 文件结构

```
relay/channel/volcengine/
├── adaptor.go      # 适配器主文件
└── constants.go    # 常量定义（ChannelName）
```

#### 适配器核心代码

```go
// Adaptor 火山引擎（豆包）供应商适配器。
// OpenAI 兼容格式，bot- 前缀模型使用 bots 端点。
type Adaptor struct {
    info *common.RelayInfo
}
```

#### 当前支持的 RelayMode

| RelayMode | 上游端点 | 状态 |
|-----------|---------|------|
| `RelayModeChatCompletions` | `/api/v3/chat/completions` 或 `/api/v3/bots/chat/completions` | 已实现 |
| `RelayModeClaudeMessages` | 转换为 OpenAI 格式后走 chat completions | 已实现 |
| `RelayModeEmbeddings` | `/api/v3/embeddings` | 已实现 |
| `RelayModeImagesGenerations` | `/api/v3/images/generations` | 待完善 |
| `RelayModeImagesEdits` | `/api/v3/images/generations`（共用） | 待实现 |

### 11.2 图像生成适配流程

```
客户端请求 POST /v1/images/generations
  → Path2RelayMode 识别为 RelayModeImagesGenerations
    → Volcengine Adaptor.GetRequestURL()
      → 构建: {baseURL}/api/v3/images/generations
    → Volcengine Adaptor.SetupRequestHeader()
      → 设置: Authorization: Bearer {apiKey}
    → Volcengine Adaptor.ConvertRequest()
      → 非 OpenAI 格式先转换
      → 模型名映射（如需要）
    → Volcengine Adaptor.DoRequest()
      → 发送 HTTP POST 请求
    → Volcengine Adaptor.DoResponse()
      → 委托 openai.Adaptor 处理响应
```

### 11.3 已注册模型列表

在 `new-api/relay/channel/volcengine/constants.go` 中定义了支持模型的完整列表：

```go
var ModelList = []string{
    "Doubao-pro-128k",
    "Doubao-pro-32k",
    "Doubao-pro-4k",
    "Doubao-lite-128k",
    "Doubao-lite-32k",
    "Doubao-lite-4k",
    "Doubao-embedding",
    "doubao-seedream-4-0-250828",
    "seedream-4-0-250828",
    "doubao-seedance-1-0-pro-250528",
    "seedance-1-0-pro-250528",
    "doubao-seed-1-6-thinking-250715",
    "seed-1-6-thinking-250715",
}
```

### 11.4 关键实现细节

#### URL 构建

```go
// 文生图和图生图共用同一端点
// 参考: https://www.volcengine.com/docs/82379/1824121
case constant.RelayModeImagesGenerations, constant.RelayModeImagesEdits:
    return fmt.Sprintf("%s/api/v3/images/generations", baseUrl), nil
```

#### 认证方式

```go
// 标准 Bearer Token 认证
header.Set("Authorization", "Bearer "+info.ChannelMeta.ApiKey)
```

#### 请求体处理

```go
// 火山引擎兼容 OpenAI 格式，只需做模型名映射
// 非 OpenAI 格式先转换为 OpenAI
if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI {
    converted, err := openai.ConvertToOpenAI(requestBody, info)
    // ...
}
// 模型名映射
if info.ChannelMeta.IsModelMapped {
    // 替换 model 字段
}
```

#### 响应处理

```go
// 响应格式与 OpenAI 一致，委托 OpenAI 适配器处理
func (a *Adaptor) DoResponse(...) (*common.Usage, error) {
    delegate := &openai.Adaptor{}
    delegate.Init(info)
    return delegate.DoResponse(ctx, resp, info, writer)
}
```

### 11.5 与即梦（Jimeng）适配器的区别

| 特性 | Volcengine 适配器 | Jimeng 适配器 |
|------|------------------|--------------|
| 上游端点 | `/api/v3/images/generations` | `/v2/images/generations` |
| 基础 URL | `https://ark.cn-beijing.volces.com` | 自定义 Base URL |
| API Key 格式 | `ark-xxxxx`（单一 Token） | `accessKey\|secretKey`（竖线分隔） |
| 响应处理 | OpenAI 兼容格式 | 原生即梦格式需转换为 OpenAI |
| 支持模型 | Seedream, Seedance 等 | 即梦专属模型 |
| 功能范围 | 文生图 + 图生图 + 文本对话 + 嵌入等 | 仅图像生成 |

### 11.6 待完善项

1. **图像生成模式完善**：当前 `team-api/relay/channel/volcengine/adaptor.go` 的 `GetRequestURL` 未覆盖 `RelayModeImagesGenerations`，需要添加图像生成路径支持
2. **图生图参数支持**：需要在 `ConvertRequest` 中处理 `image` 字段的传递
3. **模型名映射扩展**：确保所有豆包图像生成模型的名称在渠道配置中正确映射
4. **计费适配**：豆包图像生成按次计费，需要在计费模块中添加对应的价格配置
5. **错误格式适配**：确认方舟平台的错误格式与 OpenAI 完全一致

### 11.7 计费方式

| 模型 | 计费方式 | 说明 |
|------|---------|------|
| Seedream 系列 | 按次计费 | 每次生成按固定价格计费，与 token 无关 |
| Seedance 系列 | 按次计费 | 图生图按次计费，价格通常高于文生图 |
| Seed Thinking | 按次计费 | 带推理的图像生成价格最高 |

team-api 的 Relay 层 `BillingProvider` 需要根据模型类型选择适当的计费策略。图像生成模型使用固定价格计费，而非 token 计费。在 `handleImageResponse` 中需要正确返回计费信息。

### 11.8 参考链接

| 资源 | URL |
|------|-----|
| 方舟平台控制台 | https://console.volcengine.com/ark |
| API 文档首页 | https://www.volcengine.com/docs/6791/1399418 |
| 图像生成文档 | https://www.volcengine.com/docs/82379/1824121 |
| API Key 管理 | https://console.volcengine.com/ark/region:ark+cn-beijing/apiKey |
| 模型广场 | https://console.volcengine.com/ark/region:ark+cn-beijing/model |
| SDK 文档 | https://www.volcengine.com/docs/6791/1399820 |
| 定价说明 | https://www.volcengine.com/docs/6791/1361753 |

---

> **文档版本**：v1.0
>
> **最后更新**：2026-04-19
>
> **适用范围**：team-api 项目 Relay 层豆包图像生成适配器开发参考
