# Stability AI 图像生成 API 文档

## 概述

Stability AI 是 Stable Diffusion 模型的创建者，提供图像生成 REST API（v2beta）。API 基于 Stable Image 品牌提供服务，支持文生图、图生图、局部重绘、超分辨率、背景移除等功能。

- **官方文档**：https://platform.stability.ai/docs
- **API Base URL**：`https://api.stability.ai`
- **API 版本**：v2beta（当前最新）
- **认证方式**：Bearer Token（API Key）
- **协议格式**：REST + multipart/form-data（部分接口支持 JSON）

## 认证

### 获取 API Key

1. 注册 https://platform.stability.ai 账号
2. 在 Account Settings → API Keys 页面创建 Key
3. 使用 Bearer Token 认证

### 认证头

```
Authorization: Bearer sk-xxxxxxxxxxxxxxxxxxxxxxxx
```

### 计费方式

- 按 Credit 消耗计费，不同模型/服务消耗不同 Credit
- 免费账户有初始 Credit
- Credit 可在平台购买

## 核心概念

### Stable Image 服务

Stability AI 的图像生成服务按功能分为以下类别：

| 服务 | 说明 | 核心端点路径 |
|------|------|-------------|
| Text to Image | 文生图 | `/v2beta/stable-image/generate/{model}` |
| Image to Image | 图生图 | `/v2beta/stable-image/generate/{model}` |
| Inpaint / Outpaint | 局部重绘/外扩 | `/v2beta/stable-image/generate/{model}` |
| Upscale | 超分辨率放大 | `/v2beta/stable-image/upscale/{model}` |
| Remove Background | 背景移除 | `/v2beta/stable-image/remove-bg` |
| Search and Replace | 搜索替换 | `/v2beta/stable-image/generate/{model}` |

### 可用模型

| 模型 ID | 说明 | 特点 |
|---------|------|------|
| `sd3` | Stable Diffusion 3 | 高质量文生图，支持 T5 文本编码器 |
| `sd3-turbo` | SD3 Turbo | 快速生成，适合高吞吐场景 |
| `sd3.5-large` | Stable Diffusion 3.5 Large | 最新旗舰模型，最高质量 |
| `sd3.5-large-turbo` | SD3.5 Large Turbo | 旗舰快速版 |
| `sd3.5-medium` | Stable Diffusion 3.5 Medium | 质量与速度平衡 |
| `sdxl` | Stable Diffusion XL | 上代旗舰，广泛支持 |
| `sdxl-turbo` | SDXL Turbo | SDXL 快速版 |
| `core` | Stable Image Core | 通用高质量模型 |
| `ultra` | Stable Image Ultra | 最高质量，4 步生成 |

### 模型选择建议

| 场景 | 推荐模型 | 原因 |
|------|---------|------|
| 最高质量 | `sd3.5-large` 或 `ultra` | 旗舰级输出 |
| 质量与速度平衡 | `sd3.5-medium` 或 `core` | 性价比最优 |
| 快速批量生成 | `sd3-turbo` 或 `sd3.5-large-turbo` | 最低延迟 |
| 兼容性最佳 | `sdxl` | 社区生态最丰富 |

## API 端点详解

### 1. 文生图（Text to Image）

根据文本提示词生成图像。

**请求**

```
POST https://api.stability.ai/v2beta/stable-image/generate/{model}
```

**Headers**

| Header | 值 | 必填 | 说明 |
|--------|---|------|------|
| `Authorization` | `Bearer sk-xxx` | 是 | API Key 认证 |
| `Content-Type` | `multipart/form-data` | 是 | 请求体格式 |
| `Accept` | `image/*` 或 `application/json` | 是 | 响应格式。`image/*` 直接返回图像二进制；`application/json` 返回 base64 编码 |

**Body 参数（multipart/form-data）**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `prompt` | string | 是 | — | 正向提示词，描述期望生成的图像内容 |
| `negative_prompt` | string | 否 | `""` | 反向提示词，描述不希望出现的内容（部分模型支持） |
| `aspect_ratio` | string | 否 | `"1:1"` | 宽高比。可选值：`"1:1"`, `"16:9"`, `"21:9"`, `"2:3"`, `"3:2"`, `"4:5"`, `"5:4"`, `"9:16"`, `"9:21"` |
| `seed` | integer | 否 | 随机 | 随机种子，相同种子+参数可复现结果。范围：0 ~ 4294967294 |
| `output_format` | string | 否 | `"png"` | 输出格式。可选值：`"png"`, `"jpeg"`, `"webp"` |
| `mode` | string | 否 | `"text-to-image"` | 生成模式。文生图使用 `"text-to-image"` |
| `model` | string | 否 | URL 中的模型 | 覆盖 URL 路径中指定的模型（部分场景可用） |
| `style_preset` | string | 否 | 无 | 预设风格（见下方风格列表） |

**示例请求**

```bash
curl -X POST \
  https://api.stability.ai/v2beta/stable-image/generate/sd3.5-large \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Accept: image/*" \
  -F "prompt=A majestic lion standing on a cliff at sunset, cinematic lighting, 8k" \
  -F "output_format=png" \
  -F "aspect_ratio=16:9" \
  -F "seed=42" \
  -o lion.png
```

**示例请求（JSON 响应）**

```bash
curl -X POST \
  https://api.stability.ai/v2beta/stable-image/generate/sd3.5-large \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Accept: application/json" \
  -F "prompt=A majestic lion standing on a cliff at sunset" \
  -F "output_format=png"
```

**JSON 响应格式**

```json
{
  "image": "iVBORw0KGgoAAAANSUhEUgAA...",
  "seed": 42,
  "finish_reason": "SUCCESS"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `image` | string | Base64 编码的图像数据 |
| `seed` | integer | 使用的随机种子 |
| `finish_reason` | string | 完成原因。`"SUCCESS"` 表示成功 |

**二进制响应**

当 `Accept: image/*` 时，直接返回图像二进制数据，Content-Type 为 `image/png`、`image/jpeg` 或 `image/webp`。

---

### 2. 图生图（Image to Image）

基于输入图像和提示词生成新图像。

**请求**

```
POST https://api.stability.ai/v2beta/stable-image/generate/{model}
```

**Headers**

与文生图相同。

**Body 参数（multipart/form-data）**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `prompt` | string | 是 | — | 提示词，描述期望的变化 |
| `image` | file | 是 | — | 输入图像文件（PNG/JPEG/WebP） |
| `negative_prompt` | string | 否 | `""` | 反向提示词 |
| `strength` | float | 否 | `0.5` | 图像变化强度。0.0 = 不变，1.0 = 完全重新生成。范围：0.0 ~ 1.0 |
| `seed` | integer | 否 | 随机 | 随机种子 |
| `output_format` | string | 否 | `"png"` | 输出格式 |
| `mode` | string | 否 | `"image-to-image"` | 生成模式 |
| `style_preset` | string | 否 | 无 | 预设风格 |

**示例请求**

```bash
curl -X POST \
  https://api.stability.ai/v2beta/stable-image/generate/sd3.5-large \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Accept: image/*" \
  -F "prompt=Transform into a watercolor painting style" \
  -F "image=@input.jpg" \
  -F "strength=0.7" \
  -F "output_format=png" \
  -o output.png
```

---

### 3. 局部重绘（Inpaint）

使用遮罩图像指定需要重绘的区域。

**请求**

```
POST https://api.stability.ai/v2beta/stable-image/generate/{model}
```

**Headers**

与文生图相同。

**Body 参数（multipart/form-data）**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `prompt` | string | 是 | — | 描述重绘区域的内容 |
| `image` | file | 是 | — | 原始图像文件 |
| `mask` | file | 是 | — | 遮罩图像（白色区域为需要重绘的部分，PNG 格式推荐） |
| `negative_prompt` | string | 否 | `""` | 反向提示词 |
| `seed` | integer | 否 | 随机 | 随机种子 |
| `output_format` | string | 否 | `"png"` | 输出格式 |
| `mode` | string | 否 | `"inpaint"` | 生成模式 |

**示例请求**

```bash
curl -X POST \
  https://api.stability.ai/v2beta/stable-image/generate/sd3.5-large \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Accept: image/*" \
  -F "prompt=A beautiful blue sky with fluffy clouds" \
  -F "image=@original.png" \
  -F "mask=@mask.png" \
  -F "mode=inpaint" \
  -o inpainted.png
```

---

### 4. 外扩（Outpaint）

扩展图像边界，在原始图像外部生成新内容。

**请求**

```
POST https://api.stability.ai/v2beta/stable-image/generate/{model}
```

**Body 参数**

与局部重绘类似，但 `mode` 设置为 `"outpaint"`。可通过 `left`、`right`、`up`、`down` 参数指定各方向的扩展像素数。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `prompt` | string | 是 | — | 描述扩展区域的内容 |
| `image` | file | 是 | — | 原始图像 |
| `left` | integer | 否 | `0` | 左侧扩展像素数 |
| `right` | integer | 否 | `0` | 右侧扩展像素数 |
| `up` | integer | 否 | `0` | 上方扩展像素数 |
| `down` | integer | 否 | `0` | 下方扩展像素数 |
| `seed` | integer | 否 | 随机 | 随机种子 |
| `output_format` | string | 否 | `"png"` | 输出格式 |
| `mode` | string | 否 | `"outpaint"` | 生成模式 |

---

### 5. 超分辨率（Upscale）

放大图像并增强细节。

**请求**

```
POST https://api.stability.ai/v2beta/stable-image/upscale/{model}
```

**可用 Upscale 模型**

| 模型 ID | 说明 |
|---------|------|
| `creative` | 创意增强（添加新细节） |
| `fast` | 快速放大 |
| `conservative` | 保守放大（保持原始风格） |

**Body 参数**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `image` | file | 是 | — | 需要放大的图像 |
| `prompt` | string | 否 | `""` | 提示词，引导放大时的细节增强方向（`creative` 模式推荐） |
| `negative_prompt` | string | 否 | `""` | 反向提示词 |
| `seed` | integer | 否 | 随机 | 随机种子 |
| `output_format` | string | 否 | `"png"` | 输出格式 |
| `creativity` | float | 否 | `0.5` | 创意度（仅 `creative` 模式）。范围：0.0 ~ 1.0 |

**示例请求**

```bash
curl -X POST \
  https://api.stability.ai/v2beta/stable-image/upscale/creative \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Accept: image/*" \
  -F "image=@low_res.png" \
  -F "prompt=Enhance details, add texture" \
  -F "creativity=0.7" \
  -o upscaled.png
```

**注意**：Upscale 结果可能是异步的。如果图像较大，API 可能返回 `202 Accepted`，需要轮询获取结果。

---

### 6. 背景移除（Remove Background）

自动识别并移除图像背景。

**请求**

```
POST https://api.stability.ai/v2beta/stable-image/remove-bg
```

**Body 参数**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `image` | file | 是 | — | 输入图像 |
| `output_format` | string | 否 | `"png"` | 输出格式（推荐 PNG 以保留透明度） |

**示例请求**

```bash
curl -X POST \
  https://api.stability.ai/v2beta/stable-image/remove-bg \
  -H "Authorization: Bearer sk-xxxxxxxx" \
  -H "Accept: image/*" \
  -F "image=@photo.jpg" \
  -F "output_format=png" \
  -o no_bg.png
```

---

### 7. 搜索替换（Search and Replace）

在图像中查找特定内容并替换为提示词描述的新内容。

**请求**

```
POST https://api.stability.ai/v2beta/stable-image/generate/{model}
```

**Body 参数**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `prompt` | string | 是 | — | 替换后的内容描述 |
| `image` | file | 是 | — | 原始图像 |
| `search_prompt` | string | 是 | — | 要查找/替换的内容描述 |
| `negative_prompt` | string | 否 | `""` | 反向提示词 |
| `seed` | integer | 否 | 随机 | 随机种子 |
| `output_format` | string | 否 | `"png"` | 输出格式 |
| `mode` | string | 否 | `"search-and-replace"` | 生成模式 |

## 风格预设（Style Presets）

以下风格预设可用于 `style_preset` 参数：

| 类别 | 预设名称 |
|------|---------|
| 3D | `3d-model` |
| 摄影风格 | `analog-film`, `cinematic`, `photographic` |
| 动漫 | `anime`, `comic-book`, `line-art` |
| 数字艺术 | `digital-art`, `enhance`, `isometric`, `low-poly`, `pixel-art` |
| 绘画 | `craft-clay`, `fantasy-art`, `neon-punk`, `origami`, `stained-glass`, `watercolor`, `tile-texture` |
| 其他 | `anime`, `aztec-jungle`, `blockprint`, `collage`, `conceptual-art`, `diagram`, `fashion`, `filmic`, ` hdr`, `tile-texture`, `thriller`, `ukiyo-e`, `vintage`, `vivid-colors` |

## SDK 参数参考

Stability AI 提供官方 Python SDK（`stability-sdk`），以下参数在 SDK 和 REST API 中通用：

### 通用参数

| 参数 | CLI 参数 | 类型 | 默认值 | 说明 |
|------|---------|------|--------|------|
| 宽度 | `--width` | integer | 1024 | 输出图像宽度（像素） |
| 高度 | `--height` | integer | 1024 | 输出图像高度（像素） |
| CFG Scale | `--cfg_scale` | float | 7.0 | 提示词相关性/分类器自由引导尺度。范围：1 ~ 35 |
| 采样器 | `--sampler` | string | 自动 | 采样算法（见下方采样器列表） |
| 步数 | `--steps` | integer | 30 | 扩散步数。范围：10 ~ 50（Turbo 模型推荐 4） |
| 种子 | `--seed` | integer | 随机 | 随机种子。范围：0 ~ 4294967294 |
| 初始图像 | `--init_image` | file | 无 | 图生图输入图像 |
| 遮罩图像 | `--mask_image` | file | 无 | 局部重绘遮罩 |
| 引擎 | `--engine` | string | 自动 | 模型/引擎 ID |
| 样式预设 | `--style_preset` | string | 无 | 风格预设 |

### 采样器（Sampler）

| 采样器 ID | 说明 |
|-----------|------|
| `ddim` | DDIM 采样器 |
| `plms` | PLMS 采样器 |
| `k_euler` | Euler 采样器 |
| `k_euler_ancestral` | Euler Ancestral 采样器 |
| `k_heun` | Heun 采样器 |
| `k_dpm_2` | DPM2 采样器 |
| `k_dpm_2_ancestral` | DPM2 Ancestral 采样器 |
| `k_dpmpp_2s_ancestral` | DPM++ 2S Ancestral 采样器 |
| `k_dpmpp_sde` | DPM++ SDE 采样器 |
| `k_dpmpp_2m` | DPM++ 2M 采样器 |
| `k_dpmpp_2m_sde` | DPM++ 2M SDE 采样器 |
| `k_dpmpp_3m_sde` | DPM++ 3M SDE 采样器 |

## 错误处理

### HTTP 状态码

| 状态码 | 含义 |
|--------|------|
| `200` | 成功 |
| `202` | 已接受（异步任务，需轮询） |
| `400` | 请求参数错误 |
| `401` | 认证失败（API Key 无效） |
| `402` | Credit 不足 |
| `403` | 无权限 |
| `404` | 模型/端点不存在 |
| `413` | 请求体过大 |
| `429` | 请求频率超限 |
| `500` | 服务器内部错误 |

### 错误响应格式

```json
{
  "id": "6a6c3a9e-ee82-4e94-b36e-5b3f14e4c631",
  "message": "Invalid request: prompt is required",
  "name": "bad_request"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 错误唯一标识 |
| `message` | string | 错误描述 |
| `name` | string | 错误类型名称 |

### 常见错误类型

| 错误类型 name | HTTP 状态码 | 说明 |
|--------------|------------|------|
| `bad_request` | 400 | 请求参数错误 |
| `unauthorized` | 401 | API Key 无效或缺失 |
| `insufficient_credits` | 402 | Credit 余额不足 |
| `permission_denied` | 403 | 无权访问该模型/功能 |
| `not_found` | 404 | 资源不存在 |
| `rate_limit_exceeded` | 429 | 请求频率超限 |
| `server_error` | 500 | 服务器内部错误 |

## 异步操作

### 提交异步任务

对于耗时操作（如 Upscale），API 可能返回 `202 Accepted`：

```json
{
  "id": "abc123",
  "status": "in-progress"
}
```

### 轮询结果

```
GET https://api.stability.ai/v2beta/stable-image/results/{id}
```

**Headers**

```
Authorization: Bearer sk-xxx
Accept: image/* 或 application/json
```

### 响应状态

| 状态 | 说明 |
|------|------|
| `in-progress` | 正在处理 |
| `complete` | 已完成（返回图像） |
| `failed` | 处理失败 |

### 轮询建议

- 使用指数退避策略
- 建议初始间隔 1 秒，最大间隔 10 秒
- 总超时建议 5 分钟

## 速率限制

| 计划 | 并发请求数 | 每秒请求数 |
|------|-----------|-----------|
| Free | 2 | 2 |
| Basic | 4 | 4 |
| Standard | 8 | 8 |
| Premium | 16 | 16 |
| Enterprise | 自定义 | 自定义 |

速率限制响应头：

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1677652288
```

## 图像尺寸与限制

### 支持的宽高比

| 宽高比 | 尺寸（像素） | 说明 |
|--------|-------------|------|
| `1:1` | 1024 × 1024 | 正方形（默认） |
| `16:9` | 1344 × 768 | 横屏宽幅 |
| `21:9` | 1536 × 640 | 超宽屏 |
| `2:3` | 768 × 1152 | 竖屏 |
| `3:2` | 1152 × 768 | 横屏 |
| `4:5` | 896 × 1088 | 近正方形竖屏 |
| `5:4` | 1088 × 896 | 近正方形横屏 |
| `9:16` | 768 × 1344 | 竖屏长幅 |
| `9:21` | 640 × 1536 | 超长竖屏 |

### 文件大小限制

- 输入图像：最大 10MB
- 输出图像：最大 20MB

## Python SDK 使用示例

### 安装

```bash
pip install stability-sdk
```

### 文生图

```python
import io
import requests
from PIL import Image

response = requests.post(
    "https://api.stability.ai/v2beta/stable-image/generate/sd3.5-large",
    headers={
        "Authorization": f"Bearer {api_key}",
        "Accept": "image/*"
    },
    files={"none": ""},
    data={
        "prompt": "A beautiful mountain landscape at golden hour",
        "output_format": "png",
        "aspect_ratio": "16:9",
    }
)

if response.status_code == 200:
    with open("output.png", "wb") as f:
        f.write(response.content)
else:
    print(f"Error: {response.status_code}")
    print(response.json())
```

### 图生图

```python
response = requests.post(
    "https://api.stability.ai/v2beta/stable-image/generate/sd3.5-large",
    headers={
        "Authorization": f"Bearer {api_key}",
        "Accept": "image/*"
    },
    files={
        "image": open("input.jpg", "rb")
    },
    data={
        "prompt": "Transform into oil painting style",
        "strength": 0.7,
        "output_format": "png",
    }
)
```

### 局部重绘

```python
response = requests.post(
    "https://api.stability.ai/v2beta/stable-image/generate/sd3.5-large",
    headers={
        "Authorization": f"Bearer {api_key}",
        "Accept": "image/*"
    },
    files={
        "image": open("original.png", "rb"),
        "mask": open("mask.png", "rb"),
    },
    data={
        "prompt": "A clear blue sky with white clouds",
        "output_format": "png",
    }
)
```

## Go HTTP 调用示例

```go
package main

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

func generateImage(apiKey, prompt, model, outputPath string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	_ = writer.WriteField("prompt", prompt)
	_ = writer.WriteField("output_format", "png")
	_ = writer.WriteField("aspect_ratio", "1:1")

	// 必须有一个 none 字段（当没有文件上传时）
	part, _ := writer.CreateFormFile("none", "")
	part.Write([]byte{})

	writer.Close()

	url := fmt.Sprintf("https://api.stability.ai/v2beta/stable-image/generate/%s", model)
	req, _ := http.NewRequest("POST", url, &buf)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "image/*")
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, body)
	}

	file, _ := os.Create(outputPath)
	defer file.Close()
	io.Copy(file, resp.Body)

	return nil
}
```

## 与 OpenAI Images API 的对比

| 特性 | Stability AI | OpenAI Images |
|------|-------------|---------------|
| 认证方式 | Bearer Token | Bearer Token |
| 请求格式 | multipart/form-data | JSON |
| 文生图 | `/v2beta/stable-image/generate/{model}` | `/v1/images/generations` |
| 图生图 | 同端点 + image 文件 | `/v1/images/edits` |
| 局部重绘 | 同端点 + mask 文件 | `/v1/images/edits` + mask |
| 超分辨率 | `/v2beta/stable-image/upscale/{model}` | 不支持 |
| 背景移除 | `/v2beta/stable-image/remove-bg` | 不支持 |
| 响应格式 | 二进制或 JSON（base64） | JSON（URL 或 base64） |
| 流式支持 | 不支持 | gpt-image-1 支持 SSE |
| 模型数量 | 10+ | 5（DALL-E 2/3, GPT Image 系列） |
| 风格控制 | style_preset + sampler + steps | quality + style |
| 免费额度 | 初始 Credit | 无（按次付费） |

## team-api 适配器实现参考

### 端点映射

| team-api 入口 | Stability AI 上游 |
|--------------|------------------|
| `POST /v1/images/generations` | `POST /v2beta/stable-image/generate/{model}` |
| `POST /v1/images/edits` | `POST /v2beta/stable-image/generate/{model}` (mode=image-to-image) |
| `POST /v1/images/variations` | `POST /v2beta/stable-image/generate/{model}` (mode=image-to-image) |

### 请求转换要点

1. **认证转换**：OpenAI 格式 `Bearer sk-xxx` → Stability 格式 `Bearer sk-xxx`（直接透传 API Key）
2. **请求体转换**：
   - OpenAI JSON body → multipart/form-data
   - `size` → `aspect_ratio`（需要尺寸映射逻辑）
   - `n` 参数：Stability API 不支持批量生成，需要循环调用
   - `response_format` → `Accept` header（`url` → 不支持，`b64_json` → `application/json`）
3. **响应体转换**：
   - Stability 二进制响应 → OpenAI JSON 格式（base64 编码）
   - Stability JSON 响应 → 提取 `image` 字段 → 放入 OpenAI `data[].b64_json`

### DTO 参考

```go
// Stability AI 请求参数（转换后）
type StabilityImageRequest struct {
	Prompt         string  `json:"prompt"`
	NegativePrompt string  `json:"negative_prompt,omitempty"`
	AspectRatio    string  `json:"aspect_ratio,omitempty"`
	Seed           uint32  `json:"seed,omitempty"`
	OutputFormat   string  `json:"output_format,omitempty"`
	Mode           string  `json:"mode,omitempty"`
	Strength       float64 `json:"strength,omitempty"`
	StylePreset    string  `json:"style_preset,omitempty"`
}

// Stability AI JSON 响应
type StabilityImageResponse struct {
	Image        string `json:"image"`
	Seed         uint32 `json:"seed"`
	FinishReason string `json:"finish_reason"`
}

// Stability AI 错误响应
type StabilityError struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Name    string `json:"name"`
}
```

### 尺寸映射表

| OpenAI size | Stability aspect_ratio |
|-------------|----------------------|
| `256x256` | `1:1`（SDXL 不支持 256，降质） |
| `512x512` | `1:1`（SDXL 不支持 512，降质） |
| `1024x1024` | `1:1` |
| `1792x1024` | `16:9` |
| `1024x1792` | `9:16` |
| `1536x1024` | `3:2` |
| `1024x1536` | `2:3` |
