# 通义万相图像生成 API 文档

> 阿里云百炼（DashScope）平台 — 通义万相（Wanx）图像生成模型 API 完整参考
>
> 基于阿里云大模型服务平台百炼官方文档整理，覆盖万相文生图、涂鸦作画、图像局部重绘、人像风格重绘、图像编辑等通义万相系列图像生成模型的完整 API 规范。
>
> 官方文档入口：https://help.aliyun.com/zh/model-studio/developer-reference/image-generation

---

## 目录

- [1. 概述](#1-概述)
- [2. 认证方式](#2-认证方式)
- [3. API 端点列表](#3-api-端点列表)
- [4. 文生图接口详细参数](#4-文生图接口详细参数)
- [5. 涂鸦作画接口详细参数](#5-涂鸦作画接口详细参数)
- [6. 图像局部重绘接口详细参数](#6-图像局部重绘接口详细参数)
- [7. 人像风格重绘接口详细参数](#7-人像风格重绘接口详细参数)
- [8. 通用图像编辑接口详细参数](#8-通用图像编辑接口详细参数)
- [9. 同步与异步调用方式](#9-同步与异步调用方式)
- [10. 响应格式](#10-响应格式)
- [11. 错误码与错误处理](#11-错误码与错误处理)
- [12. 请求与响应示例](#12-请求与响应示例)
- [13. OpenAI 兼容接口说明](#13-openai-兼容接口说明)
- [14. team-api 适配器实现说明](#14-team-api-适配器实现说明)

---

## 1. 概述

### 1.1 平台简介

通义万相（Tongyi Wanxiang / Wanx）是阿里巴巴推出的 AI 图像生成模型系列，通过**阿里云大模型服务平台百炼（DashScope）**提供 API 服务。百炼平台是阿里云的一站式大模型推理服务平台，支持文本生成、图像生成、语音合成、视频生成等多种 AI 能力。

通义万相系列覆盖从文生图、图生图、涂鸦作画、人像风格重绘、图像局部重绘到通用图像编辑等多种图像生成和编辑场景，是目前国内功能最全面的 AI 图像生成模型系列之一。

### 1.2 API 基本信息总览

| 项目 | 值 |
|------|------|
| 平台名称 | 阿里云大模型服务平台百炼（DashScope） |
| 基础 URL | `https://dashscope.aliyuncs.com` |
| API 版本 | v1 |
| 协议风格 | RESTful JSON |
| 兼容模式 | 支持 OpenAI 兼容接口（`/compatible-mode/v1/`） |
| 控制台地址 | https://bailian.console.aliyun.com |

### 1.3 支持模型完整列表

通义万相系列及百炼平台支持的全部图像生成模型如下：

| 模型分类 | 模型名称（model 参数值） | 功能定位 |
|---------|------------------------|---------|
| **万相-文生图** | `wanx-v2` | 万相文生图 V2 版本，根据文本描述生成高质量图像 |
| **万相-文生图** | `wanx-v1` | 万相文生图 V1 版本，根据文本描述生成图像（基础版） |
| **万相-图像生成与编辑** | `wanx2.1-t2i-ediff` | 万相图像生成与编辑 2.6 版本 |
| **万相-通用图像编辑** | `wanx2.5-image-edit` | 万相通用图像编辑 2.5 版本 |
| **万相-通用图像编辑** | `wanx2.1-image-edit` | 万相通用图像编辑 2.1 版本 |
| **万相-涂鸦作画** | `wanx-sketch-to-image-v2` | 涂鸦作画，将涂鸦草图转化为精美图像 |
| **万相-涂鸦作画** | `wanx-sketch-to-image` | 涂鸦作画 V1 版本 |
| **万相-图像局部重绘** | `wanx-image-inpainting` | 图像局部重绘，对图像指定区域进行内容替换 |
| **万相-人像风格重绘** | `wanx-style-repaint` | 人像风格重绘，将人像照片转换为指定风格 |
| **万相-图像画面扩展** | `wanx-image-outpainting` | 图像画面扩展，扩展图像边界内容 |
| **千问-文生图** | `qwen-image-generation` | 千问文生图模型，千问多模态能力 |
| **千问-图像编辑** | `qwen-image-edit` | 千问图像编辑模型 |
| **千问-图像翻译** | `qwen-image-translate` | 千问图像翻译模型 |
| **文生图 Z-Image** | `zimage` | Z-Image 文生图模型 |
| **虚拟模特** | `virtualmodel` | 虚拟模特试衣展示 |
| **鞋靴模特** | `shoes-model` | 鞋靴模特展示 |
| **创意海报生成** | `poster-generation` | 创意海报自动生成 |
| **人物实例分割** | `person-instance-segmentation` | 人物实例分割 |
| **图像背景生成** | `image-background-generation` | 图像背景生成 |
| **图像擦除补全** | `image-erase-supplement` | 图像擦除补全 |
| **AI 试衣** | `OutfitAnyone` | AI 虚拟试衣 OutfitAnyone |
| **人物写真** | `FaceChain` | 人物写真 FaceChain |
| **创意文字** | `WordArt` | 创意文字生成（锦书） |
| **文生图 SD** | `stable-diffusion-xl` | Stable Diffusion XL 文生图 |
| **文生图 FLUX** | `flux-dev` / `flux-schnell` | FLUX 文生图模型 |

### 1.4 核心特性

- **多模型覆盖**：从基础文生图到高级图像编辑，覆盖图像生成全场景
- **同步/异步双模式**：支持同步调用（直接返回结果）和异步调用（提交任务后轮询结果）
- **OpenAI 兼容**：通过 `/compatible-mode/v1/` 路径提供 OpenAI 格式兼容接口
- **多种输出格式**：支持 URL 返回和 Base64 编码返回
- **多尺寸支持**：支持 512x512、720x1280、1280x720 等多种分辨率
- **风格化生成**：支持指定摄影风格、艺术风格等
- **中文优化**：对中文提示词有良好支持

---

## 2. 认证方式

### 2.1 API Key 认证

百炼平台使用 Bearer Token 认证方式，在请求头中携带 API Key：

```http
Authorization: Bearer <your-dashscope-api-key>
Content-Type: application/json
```

### 2.2 API Key 获取

1. 登录阿里云控制台：https://bailian.console.aliyun.com
2. 进入百炼平台，在左侧导航栏选择「API-KEY 管理」
3. 点击「创建 API Key」，系统自动生成 API Key
4. API Key 格式示例：`sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`

### 2.3 API Key 权限说明

| 权限项 | 说明 |
|--------|------|
| 工作空间绑定 | API Key 绑定到特定工作空间，只能访问该空间下的资源 |
| 模型访问 | 需要在工作空间中开通对应模型的访问权限 |
| 免费额度 | 部分模型提供免费额度，超出后按量计费 |
| 并发限制 | 不同 API Key 有不同的并发请求数限制 |

### 2.4 请求头完整示例

```http
POST /api/v1/services/aigc/text2image/image-synthesis HTTP/1.1
Host: dashscope.aliyuncs.com
Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Content-Type: application/json
X-DashScope-Async: enable
```

| 请求头 | 说明 |
|--------|------|
| `Authorization` | 必填，Bearer Token 认证，格式为 `Bearer <api-key>` |
| `Content-Type` | 必填，请求体格式，固定为 `application/json` |
| `X-DashScope-Async` | 可选，设置为 `enable` 时启用异步调用模式 |

---

## 3. API 端点列表

### 3.1 DashScope 原生端点

通义万相图像生成使用 DashScope 原生 API 端点，按功能分为以下几类：

#### 3.1.1 文生图（Text-to-Image）

| 方法 | 路径 | 功能 | 支持模型 |
|------|------|------|---------|
| POST | `/api/v1/services/aigc/text2image/image-synthesis` | 文本生成图像 | `wanx-v1`, `wanx-v2`, `wanx2.1-t2i-ediff` |
| POST | `/api/v1/services/aigc/text2image/image-synthesis` | 千问文生图 | `qwen-image-generation` |
| POST | `/api/v1/services/aigc/text2image/image-synthesis` | Z-Image 文生图 | `zimage` |

#### 3.1.2 涂鸦作画（Sketch-to-Image）

| 方法 | 路径 | 功能 | 支持模型 |
|------|------|------|---------|
| POST | `/api/v1/services/aigc/image2image/image-synthesis` | 涂鸦作画 | `wanx-sketch-to-image`, `wanx-sketch-to-image-v2` |

#### 3.1.3 图像局部重绘（Image Inpainting）

| 方法 | 路径 | 功能 | 支持模型 |
|------|------|------|---------|
| POST | `/api/v1/services/aigc/image-inpainting/image-synthesis` | 图像局部重绘 | `wanx-image-inpainting` |

#### 3.1.4 人像风格重绘（Portrait Style Repaint）

| 方法 | 路径 | 功能 | 支持模型 |
|------|------|------|---------|
| POST | `/api/v1/services/aigc/image-generation/image-synthesis` | 人像风格重绘 | `wanx-style-repaint` |

#### 3.1.5 通用图像编辑（Image Edit）

| 方法 | 路径 | 功能 | 支持模型 |
|------|------|------|---------|
| POST | `/api/v1/services/aigc/image-edit/image-synthesis` | 通用图像编辑 | `wanx2.1-image-edit`, `wanx2.5-image-edit` |

#### 3.1.6 图像画面扩展（Image Outpainting）

| 方法 | 路径 | 功能 | 支持模型 |
|------|------|------|---------|
| POST | `/api/v1/services/aigc/image-outpainting/image-synthesis` | 图像画面扩展 | `wanx-image-outpainting` |

#### 3.1.7 异步任务管理端点

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/v1/tasks/{task_id}` | 查询异步任务状态和结果 |

### 3.2 OpenAI 兼容端点

百炼平台同时提供 OpenAI 兼容接口，基础路径为 `/compatible-mode/v1/`：

| 方法 | 路径 | 功能 | 说明 |
|------|------|------|------|
| POST | `/compatible-mode/v1/images/generations` | 文本生成图像 | 兼容 OpenAI Images API 格式 |

---

## 4. 文生图接口详细参数

### 4.1 万相文生图 V2（wanx-v2）

**端点**：`POST /api/v1/services/aigc/text2image/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `wanx-v2` |
| `input.prompt` | string | 是 | — | 文本提示词，描述希望生成的图像内容，支持中英文 |
| `input.negative_prompt` | string | 否 | `""` | 反向提示词，描述不希望出现在图像中的内容 |
| `parameters.size` | string | 否 | `"1024*1024"` | 图像尺寸，格式为 `宽*高` |
| `parameters.n` | integer | 否 | `1` | 生成图像数量，取值范围 1-4 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子，相同种子和参数可复现结果 |
| `parameters.ref_strength` | float | 否 | `0.5` | 参考图影响强度，取值范围 0.0-1.0 |
| `parameters.ref_img` | string | 否 | `""` | 参考图片 URL，用于风格参考 |
| `parameters.style` | string | 否 | `"<auto>"` | 图像风格，可选值见下方风格列表 |
| `parameters.num_inference_steps` | integer | 否 | `50` | 推理步数，影响生成质量和速度 |

#### 支持的图像尺寸（wanx-v2）

| 尺寸 | 格式 | 说明 |
|------|------|------|
| 1024x1024 | `"1024*1024"` | 正方形 |
| 720x1280 | `"720*1280"` | 竖版（手机壁纸） |
| 1280x720 | `"1280*720"` | 横版（桌面壁纸） |
| 960x1280 | `"960*1280"` | 竖版（海报） |
| 1280x960 | `"1280*960"` | 横版（海报） |

#### 支持的风格（wanx-v2）

| 风格值 | 说明 |
|--------|------|
| `"<photography>"` | 摄影风格 |
| `"<portrait>"` | 人像摄影 |
| `"<3d-cartoon>"` | 3D 卡通 |
| `"<anime>"` | 动漫风格 |
| `"<oil-painting>"` | 油画风格 |
| `"<watercolor>"` | 水彩风格 |
| `"<sketch>"` | 素描风格 |
| `"<chinese-painting>"` | 中国画风格 |
| `"<auto>"` | 自动风格（由模型决定） |
| `"<auto>"` (默认) | 自动选择最佳风格 |

#### 请求体示例

```json
{
    "model": "wanx-v2",
    "input": {
        "prompt": "一只可爱的橘猫坐在窗台上，阳光洒在身上，温暖的色调",
        "negative_prompt": "低质量, 模糊, 变形"
    },
    "parameters": {
        "size": "1024*1024",
        "n": 1,
        "seed": 42,
        "style": "<photography>"
    }
}
```

### 4.2 万相文生图 V1（wanx-v1）

**端点**：`POST /api/v1/services/aigc/text2image/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `wanx-v1` |
| `input.prompt` | string | 是 | — | 文本提示词，描述希望生成的图像内容 |
| `input.negative_prompt` | string | 否 | `""` | 反向提示词 |
| `parameters.size` | string | 否 | `"1024*1024"` | 图像尺寸 |
| `parameters.n` | integer | 否 | `1` | 生成图像数量，取值范围 1-4 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子 |

### 4.3 千问文生图（qwen-image-generation）

**端点**：`POST /api/v1/services/aigc/text2image/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `qwen-image-generation` |
| `input.prompt` | string | 是 | — | 文本提示词，详细描述希望生成的图像 |
| `input.negative_prompt` | string | 否 | `""` | 反向提示词 |
| `parameters.size` | string | 否 | `"1024*1024"` | 图像尺寸，支持 `"1024*1024"`、`"720*1280"`、`"1280*720"` |
| `parameters.n` | integer | 否 | `1` | 生成图像数量 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子 |

### 4.4 万相图像生成与编辑 2.6（wanx2.1-t2i-ediff）

**端点**：`POST /api/v1/services/aigc/text2image/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `wanx2.1-t2i-ediff` |
| `input.prompt` | string | 是 | — | 文本提示词 |
| `input.negative_prompt` | string | 否 | `""` | 反向提示词 |
| `parameters.size` | string | 否 | `"1024*1024"` | 图像尺寸 |
| `parameters.n` | integer | 否 | `1` | 生成图像数量 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子 |
| `parameters.ref_img` | string | 否 | `""` | 参考图片 URL |
| `parameters.ref_strength` | float | 否 | `0.5` | 参考图影响强度，0.0-1.0 |

---

## 5. 涂鸦作画接口详细参数

### 5.1 万相涂鸦作画 V2（wanx-sketch-to-image-v2）

**端点**：`POST /api/v1/services/aigc/image2image/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `wanx-sketch-to-image-v2` |
| `input.prompt` | string | 是 | — | 文本提示词，描述希望生成的图像内容 |
| `input.negative_prompt` | string | 否 | `""` | 反向提示词 |
| `input.sketch.image_url` | string | 是 | — | 涂鸦草图的 URL 地址 |
| `input.sketch.image_format` | string | 否 | `"png"` | 涂鸦草图格式，支持 `png`、`jpg`、`jpeg` |
| `input.base_image.image_url` | string | 否 | `""` | 底图 URL，提供后涂鸦将叠加在底图上进行风格化 |
| `parameters.size` | string | 否 | `"1024*1024"` | 输出图像尺寸 |
| `parameters.n` | integer | 否 | `1` | 生成图像数量，取值范围 1-4 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子 |
| `parameters.strength` | float | 否 | `0.5` | 涂鸦影响强度，取值范围 0.0-1.0，值越大越贴近涂鸦 |
| `parameters.style` | string | 否 | `"<auto>"` | 图像风格，与文生图风格列表相同 |

#### 请求体示例

```json
{
    "model": "wanx-sketch-to-image-v2",
    "input": {
        "prompt": "一只彩色的蝴蝶在花丛中飞舞，水彩画风格",
        "sketch": {
            "image_url": "https://example.com/sketch.png"
        }
    },
    "parameters": {
        "size": "1024*1024",
        "n": 1,
        "strength": 0.6,
        "style": "<watercolor>"
    }
}
```

### 5.2 万相涂鸦作画 V1（wanx-sketch-to-image）

**端点**：`POST /api/v1/services/aigc/image2image/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `wanx-sketch-to-image` |
| `input.prompt` | string | 是 | — | 文本提示词 |
| `input.sketch.image_url` | string | 是 | — | 涂鸦草图的 URL 地址 |
| `parameters.size` | string | 否 | `"1024*1024"` | 输出图像尺寸 |
| `parameters.n` | integer | 否 | `1` | 生成图像数量 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子 |
| `parameters.strength` | float | 否 | `0.5` | 涂鸦影响强度 |

---

## 6. 图像局部重绘接口详细参数

### 6.1 万相图像局部重绘（wanx-image-inpainting）

**端点**：`POST /api/v1/services/aigc/image-inpainting/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `wanx-image-inpainting` |
| `input.prompt` | string | 是 | — | 文本提示词，描述重绘区域希望生成的内容 |
| `input.negative_prompt` | string | 否 | `""` | 反向提示词 |
| `input.image_url` | string | 是 | — | 原始图片的 URL 地址 |
| `input.mask_image_url` | string | 是 | — | 遮罩图片 URL，白色区域为需要重绘的部分 |
| `parameters.size` | string | 否 | 与原图相同 | 输出图像尺寸，默认保持与原图一致 |
| `parameters.n` | integer | 否 | `1` | 生成图像数量，取值范围 1-4 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子 |
| `parameters.strength` | float | 否 | `0.8` | 重绘强度，取值范围 0.0-1.0 |
| `parameters.ref_img` | string | 否 | `""` | 参考图片 URL，重绘区域风格将参考此图 |

#### 遮罩图片说明

遮罩图片（mask）用于标记需要重绘的区域：

| 颜色 | 含义 |
|------|------|
| 白色（RGB 255,255,255） | 需要重绘的区域 |
| 黑色（RGB 0,0,0） | 保持不变的区域 |

- 遮罩图片尺寸必须与原始图片尺寸一致
- 遮罩图片格式支持 PNG、JPG
- 支持多个不连续的重绘区域

#### 请求体示例

```json
{
    "model": "wanx-image-inpainting",
    "input": {
        "prompt": "把背景替换为日落时分的海边",
        "image_url": "https://example.com/original.png",
        "mask_image_url": "https://example.com/mask.png"
    },
    "parameters": {
        "n": 1,
        "strength": 0.8,
        "seed": 42
    }
}
```

---

## 7. 人像风格重绘接口详细参数

### 7.1 万相人像风格重绘（wanx-style-repaint）

**端点**：`POST /api/v1/services/aigc/image-generation/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `wanx-style-repaint` |
| `input.prompt` | string | 否 | `""` | 文本提示词，补充描述风格化方向 |
| `input.image_url` | string | 是 | — | 原始人像图片的 URL 地址 |
| `parameters.style` | string | 是 | — | 目标风格，见下方支持的风格列表 |
| `parameters.n` | integer | 否 | `1` | 生成图像数量，取值范围 1-4 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子 |
| `parameters.strength` | float | 否 | `0.6` | 风格化强度，取值范围 0.0-1.0，值越大风格越明显 |
| `parameters.size` | string | 否 | 与原图相同 | 输出图像尺寸 |

#### 支持的风格（人像风格重绘）

| 风格值 | 说明 |
|--------|------|
| `"cartoon"` | 卡通风格 |
| `"3d_cartoon"` | 3D 卡通风格 |
| `"anime"` | 动漫风格 |
| `"oil_painting"` | 油画风格 |
| `"watercolor"` | 水彩风格 |
| `"sketch"` | 素描风格 |
| `"cyberpunk"` | 赛博朋克风格 |
| `"chinese_painting"` | 国画风格 |
| `"id_photo"` | 证件照风格 |
| `"template"` | 模板风格（需配合 template_id 使用） |

#### 输入图片要求

| 要求项 | 说明 |
|--------|------|
| 图片格式 | PNG、JPG、JPEG |
| 图片大小 | 不超过 10MB |
| 图片尺寸 | 建议不超过 4096x4096 |
| 图片内容 | 必须包含清晰可识别的人像面部 |
| URL 可达性 | 图片 URL 必须公网可访问 |

#### 请求体示例

```json
{
    "model": "wanx-style-repaint",
    "input": {
        "image_url": "https://example.com/portrait.jpg"
    },
    "parameters": {
        "style": "anime",
        "strength": 0.7,
        "n": 1,
        "seed": 100
    }
}
```

---

## 8. 通用图像编辑接口详细参数

### 8.1 万相通用图像编辑 2.5（wanx2.5-image-edit）

**端点**：`POST /api/v1/services/aigc/image-edit/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `wanx2.5-image-edit` |
| `input.prompt` | string | 是 | — | 文本提示词，描述希望对图像进行的编辑操作 |
| `input.negative_prompt` | string | 否 | `""` | 反向提示词 |
| `input.image_url` | string | 是 | — | 原始图片的 URL 地址 |
| `input.mask_image_url` | string | 否 | `""` | 遮罩图片 URL，指定编辑区域 |
| `parameters.size` | string | 否 | 与原图相同 | 输出图像尺寸 |
| `parameters.n` | integer | 否 | `1` | 生成图像数量 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子 |
| `parameters.strength` | float | 否 | `0.7` | 编辑强度，取值范围 0.0-1.0 |
| `parameters.ref_img` | string | 否 | `""` | 参考图片 URL |

### 8.2 万相通用图像编辑 2.1（wanx2.1-image-edit）

**端点**：`POST /api/v1/services/aigc/image-edit/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `wanx2.1-image-edit` |
| `input.prompt` | string | 是 | — | 文本提示词 |
| `input.image_url` | string | 是 | — | 原始图片的 URL 地址 |
| `input.mask_image_url` | string | 否 | `""` | 遮罩图片 URL |
| `parameters.size` | string | 否 | 与原图相同 | 输出图像尺寸 |
| `parameters.n` | integer | 否 | `1` | 生成图像数量 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子 |
| `parameters.strength` | float | 否 | `0.7` | 编辑强度 |

### 8.3 千问图像编辑（qwen-image-edit）

**端点**：`POST /api/v1/services/aigc/image-edit/image-synthesis`

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `model` | string | 是 | — | 模型名称，固定为 `qwen-image-edit` |
| `input.prompt` | string | 是 | — | 文本提示词，描述编辑操作 |
| `input.image_url` | string | 是 | — | 原始图片 URL |
| `input.mask_image_url` | string | 否 | `""` | 遮罩图片 URL |
| `parameters.size` | string | 否 | `"1024*1024"` | 输出图像尺寸 |
| `parameters.n` | integer | 否 | `1` | 生成图像数量 |
| `parameters.seed` | integer | 否 | 随机 | 随机种子 |

#### 请求体示例（通用图像编辑）

```json
{
    "model": "wanx2.5-image-edit",
    "input": {
        "prompt": "将天空变为晴天蓝天白云的效果",
        "image_url": "https://example.com/photo.jpg",
        "mask_image_url": "https://example.com/sky_mask.png"
    },
    "parameters": {
        "strength": 0.7,
        "n": 1
    }
}
```

---

## 9. 同步与异步调用方式

### 9.1 调用模式概述

通义万相 API 支持两种调用模式：

| 模式 | 适用场景 | 响应方式 |
|------|---------|---------|
| 同步调用 | 生成速度快的小图、简单场景 | 请求阻塞等待，直接返回生成结果 |
| 异步调用 | 高分辨率图像、批量生成、耗时操作 | 立即返回任务 ID，通过轮询获取结果 |

### 9.2 同步调用

同步调用时，不设置 `X-DashScope-Async` 请求头，API 会阻塞等待图像生成完成后直接返回结果。

**请求示例（同步）**：

```bash
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis' \
  -H 'Authorization: Bearer sk-xxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "wanx-v2",
    "input": {
        "prompt": "一只可爱的橘猫坐在窗台上"
    },
    "parameters": {
        "size": "1024*1024",
        "n": 1
    }
  }'
```

**同步响应**：

请求成功时，HTTP 状态码为 200，响应体中直接包含生成的图像 URL：

```json
{
    "output": {
        "task_status": "SUCCEEDED",
        "results": [
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx.png"
            }
        ]
    },
    "request_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

### 9.3 异步调用

异步调用时，在请求头中设置 `X-DashScope-Async: enable`，API 立即返回任务 ID。

**步骤一：提交异步任务**

```bash
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis' \
  -H 'Authorization: Bearer sk-xxxxxxxx' \
  -H 'Content-Type: application/json' \
  -H 'X-DashScope-Async: enable' \
  -d '{
    "model": "wanx-v2",
    "input": {
        "prompt": "一只可爱的橘猫坐在窗台上"
    },
    "parameters": {
        "size": "1024*1024",
        "n": 4
    }
  }'
```

**异步任务提交响应**：

```json
{
    "output": {
        "task_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
        "task_status": "PENDING"
    },
    "request_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

**步骤二：轮询任务状态**

使用返回的 `task_id` 查询任务状态：

```bash
curl -X GET 'https://dashscope.aliyuncs.com/api/v1/tasks/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx' \
  -H 'Authorization: Bearer sk-xxxxxxxx'
```

**轮询响应（任务进行中）**：

```json
{
    "output": {
        "task_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
        "task_status": "RUNNING"
    },
    "request_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

**轮询响应（任务完成）**：

```json
{
    "output": {
        "task_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
        "task_status": "SUCCEEDED",
        "results": [
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx_0.png"
            },
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx_1.png"
            },
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx_2.png"
            },
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx_3.png"
            }
        ],
        "submit_time": "2026-04-19 10:00:00.000",
        "scheduled_time": "2026-04-19 10:00:01.000",
        "end_time": "2026-04-19 10:00:15.000"
    },
    "usage": {
        "image_count": 4
    },
    "request_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

### 9.4 任务状态流转

| 状态 | 说明 | 是否终态 |
|------|------|---------|
| `PENDING` | 任务已提交，排队等待中 | 否 |
| `RUNNING` | 任务正在执行中 | 否 |
| `SUCCEEDED` | 任务执行成功 | 是 |
| `FAILED` | 任务执行失败 | 是 |
| `UNKNOWN` | 任务状态未知 | 是 |
| `CANCELED` | 任务已取消 | 是 |

### 9.5 轮询建议

| 建议项 | 说明 |
|--------|------|
| 轮询间隔 | 建议 2-5 秒一次，不宜过于频繁 |
| 最大轮询次数 | 建议 60 次（约 3-5 分钟） |
| 超时处理 | 超过最大轮询次数后视为超时 |
| 终态判断 | `task_status` 为 `SUCCEEDED`、`FAILED`、`UNKNOWN` 或 `CANCELED` 时停止轮询 |

---

## 10. 响应格式

### 10.1 通用响应结构

所有 DashScope API 响应都遵循统一结构：

```json
{
    "output": { ... },
    "usage": { ... },
    "request_id": "string"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `output` | object | 输出内容，包含任务状态和结果 |
| `usage` | object | 用量信息 |
| `request_id` | string | 请求唯一标识，用于问题排查 |

### 10.2 同步调用成功响应

```json
{
    "output": {
        "task_status": "SUCCEEDED",
        "results": [
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/1a2b3c/image_0.png?Expires=1745088000&OSSAccessKeyId=xxx&Signature=xxx"
            }
        ]
    },
    "usage": {
        "image_count": 1
    },
    "request_id": "f1e2d3c4-b5a6-7890-abcd-ef1234567890"
}
```

### 10.3 异步调用任务提交响应

```json
{
    "output": {
        "task_id": "f1e2d3c4-b5a6-7890-abcd-ef1234567890",
        "task_status": "PENDING"
    },
    "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 10.4 异步任务完成响应

```json
{
    "output": {
        "task_id": "f1e2d3c4-b5a6-7890-abcd-ef1234567890",
        "task_status": "SUCCEEDED",
        "results": [
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/image_0.png?Expires=xxx&OSSAccessKeyId=xxx&Signature=xxx"
            },
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/image_1.png?Expires=xxx&OSSAccessKeyId=xxx&Signature=xxx"
            }
        ],
        "submit_time": "2026-04-19 10:00:00.000",
        "scheduled_time": "2026-04-19 10:00:01.000",
        "end_time": "2026-04-19 10:00:15.000"
    },
    "usage": {
        "image_count": 2
    },
    "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 10.5 异步任务失败响应

```json
{
    "output": {
        "task_id": "f1e2d3c4-b5a6-7890-abcd-ef1234567890",
        "task_status": "FAILED",
        "code": "InvalidParameter",
        "message": "Prompt content is empty."
    },
    "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
}
```

### 10.6 结果 URL 说明

| 属性 | 说明 |
|------|------|
| 有效期 | 结果 URL 有效期为 **1 小时**，过期后无法访问 |
| 格式 | OSS 签名 URL，包含 Expires、OSSAccessKeyId、Signature 参数 |
| 建议处理 | 获取 URL 后尽快下载保存，避免过期失效 |
| 域名 | 通常为 `dashscope-result-bj.oss-cn-beijing.aliyuncs.com` |

### 10.7 OpenAI 兼容模式响应

使用 `/compatible-mode/v1/images/generations` 端点时，返回 OpenAI 格式响应：

```json
{
    "created": 1745088000,
    "data": [
        {
            "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/image_0.png"
        }
    ]
}
```

---

## 11. 错误码与错误处理

### 11.1 HTTP 状态码

| HTTP 状态码 | 说明 |
|-------------|------|
| 200 | 请求成功 |
| 400 | 请求参数错误 |
| 401 | 认证失败（API Key 无效或过期） |
| 403 | 权限不足（无权访问该模型或资源） |
| 404 | 资源不存在（模型不存在或任务不存在） |
| 429 | 请求频率超限或配额不足 |
| 500 | 服务器内部错误 |
| 503 | 模型服务不可用 |

### 11.2 错误响应格式

DashScope 原生错误响应格式：

```json
{
    "code": "InvalidParameter",
    "message": "Prompt content is empty.",
    "request_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

OpenAI 兼容模式错误响应格式：

```json
{
    "error": {
        "code": "invalid_request_error",
        "message": "Prompt content is empty.",
        "param": "prompt",
        "type": "invalid_request_error"
    }
}
```

### 11.3 完整错误码列表

#### 400 系列 — 请求参数错误

| 错误码 | 错误消息 | 说明 |
|--------|---------|------|
| `InvalidParameter` | Prompt content is empty | 提示词内容为空 |
| `InvalidParameter` | Image format is not supported | 不支持的图片格式 |
| `InvalidParameter` | Image size exceeds the limit | 图片尺寸超出限制 |
| `InvalidParameter` | The value of parameter `n` is invalid | 参数 n 的值无效 |
| `InvalidParameter` | The value of parameter `size` is invalid | 参数 size 的值无效 |
| `InvalidParameter` | The value of parameter `seed` is invalid | 参数 seed 的值无效 |
| `InvalidParameter` | The value of parameter `strength` is invalid | 参数 strength 的值无效 |
| `InvalidParameter` | The value of parameter `style` is invalid | 参数 style 的值无效 |
| `InvalidParameter` | Prompt content may contain inappropriate content | 提示词可能包含不适当内容 |
| `InvalidParameter` | The number of input images exceeds the limit | 输入图片数量超出限制 |
| `InvalidParameter` | Mask image size does not match original image | 遮罩图片尺寸与原图不匹配 |
| `InvalidParameter` | Image URL is not accessible | 图片 URL 不可访问 |
| `InvalidParameter` | Model is not found | 模型不存在 |

#### 401 系列 — 认证错误

| 错误码 | 错误消息 | 说明 |
|--------|---------|------|
| `InvalidApiKey` | Invalid API-key provided | 提供的 API Key 无效 |
| `invalid_api_key` | Invalid API key | API Key 无效（OpenAI 兼容模式） |
| `InvalidApiKey` | API key is not valid or has expired | API Key 已过期或无效 |
| `InvalidAuthorization` | Authorization header is empty | 认证头为空 |

#### 403 系列 — 权限错误

| 错误码 | 错误消息 | 说明 |
|--------|---------|------|
| `AccessDenied` | Access denied | 访问被拒绝 |
| `Model.AccessDenied` | You are not authorized to use this model | 无权使用该模型 |
| `AllocationQuota.FreeTierOnly` | Free tier quota exceeded | 免费额度已用完 |
| `AccessDenied` | Workspace does not have permission to access this resource | 工作空间无权访问该资源 |

#### 404 系列 — 资源不存在

| 错误码 | 错误消息 | 说明 |
|--------|---------|------|
| `ModelNotFound` | Model not found | 模型不存在 |
| `model_not_supported` | Model is not supported | 不支持的模型（OpenAI 兼容模式） |
| `TaskNotFound` | Task not found | 任务不存在（查询异步任务时） |

#### 429 系列 — 限流错误

| 错误码 | 错误消息 | 说明 |
|--------|---------|------|
| `Throttling` | Request was throttled. Expected available in X seconds | 请求被限流，预期 X 秒后可用 |
| `Throttting` | Requests rate limit exceeded | 请求频率超限 |
| `Throttling` | Quota exceeded | 配额不足 |
| `Throttling` | Concurrent request limit exceeded | 并发请求限制超出 |
| `insufficient_quota` | Insufficient quota | 配额不足（OpenAI 兼容模式） |
| `rate_limit_error` | Rate limit reached | 速率限制（OpenAI 兼容模式） |

#### 500 系列 — 服务器错误

| 错误码 | 错误消息 | 说明 |
|--------|---------|------|
| `InternalError` | An internal error has occurred | 内部错误 |
| `RequestTimeOut` | Request timed out | 请求超时 |
| `ModelUnavailable` | Model is temporarily unavailable | 模型暂时不可用 |
| `ModelServingError` | Model serving error | 模型推理服务错误 |
| `InternalError` | Image generation failed | 图像生成失败 |

### 11.4 错误处理最佳实践

| 实践 | 说明 |
|------|------|
| 重试策略 | 500/503 错误建议指数退避重试，最多 3 次 |
| 限流处理 | 429 错误根据 `Retry-After` 响应头等待后重试 |
| 参数校验 | 400 错误检查请求参数，修正后重新提交 |
| 认证检查 | 401 错误检查 API Key 是否正确且未过期 |
| 权限确认 | 403 错误确认已在百炼平台开通对应模型权限 |
| 超时设置 | 建议同步调用设置 60 秒超时，异步调用设置 10 秒超时 |
| URL 及时下载 | 生成结果 URL 有效期 1 小时，建议获取后立即下载 |
| 任务超时 | 异步任务建议最长等待 5 分钟，超时视为失败 |

---

## 12. 请求与响应示例

### 12.1 文生图 — 同步调用

**请求**：

```bash
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis' \
  -H 'Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "wanx-v2",
    "input": {
        "prompt": "一座古色古香的中式庭院，小桥流水，假山翠竹，傍晚的暖色调光线",
        "negative_prompt": "低质量, 模糊, 变形, 水印"
    },
    "parameters": {
        "size": "1280*720",
        "n": 1,
        "seed": 42,
        "style": "<photography>"
    }
  }'
```

**响应**：

```json
{
    "output": {
        "task_status": "SUCCEEDED",
        "results": [
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/a1b2c3d4/e5f6g7h8.png?Expires=1745091600&OSSAccessKeyId=LTAI5txxx&Signature=xxxxx"
            }
        ]
    },
    "usage": {
        "image_count": 1
    },
    "request_id": "f1e2d3c4-b5a6-7890-abcd-ef1234567890"
}
```

### 12.2 文生图 — 异步调用

**步骤一：提交任务**

```bash
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis' \
  -H 'Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' \
  -H 'Content-Type: application/json' \
  -H 'X-DashScope-Async: enable' \
  -d '{
    "model": "wanx-v2",
    "input": {
        "prompt": "未来城市的天际线，赛博朋克风格，霓虹灯光"
    },
    "parameters": {
        "size": "1280*720",
        "n": 4,
        "style": "<auto>"
    }
  }'
```

**提交任务响应**：

```json
{
    "output": {
        "task_id": "abcd1234-5678-efgh-9012-ijkl34567890",
        "task_status": "PENDING"
    },
    "request_id": "1234abcd-5678-efgh-9012-ijkl34567890"
}
```

**步骤二：查询任务结果**

```bash
curl -X GET 'https://dashscope.aliyuncs.com/api/v1/tasks/abcd1234-5678-efgh-9012-ijkl34567890' \
  -H 'Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'
```

**任务完成响应**：

```json
{
    "output": {
        "task_id": "abcd1234-5678-efgh-9012-ijkl34567890",
        "task_status": "SUCCEEDED",
        "results": [
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/img_0.png?Expires=1745091600&OSSAccessKeyId=xxx&Signature=xxx"
            },
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/img_1.png?Expires=1745091600&OSSAccessKeyId=xxx&Signature=xxx"
            },
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/img_2.png?Expires=1745091600&OSSAccessKeyId=xxx&Signature=xxx"
            },
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/img_3.png?Expires=1745091600&OSSAccessKeyId=xxx&Signature=xxx"
            }
        ],
        "submit_time": "2026-04-19 10:00:00.000",
        "scheduled_time": "2026-04-19 10:00:01.500",
        "end_time": "2026-04-19 10:00:20.000"
    },
    "usage": {
        "image_count": 4
    },
    "request_id": "1234abcd-5678-efgh-9012-ijkl34567890"
}
```

### 12.3 涂鸦作画

**请求**：

```bash
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/image2image/image-synthesis' \
  -H 'Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "wanx-sketch-to-image-v2",
    "input": {
        "prompt": "一栋温馨的小木屋，周围是绿色的草地和鲜花，童话风格",
        "sketch": {
            "image_url": "https://example.com/my-sketch.png"
        }
    },
    "parameters": {
        "size": "1024*1024",
        "n": 1,
        "strength": 0.5,
        "style": "<watercolor>"
    }
  }'
```

**响应**：

```json
{
    "output": {
        "task_status": "SUCCEEDED",
        "results": [
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/sketch_result.png?Expires=1745091600&OSSAccessKeyId=xxx&Signature=xxx"
            }
        ]
    },
    "usage": {
        "image_count": 1
    },
    "request_id": "c3d4e5f6-a7b8-9012-cdef-345678901234"
}
```

### 12.4 图像局部重绘

**请求**：

```bash
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/image-inpainting/image-synthesis' \
  -H 'Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "wanx-image-inpainting",
    "input": {
        "prompt": "一只金色的花瓶放在桌子上",
        "image_url": "https://example.com/room.png",
        "mask_image_url": "https://example.com/room_mask.png"
    },
    "parameters": {
        "n": 1,
        "strength": 0.8
    }
  }'
```

**响应**：

```json
{
    "output": {
        "task_status": "SUCCEEDED",
        "results": [
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/inpaint_result.png?Expires=1745091600&OSSAccessKeyId=xxx&Signature=xxx"
            }
        ]
    },
    "usage": {
        "image_count": 1
    },
    "request_id": "e5f6a7b8-c9d0-1234-ef56-789012345678"
}
```

### 12.5 人像风格重绘

**请求**：

```bash
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/image-generation/image-synthesis' \
  -H 'Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "wanx-style-repaint",
    "input": {
        "image_url": "https://example.com/my-photo.jpg"
    },
    "parameters": {
        "style": "anime",
        "strength": 0.7,
        "n": 1
    }
  }'
```

**响应**：

```json
{
    "output": {
        "task_status": "SUCCEEDED",
        "results": [
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/style_result.png?Expires=1745091600&OSSAccessKeyId=xxx&Signature=xxx"
            }
        ]
    },
    "usage": {
        "image_count": 1
    },
    "request_id": "f6a7b8c9-d0e1-2345-fa67-890123456789"
}
```

### 12.6 通用图像编辑

**请求**：

```bash
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/image-edit/image-synthesis' \
  -H 'Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "wanx2.5-image-edit",
    "input": {
        "prompt": "将图片中的猫咪替换为一只小狗",
        "image_url": "https://example.com/cat_photo.jpg",
        "mask_image_url": "https://example.com/cat_mask.png"
    },
    "parameters": {
        "strength": 0.8,
        "n": 1,
        "seed": 42
    }
  }'
```

**响应**：

```json
{
    "output": {
        "task_status": "SUCCEEDED",
        "results": [
            {
                "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/edit_result.png?Expires=1745091600&OSSAccessKeyId=xxx&Signature=xxx"
            }
        ]
    },
    "usage": {
        "image_count": 1
    },
    "request_id": "a7b8c9d0-e1f2-3456-ab78-901234567890"
}
```

### 12.7 错误响应示例

#### 参数错误（400）

```json
{
    "code": "InvalidParameter",
    "message": "The value of parameter `size` is invalid. Supported values: 1024*1024, 720*1280, 1280*720, 960*1280, 1280*960",
    "request_id": "b8c9d0e1-f2a3-4567-bc89-012345678901"
}
```

#### 认证失败（401）

```json
{
    "code": "InvalidApiKey",
    "message": "Invalid API-key provided.",
    "request_id": "c9d0e1f2-a3b4-5678-cd90-123456789012"
}
```

#### 权限不足（403）

```json
{
    "code": "Model.AccessDenied",
    "message": "You are not authorized to use this model. Please activate the model in the Bailian console first.",
    "request_id": "d0e1f2a3-b4c5-6789-de01-234567890123"
}
```

#### 模型不存在（404）

```json
{
    "code": "ModelNotFound",
    "message": "Model not found: wanx-v99",
    "request_id": "e1f2a3b4-c5d6-7890-ef12-345678901234"
}
```

#### 请求限流（429）

```json
{
    "code": "Throttling",
    "message": "Request was throttled. Expected available in 5 seconds.",
    "request_id": "f2a3b4c5-d6e7-8901-fa23-456789012345"
}
```

#### 服务器错误（500）

```json
{
    "code": "InternalError",
    "message": "An internal error has occurred. Please try again later.",
    "request_id": "a3b4c5d6-e7f8-9012-ab34-567890123456"
}
```

#### 内容审核拦截（400）

```json
{
    "output": {
        "task_id": "b4c5d6e7-f8a9-0123-bc45-678901234567",
        "task_status": "FAILED",
        "code": "InvalidParameter",
        "message": "Prompt content may contain inappropriate content."
    },
    "request_id": "c5d6e7f8-a9b0-1234-cd56-789012345678"
}
```

---

## 13. OpenAI 兼容接口说明

### 13.1 概述

百炼平台提供 OpenAI 兼容接口，允许使用 OpenAI SDK 或兼容 OpenAI 格式的客户端直接调用通义万相图像生成模型。

### 13.2 兼容端点

| 方法 | 路径 | 功能 |
|------|------|------|
| POST | `/compatible-mode/v1/images/generations` | 文本生成图像（兼容 OpenAI Images API） |

### 13.3 认证方式

与 OpenAI 相同的 Bearer Token 认证：

```http
Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Content-Type: application/json
```

### 13.4 请求参数映射

OpenAI 兼容模式的请求参数与 DashScope 原生参数映射关系：

| OpenAI 参数 | DashScope 参数 | 说明 |
|-------------|---------------|------|
| `model` | `model` | 模型名称，直接映射 |
| `prompt` | `input.prompt` | 提示词 |
| `n` | `parameters.n` | 生成数量 |
| `size` | `parameters.size` | 图像尺寸（格式需转换） |
| `response_format` | — | `url` 或 `b64_json` |

**尺寸格式差异**：

| OpenAI 格式 | DashScope 格式 |
|-------------|---------------|
| `"1024x1024"` | `"1024*1024"` |
| `"1024x1792"` | — |
| `"1792x1024"` | — |

注意：OpenAI 使用 `x` 分隔宽高，DashScope 使用 `*` 分隔宽高。

### 13.5 请求示例

```bash
curl -X POST 'https://dashscope.aliyuncs.com/compatible-mode/v1/images/generations' \
  -H 'Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "wanx-v2",
    "prompt": "一只可爱的橘猫坐在窗台上，阳光洒在身上",
    "n": 1,
    "size": "1024x1024"
  }'
```

### 13.6 响应格式

OpenAI 兼容模式返回 OpenAI 标准响应格式：

```json
{
    "created": 1745088000,
    "data": [
        {
            "url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/image_0.png"
        }
    ]
}
```

当 `response_format` 为 `b64_json` 时：

```json
{
    "created": 1745088000,
    "data": [
        {
            "b64_json": "iVBORw0KGgoAAAANSUhEUgAABAAAAAQACAIAAADwf7zUAAAABGdBTUEAALGPC..."
        }
    ]
}
```

### 13.7 OpenAI 兼容模式支持的模型

以下模型支持通过 OpenAI 兼容端点调用：

| 模型名称 | 说明 |
|---------|------|
| `wanx-v1` | 万相文生图 V1 |
| `wanx-v2` | 万相文生图 V2 |
| `wanx2.1-t2i-ediff` | 万相图像生成与编辑 2.6 |
| `qwen-image-generation` | 千问文生图 |
| `zimage` | Z-Image 文生图 |
| `flux-dev` | FLUX Dev 文生图 |
| `flux-schnell` | FLUX Schnell 文生图 |

---

## 14. team-api 适配器实现说明

### 14.1 协议转换要点

在 team-api 的 relay 层实现通义万相适配器时，需要注意以下关键转换：

| 转换方向 | 转换内容 |
|---------|---------|
| 统一入口 → DashScope | OpenAI `/v1/images/generations` 格式转为 DashScope 原生格式 |
| DashScope → 统一出口 | DashScope 响应转为 OpenAI Images API 响应格式 |
| 尺寸格式 | `宽x高` → `宽*高`（OpenAI 格式转 DashScope 格式） |
| 响应结构 | `output.results[].url` → `data[].url` |

### 14.2 异步任务处理

由于通义万相高分辨率图像生成耗时较长，适配器需要处理异步模式：

1. **同步模式**：对于简单请求（低分辨率、单图），使用同步调用直接转发响应
2. **异步模式**：对于耗时请求（高分辨率、多图），使用异步调用：
   - 提交任务获取 task_id
   - 内部轮询任务状态（对客户端透明）
   - 任务完成后转换为 OpenAI 格式响应返回

### 14.3 错误格式转换

| DashScope 错误码 | OpenAI 错误类型 | HTTP 状态码 |
|-----------------|----------------|-------------|
| `InvalidApiKey` | `authentication_error` | 401 |
| `Model.AccessDenied` | `permission_error` | 403 |
| `InvalidParameter` | `invalid_request_error` | 400 |
| `ModelNotFound` | `invalid_request_error` | 404 |
| `Throttling` | `rate_limit_error` | 429 |
| `InternalError` | `server_error` | 500 |
| `ModelServingError` | `server_error` | 503 |

### 14.4 适配器代码结构建议

在 `relay/channel/ali/` 目录下实现通义万相适配器：

```
relay/channel/ali/
├── adaptor.go              # Adaptor 接口实现
├── image.go                # 图像生成请求/响应转换
├── image_types.go          # DashScope 图像 API 类型定义
└── async_task.go           # 异步任务轮询处理
```

### 14.5 渠道配置说明

| 配置项 | 说明 | 示例值 |
|--------|------|--------|
| Base URL | DashScope API 地址 | `https://dashscope.aliyuncs.com` |
| 模型映射 | 统一模型名 → DashScope 模型名 | `wanx-v2` → `wanx-v2` |
| 超时设置 | 同步调用建议 60s，异步提交建议 10s | — |
| 重试策略 | 500/503 错误指数退避重试，最多 3 次 | — |
| 默认模式 | 建议默认使用异步模式，避免同步超时 | `X-DashScope-Async: enable` |

### 14.6 模型能力矩阵

| 模型 | 文生图 | 图生图 | 图像编辑 | 局部重绘 | 风格重绘 | 画面扩展 |
|------|--------|--------|---------|---------|---------|---------|
| `wanx-v2` | 是 | — | — | — | — | — |
| `wanx-v1` | 是 | — | — | — | — | — |
| `wanx2.1-t2i-ediff` | 是 | — | — | — | — | — |
| `wanx-sketch-to-image-v2` | — | 是 | — | — | — | — |
| `wanx-image-inpainting` | — | — | — | 是 | — | — |
| `wanx-style-repaint` | — | — | — | — | 是 | — |
| `wanx2.5-image-edit` | — | — | 是 | 是 | — | — |
| `wanx2.1-image-edit` | — | — | 是 | 是 | — | — |
| `wanx-image-outpainting` | — | — | — | — | — | 是 |
| `qwen-image-generation` | 是 | — | — | — | — | — |
| `qwen-image-edit` | — | — | 是 | — | — | — |

### 14.7 计费相关说明

| 模型 | 计费单位 | 说明 |
|------|---------|------|
| `wanx-v1` | 每张 | 按生成图片数量计费 |
| `wanx-v2` | 每张 | 按生成图片数量计费 |
| `wanx-sketch-to-image-v2` | 每张 | 按生成图片数量计费 |
| `wanx-image-inpainting` | 每张 | 按生成图片数量计费 |
| `wanx-style-repaint` | 每张 | 按生成图片数量计费 |
| `wanx2.5-image-edit` | 每张 | 按生成图片数量计费 |

在 team-api 的计费引擎中，需要将 DashScope 的 `usage.image_count` 字段转换为统一的 token 用量或次数计量，以便纳入平台的预扣-结算-退款流程。

---

## 附录 A：各端点完整 URL 汇总

| 功能 | 完整 URL |
|------|---------|
| 文生图 | `https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis` |
| 涂鸦作画 | `https://dashscope.aliyuncs.com/api/v1/services/aigc/image2image/image-synthesis` |
| 图像局部重绘 | `https://dashscope.aliyuncs.com/api/v1/services/aigc/image-inpainting/image-synthesis` |
| 人像风格重绘 | `https://dashscope.aliyuncs.com/api/v1/services/aigc/image-generation/image-synthesis` |
| 通用图像编辑 | `https://dashscope.aliyuncs.com/api/v1/services/aigc/image-edit/image-synthesis` |
| 图像画面扩展 | `https://dashscope.aliyuncs.com/api/v1/services/aigc/image-outpainting/image-synthesis` |
| 异步任务查询 | `https://dashscope.aliyuncs.com/api/v1/tasks/{task_id}` |
| OpenAI 兼容文生图 | `https://dashscope.aliyuncs.com/compatible-mode/v1/images/generations` |

## 附录 B：常用 cURL 模板

### B.1 文生图同步模板

```bash
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "wanx-v2",
    "input": {
        "prompt": "YOUR_PROMPT_HERE",
        "negative_prompt": ""
    },
    "parameters": {
        "size": "1024*1024",
        "n": 1,
        "seed": 42,
        "style": "<auto>"
    }
  }'
```

### B.2 文生图异步模板

```bash
curl -X POST 'https://dashscope.aliyuncs.com/api/v1/services/aigc/text2image/image-synthesis' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -H 'X-DashScope-Async: enable' \
  -d '{
    "model": "wanx-v2",
    "input": {
        "prompt": "YOUR_PROMPT_HERE"
    },
    "parameters": {
        "size": "1024*1024",
        "n": 4
    }
  }'
```

### B.3 查询异步任务模板

```bash
curl -X GET 'https://dashscope.aliyuncs.com/api/v1/tasks/YOUR_TASK_ID' \
  -H 'Authorization: Bearer YOUR_API_KEY'
```

### B.4 OpenAI 兼容模板

```bash
curl -X POST 'https://dashscope.aliyuncs.com/compatible-mode/v1/images/generations' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "wanx-v2",
    "prompt": "YOUR_PROMPT_HERE",
    "n": 1,
    "size": "1024x1024"
  }'
```
