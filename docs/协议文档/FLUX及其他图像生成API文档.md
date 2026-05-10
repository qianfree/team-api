# FLUX 及其他图像生成 API 文档

## 概述

本文档涵盖 FLUX 模型系列及通过第三方平台（Together AI、Replicate、Fireworks AI、Black Forest Labs）提供的图像生成 API。

---

## 一、Black Forest Labs（BFL）— FLUX 模型官方 API

### 概述

Black Forest Labs（BFL）是 FLUX 模型的创建者，由 Stable Diffusion 原始团队成员创立。FLUX 系列是目前开源图像生成领域最先进的模型之一。

- **官网**：https://blackforestlabs.ai
- **API 文档**：https://docs.bfl.ml
- **API Base URL**：`https://api.bfl.ml`
- **认证方式**：Bearer Token（API Key）
- **协议格式**：REST + JSON

### 可用模型

| 模型 ID | 说明 | 特点 |
|---------|------|------|
| `flux-pro-1.1` | FLUX.1.1 Pro | 高质量，6 步生成 |
| `flux-pro-1.0` | FLUX.1 Pro | 旗舰级质量 |
| `flux-dev-1.0` | FLUX.1 Dev | 开发者版本，可自部署 |
| `flux-schnell-1.0` | FLUX.1 Schnell | 快速版，4 步生成，Apache 2.0 开源 |
| `flux-kontext-1.0` | FLUX.1 Kontext | 图像编辑（inpainting/outpainting） |
| `flux-canny-1.0` | FLUX.1 Canny | Canny 边缘引导生成 |
| `flux-depth-1.0` | FLUX.1 Depth | 深度图引导生成 |
| `flux-fill-1.0` | FLUX.1 Fill | 局部重绘/填充 |

### 认证

```
X-Key: sk-xxxxxxxxxxxxxxxxxxxxxxxx
```

在请求头中使用 `X-Key` 字段传递 API Key。

### API 端点

#### 1. 文生图（Text to Image）

**请求**

```
POST https://api.bfl.ml/v1/flux-pro-1.1
```

**Headers**

| Header | 值 | 必填 |
|--------|---|------|
| `Content-Type` | `application/json` | 是 |
| `X-Key` | `sk-xxx` | 是 |

**请求体**

```json
{
  "prompt": "A beautiful sunset over a calm ocean with vibrant colors",
  "width": 1024,
  "height": 1024,
  "steps": 25,
  "guidance": 3.5,
  "seed": 42,
  "safety_tolerance": 2,
  "output_format": "jpeg",
  "output_quality": 80
}
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `prompt` | string | 是 | — | 正向提示词 |
| `width` | integer | 否 | `1024` | 图像宽度。推荐值：512 ~ 1536，必须是 16 的倍数 |
| `height` | integer | 否 | `1024` | 图像高度。推荐值：512 ~ 1536，必须是 16 的倍数 |
| `steps` | integer | 否 | `25` | 推理步数。范围：1 ~ 50（Schnell 推荐 4） |
| `guidance` | float | 否 | `3.0` | CFG 引导尺度。范围：1.0 ~ 20.0 |
| `seed` | integer | 否 | 随机 | 随机种子。范围：0 ~ 4294967295 |
| `safety_tolerance` | integer | 否 | `2` | 安全过滤级别。范围：1 ~ 6，1=最严格，6=最宽松 |
| `output_format` | string | 否 | `"jpeg"` | 输出格式。可选值：`"jpeg"`, `"png"` |
| `output_quality` | integer | 否 | `80` | JPEG 输出质量。范围：1 ~ 100（仅 JPEG 有效） |

#### 2. 图生图（Image to Image）

**请求**

```
POST https://api.bfl.ml/v1/flux-pro-1.1-ultra
```

**请求体**

```json
{
  "prompt": "Transform into a watercolor painting",
  "image": "base64_encoded_image_data",
  "image_prompt": "watercolor style artwork",
  "strength": 0.7,
  "width": 1024,
  "height": 1024,
  "steps": 25,
  "guidance": 3.5,
  "seed": 42,
  "output_format": "png"
}
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `prompt` | string | 是 | — | 提示词 |
| `image` | string | 是 | — | Base64 编码的输入图像 |
| `image_prompt` | string | 否 | `""` | 图像提示词（额外引导） |
| `strength` | float | 否 | `0.5` | 图像变化强度。范围：0.0 ~ 1.0 |

#### 3. 局部重绘（Inpainting / Fill）

```
POST https://api.bfl.ml/v1/flux-fill-1.0
```

```json
{
  "prompt": "A beautiful garden with flowers",
  "image": "base64_encoded_image",
  "mask": "base64_encoded_mask",
  "width": 1024,
  "height": 1024,
  "steps": 25,
  "guidance": 3.5,
  "seed": 42
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `prompt` | string | 是 | 重绘区域描述 |
| `image` | string | 是 | Base64 编码的原始图像 |
| `mask` | string | 是 | Base64 编码的遮罩图像（白色区域为重绘区域） |

#### 4. Canny 边缘引导生成

```
POST https://api.bfl.ml/v1/flux-canny-1.0
```

```json
{
  "prompt": "A futuristic city skyline",
  "control_image": "base64_encoded_control_image",
  "guidance": 3.5,
  "steps": 25,
  "seed": 42
}
```

#### 5. 深度图引导生成

```
POST https://api.bfl.ml/v1/flux-depth-1.0
```

```json
{
  "prompt": "A cozy living room",
  "control_image": "base64_encoded_depth_map",
  "guidance": 3.5,
  "steps": 25,
  "seed": 42
}
```

### 异步响应

BFL API 默认返回异步任务：

```json
{
  "id": "abc123-def456",
  "status": "Pending",
  "result": null
}
```

**轮询结果**

```
GET https://api.bfl.ml/v1/get_result?id=abc123-def456
```

**Headers**

```
X-Key: sk-xxx
```

**完成响应**

```json
{
  "id": "abc123-def456",
  "status": "Ready",
  "result": {
    "sample": "https://delivery.bfl.ml/xxx/image.jpeg",
    "prompt": "A beautiful sunset..."
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 任务 ID |
| `status` | string | 任务状态：`"Pending"`、`"Ready"`、`"Failed"` |
| `result.sample` | string | 生成图像的 URL |
| `result.prompt` | string | 使用的提示词 |

### 错误响应

```json
{
  "status_code": 400,
  "errors": [
    {
      "message": "Invalid request: prompt is required"
    }
  ]
}
```

### 定价（参考）

| 模型 | 每张价格 | 说明 |
|------|---------|------|
| FLUX.1.1 Pro | ~$0.04 | 高质量 |
| FLUX.1 Pro | ~$0.05 | 旗舰级 |
| FLUX.1 Schnell | ~$0.003 | 快速/低成本 |

---

## 二、Together AI — FLUX/SDXL 图像生成

### 概述

Together AI 提供多种开源图像模型的托管推理服务，包括 FLUX 系列、Stable Diffusion XL 等。API 兼容 OpenAI Images 格式。

- **官网**：https://together.ai
- **API 文档**：https://docs.together.ai
- **API Base URL**：`https://api.together.xyz`
- **认证方式**：Bearer Token
- **协议格式**：兼容 OpenAI Images API

### 认证

```
Authorization: Bearer {TOGETHER_API_KEY}
```

### 可用图像模型

| 模型 ID | 说明 | 每步价格 | 质量 |
|---------|------|---------|------|
| `black-forest-labs/FLUX.1-schnell` | FLUX.1 Schnell（免费） | $0.00 | 快速 |
| `black-forest-labs/FLUX.1-schnell-Free` | FLUX.1 Schnell 免费版 | $0.00 | 快速（限速） |
| `black-forest-labs/FLUX.1.1-pro` | FLUX.1.1 Pro | 按量计费 | 高质量 |
| `black-forest-labs/FLUX.1-pro` | FLUX.1 Pro | 按量计费 | 旗舰级 |
| `stabilityai/stable-diffusion-xl-base-1.0` | SDXL 1.0 Base | 按量计费 | 中等质量 |

### API 端点

#### 图像生成

```
POST https://api.together.xyz/v1/images/generations
```

**请求体**

```json
{
  "model": "black-forest-labs/FLUX.1-schnell",
  "prompt": "A cat sitting on a windowsill looking at the moon",
  "n": 1,
  "size": "1024x1024",
  "steps": 4,
  "seed": 42,
  "response_format": "b64_json",
  "guidance_scale": 7.5
}
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `model` | string | 是 | — | 模型 ID（见上方模型列表） |
| `prompt` | string | 是 | — | 提示词 |
| `n` | integer | 否 | `1` | 生成图像数量（最大 8） |
| `size` | string | 否 | `"1024x1024"` | 图像尺寸。支持：`"256x256"`, `"512x512"`, `"1024x1024"`, `"1024x768"`, `"768x1024"`, `"1792x1024"`, `"1024x1792"` |
| `steps` | integer | 否 | `20` | 推理步数。FLUX Schnell 推荐 4 |
| `seed` | integer | 否 | 随机 | 随机种子 |
| `response_format` | string | 否 | `"url"` | 响应格式。可选值：`"url"`, `"b64_json"` |
| `guidance_scale` | float | 否 | `7.5` | CFG 引导尺度 |
| `negative_prompt` | string | 否 | `""` | 反向提示词（部分模型支持） |
| `width` | integer | 否 | 1024 | 图像宽度（可替代 size） |
| `height` | integer | 否 | 1024 | 图像高度（可替代 size） |

**响应**

```json
{
  "id": "gen-abc123",
  "object": "list",
  "created": 1677652288,
  "data": [
    {
      "url": "https://api.together.xyz/images/abc123.png",
      "b64_json": null
    }
  ]
}
```

或（`response_format: "b64_json"`）：

```json
{
  "id": "gen-abc123",
  "object": "list",
  "created": 1677652288,
  "data": [
    {
      "url": null,
      "b64_json": "iVBORw0KGgoAAAANSUhEUgAA..."
    }
  ]
}
```

**示例请求（cURL）**

```bash
curl -X POST https://api.together.xyz/v1/images/generations \
  -H "Authorization: Bearer $TOGETHER_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "black-forest-labs/FLUX.1-schnell",
    "prompt": "A serene Japanese garden with cherry blossoms",
    "n": 1,
    "size": "1024x1024",
    "steps": 4,
    "response_format": "url"
  }'
```

**示例请求（Python）**

```python
from openai import OpenAI

client = OpenAI(
    api_key="your_together_api_key",
    base_url="https://api.together.xyz/v1"
)

response = client.images.generate(
    model="black-forest-labs/FLUX.1-schnell",
    prompt="A serene Japanese garden with cherry blossoms",
    n=1,
    size="1024x1024",
    response_format="url"
)

print(response.data[0].url)
```

### 错误响应

与 OpenAI 格式一致：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid model ID",
    "param": "model",
    "code": null
  }
}
```

### 定价

| 模型 | 计费方式 |
|------|---------|
| FLUX.1 Schnell Free | 免费（有限速率） |
| FLUX.1 Schnell | $0.002/张 |
| FLUX.1.1 Pro | ~$0.03/张 |
| FLUX.1 Pro | ~$0.04/张 |
| SDXL 1.0 | ~$0.002/张 |

---

## 三、Replicate — 通用模型推理平台

### 概述

Replicate 提供云端模型推理服务，支持数千种模型（图像、视频、音频、文本）。图像生成是其核心功能之一，支持 FLUX、SDXL、Kandinsky、Playground 等多种模型。

- **官网**：https://replicate.com
- **API 文档**：https://replicate.com/docs
- **API Base URL**：`https://api.replicate.com`
- **认证方式**：Bearer Token
- **协议格式**：REST + JSON

### 认证

```
Authorization: Bearer r8_xxxxxxxxxxxxxxxxxxxxxxxx
```

在 https://replicate.com/account/api-tokens 获取 API Token。

### API 端点

#### 1. 创建预测（同步）

```
POST https://api.replicate.com/v1/predictions
```

**Headers**

| Header | 值 | 必填 |
|--------|---|------|
| `Authorization` | `Bearer r8_xxx` | 是 |
| `Content-Type` | `application/json` | 是 |
| `Prefer` | `wait` | 否 | 设置为 `wait` 表示同步等待结果（最长 60s） |

**请求体**

```json
{
  "version": "ac732df83cea7fff18b947a8b9a204a33cd3f2f1e5e6b1e5f5f5f5f5f5f5f5f5",
  "input": {
    "prompt": "A beautiful sunset over mountains",
    "width": 1024,
    "height": 1024,
    "num_outputs": 1,
    "guidance_scale": 7.5,
    "num_inference_steps": 28,
    "seed": 42
  }
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `version` | string | 否 | 模型版本 ID（与 `model` 二选一） |
| `model` | string | 否 | 模型标识（如 `"black-forest-labs/flux-schnell"`） |
| `input` | object | 是 | 模型输入参数（因模型而异） |
| `webhook` | string | 否 | 完成/失败时的回调 URL |
| `webhook_events_filter` | array | 否 | 触发回调的事件：`["start", "output", "logs", "completed"]` |
| `Cancel-After` | string | 否 | 超时时间（ISO 8601 duration，如 `"PT60S"`） |

#### 2. 创建预测（使用模型最新版本）

```
POST https://api.replicate.com/v1/models/{owner}/{model}/predictions
```

**请求体**

```json
{
  "input": {
    "prompt": "A cat wearing a hat",
    "width": 1024,
    "height": 768,
    "num_inference_steps": 28,
    "guidance_scale": 3.5,
    "seed": 42
  }
}
```

#### 3. 获取预测结果

```
GET https://api.replicate.com/v1/predictions/{prediction_id}
```

#### 4. 取消预测

```
POST https://api.replicate.com/v1/predictions/{prediction_id}/cancel
```

#### 5. 列出预测

```
GET https://api.replicate.com/v1/predictions
```

### 预测响应

```json
{
  "id": "abc123-def456-ghi789",
  "model": "black-forest-labs/flux-schnell",
  "version": "ac732df83cea...",
  "input": {
    "prompt": "A beautiful sunset"
  },
  "output": [
    "https://replicate.delivery/xxx/image1.png"
  ],
  "status": "succeeded",
  "created_at": "2024-01-15T10:30:00.000Z",
  "started_at": "2024-01-15T10:30:01.000Z",
  "completed_at": "2024-01-15T10:30:15.000Z",
  "logs": "Using seed: 42\nGenerating...",
  "error": null,
  "metrics": {
    "predict_time": 14.5
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | string | 预测唯一 ID |
| `model` | string | 模型标识 |
| `version` | string | 模型版本 ID |
| `input` | object | 输入参数 |
| `output` | array | 输出结果（通常是图像 URL 数组） |
| `status` | string | 状态：`"starting"`, `"processing"`, `"succeeded"`, `"failed"`, `"canceled"` |
| `created_at` | string | 创建时间（ISO 8601） |
| `started_at` | string | 开始处理时间 |
| `completed_at` | string | 完成时间 |
| `logs` | string | 执行日志 |
| `error` | string | 错误信息（失败时） |
| `metrics` | object | 性能指标 |

### 状态流转

```
starting → processing → succeeded
                      → failed
                      → canceled（手动取消）
```

### Webhook 回调

当预测完成时，Replicate 向指定 URL 发送 POST 请求：

```json
{
  "id": "abc123",
  "event": "completed",
  "status": "succeeded",
  "output": ["https://replicate.delivery/xxx/image.png"],
  "error": null
}
```

### 常用图像模型及输入参数

#### FLUX.1 Schnell

```json
{
  "model": "black-forest-labs/flux-schnell",
  "input": {
    "prompt": "description of the image",
    "num_outputs": 1,
    "aspect_ratio": "1:1",
    "output_format": "webp",
    "output_quality": 80,
    "seed": 42
  }
}
```

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `prompt` | string | — | 提示词 |
| `num_outputs` | integer | 1 | 输出图像数量（1 ~ 4） |
| `aspect_ratio` | string | `"1:1"` | 宽高比：`"1:1"`, `"16:9"`, `"9:16"`, `"4:3"`, `"3:4"`, `"3:2"`, `"2:3"` |
| `output_format` | string | `"webp"` | 输出格式：`"webp"`, `"jpg"`, `"png"` |
| `output_quality` | integer | 80 | 输出质量（1 ~ 100） |
| `seed` | integer | 随机 | 随机种子 |

#### FLUX.1 Pro

```json
{
  "model": "black-forest-labs/flux-pro",
  "input": {
    "prompt": "description",
    "width": 1024,
    "height": 1024,
    "steps": 28,
    "guidance": 3.5,
    "interval": 0.5,
    "seed": 42
  }
}
```

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `prompt` | string | — | 提示词 |
| `width` | integer | 1024 | 图像宽度 |
| `height` | integer | 1024 | 图像高度 |
| `steps` | integer | 28 | 推理步数 |
| `guidance` | float | 3.5 | CFG 引导尺度 |
| `interval` | float | 0.5 | 引导间隔 |
| `seed` | integer | 随机 | 随机种子 |

#### SDXL

```json
{
  "model": "stability-ai/sdxl",
  "input": {
    "prompt": "description",
    "negative_prompt": "blurry, bad quality",
    "width": 1024,
    "height": 1024,
    "num_inference_steps": 30,
    "guidance_scale": 7.5,
    "scheduler": "K_EULER",
    "seed": 42,
    "num_outputs": 1
  }
}
```

### 错误响应

```json
{
  "status": 422,
  "detail": "Input validation failed: prompt is required"
}
```

### 定价

Replicate 按推理时间计费，不同模型价格不同：

| 模型 | 每秒价格 | 典型生成时间 | 每张价格 |
|------|---------|-------------|---------|
| FLUX.1 Schnell | ~$0.0003/s | ~2s | ~$0.003 |
| FLUX.1 Pro | ~$0.0003/s | ~10s | ~$0.03 |
| SDXL | ~$0.0003/s | ~5s | ~$0.01 |

### Python SDK

```python
import replicate

# 方式1：使用模型标识
output = replicate.run(
    "black-forest-labs/flux-schnell",
    input={
        "prompt": "A cat in a garden",
        "aspect_ratio": "16:9",
        "output_format": "png"
    }
)

# output 是图像 URL 列表
print(output[0])  # https://replicate.delivery/xxx/image.png

# 方式2：异步预测
prediction = replicate.predictions.create(
    model="black-forest-labs/flux-schnell",
    input={"prompt": "A cat in a garden"},
    webhook="https://your-app.com/webhook",
    webhook_events_filter=["completed"]
)

# 轮询
prediction.reload()
while prediction.status not in ["succeeded", "failed", "canceled"]:
    time.sleep(1)
    prediction.reload()
```

### Go HTTP 调用示例

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type PredictionRequest struct {
	Model string                 `json:"model"`
	Input map[string]interface{} `json:"input"`
}

type Prediction struct {
	ID     string        `json:"id"`
	Status string        `json:"status"`
	Output []string      `json:"output"`
	Error  string        `json:"error"`
}

func createPrediction(apiKey string, req PredictionRequest) (*Prediction, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST",
		"https://api.replicate.com/v1/predictions",
		bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Prefer", "wait")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pred Prediction
	json.NewDecoder(resp.Body).Decode(&pred)
	return &pred, nil
}
```

---

## 四、Fireworks AI — 图像生成

### 概述

Fireworks AI 提供快速推理服务，支持 FLUX、Stable Diffusion 等多种图像生成模型，以低延迟和高吞吐著称。

- **官网**：https://fireworks.ai
- **API 文档**：https://docs.fireworks.ai
- **API Base URL**：`https://api.fireworks.ai`
- **认证方式**：Bearer Token
- **协议格式**：兼容 OpenAI Images API

### 认证

```
Authorization: Bearer {FIREWORKS_API_KEY}
```

### 可用模型

| 模型 ID | 说明 |
|---------|------|
| `stable-diffusion-xl-1024-v1-0` | SDXL 1.0 |
| `FLUX.1-schnell` | FLUX Schnell 快速版 |
| `FLUX.1-dev` | FLUX Dev 开发者版 |

### API 端点

#### 图像生成

```
POST https://api.fireworks.ai/v1/images/generations
```

**请求体**

```json
{
  "model": "FLUX.1-schnell",
  "prompt": "A dragon flying over a medieval castle",
  "n": 1,
  "size": "1024x1024",
  "cfg_scale": 7.5,
  "steps": 4,
  "seed": 42,
  "sampler": "DDIM"
}
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `model` | string | 是 | — | 模型 ID |
| `prompt` | string | 是 | — | 提示词 |
| `n` | integer | 否 | `1` | 生成数量 |
| `size` | string | 否 | `"1024x1024"` | 图像尺寸 |
| `cfg_scale` | float | 否 | `7.0` | CFG 引导尺度 |
| `steps` | integer | 否 | `20` | 推理步数 |
| `seed` | integer | 否 | 随机 | 随机种子 |
| `sampler` | string | 否 | 自动 | 采样器类型 |
| `negative_prompt` | string | 否 | `""` | 反向提示词 |

**响应**

兼容 OpenAI Images API 格式：

```json
{
  "created": 1677652288,
  "data": [
    {
      "url": "https://fireworks-api-images.s3.amazonaws.com/xxx/image.png"
    }
  ]
}
```

### 示例请求

```bash
curl -X POST https://api.fireworks.ai/v1/images/generations \
  -H "Authorization: Bearer $FIREWORKS_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "FLUX.1-schnell",
    "prompt": "A majestic phoenix rising from flames",
    "n": 1,
    "size": "1024x1024",
    "steps": 4
  }'
```

### Python 示例（兼容 OpenAI SDK）

```python
from openai import OpenAI

client = OpenAI(
    api_key="your_fireworks_api_key",
    base_url="https://api.fireworks.ai/v1"
)

response = client.images.generate(
    model="FLUX.1-schnell",
    prompt="A majestic phoenix rising from flames",
    n=1,
    size="1024x1024"
)

print(response.data[0].url)
```

---

## 五、各平台对比总结

### 功能对比

| 特性 | BFL 官方 | Together AI | Replicate | Fireworks AI |
|------|---------|-------------|-----------|-------------|
| 认证方式 | `X-Key` Header | Bearer Token | Bearer Token | Bearer Token |
| 请求格式 | JSON | JSON（OpenAI 兼容） | JSON | JSON（OpenAI 兼容） |
| 响应格式 | 异步 + URL | OpenAI 格式 | 异步 + URL | OpenAI 格式 |
| OpenAI SDK 兼容 | 否 | 是 | 否 | 是 |
| 文生图 | ✓ | ✓ | ✓ | ✓ |
| 图生图 | ✓ | 部分支持 | ✓ | 部分支持 |
| 局部重绘 | ✓（Fill 模型） | 部分支持 | ✓ | 部分支持 |
| Canny/Depth 引导 | ✓ | — | ✓ | — |
| Webhook | — | — | ✓ | — |
| 免费额度 | 无 | Schnell 免费 | 无 | 无 |
| FLUX 模型 | 全系列 | Schnell + Pro | 全系列 | Schnell + Dev |
| SDXL | — | ✓ | ✓ | ✓ |

### 价格对比（每张 1024x1024 图像，参考价格）

| 模型 | BFL 官方 | Together AI | Replicate | Fireworks AI |
|------|---------|-------------|-----------|-------------|
| FLUX.1 Schnell | ~$0.003 | 免费 / $0.002 | ~$0.003 | ~$0.002 |
| FLUX.1 Pro | ~$0.05 | ~$0.04 | ~$0.03 | — |
| FLUX.1.1 Pro | ~$0.04 | ~$0.03 | ~$0.035 | — |
| SDXL | — | ~$0.002 | ~$0.01 | ~$0.002 |

### 延迟对比

| 模型 | BFL 官方 | Together AI | Replicate | Fireworks AI |
|------|---------|-------------|-----------|-------------|
| FLUX Schnell | ~2s | ~2s | ~3s | ~1s |
| FLUX Pro | ~10s | ~10s | ~12s | — |
| SDXL | — | ~5s | ~6s | ~3s |

---

## 六、team-api 适配器实现参考

### 端点映射

对于 OpenAI 兼容的供应商（Together AI、Fireworks AI），可以直接透传 `/v1/images/generations` 请求，只需替换 base URL 和 API Key。

| team-api 入口 | Together AI | Fireworks AI | Replicate |
|--------------|-------------|-------------|-----------|
| `POST /v1/images/generations` | `POST /v1/images/generations` | `POST /v1/images/generations` | `POST /v1/predictions` |
| `POST /v1/images/edits` | 不支持 | 不支持 | `POST /v1/predictions` |
| `POST /v1/images/variations` | 不支持 | 不支持 | `POST /v1/predictions` |

### 请求转换（OpenAI 兼容供应商）

Together AI 和 Fireworks AI 的请求格式与 OpenAI 基本一致，适配器只需：

1. 替换 `base_url`（`api.openai.com` → `api.together.xyz` / `api.fireworks.ai`）
2. 替换 `model` 字段（`dall-e-3` → `black-forest-labs/FLUX.1-schnell`）
3. 透传其余参数

### 请求转换（Replicate）

Replicate 的请求格式与 OpenAI 差异较大，需要专用适配器：

```go
// OpenAI Images 请求 → Replicate 预测请求
func ConvertToReplicateRequest(openaiReq *OpenAIImageRequest) *ReplicateRequest {
    input := map[string]interface{}{
        "prompt":       openaiReq.Prompt,
        "aspect_ratio": sizeToAspectRatio(openaiReq.Size),
        "output_format": "png",
        "num_outputs":   openaiReq.N,
    }
    if openaiReq.Seed > 0 {
        input["seed"] = openaiReq.Seed
    }

    return &ReplicateRequest{
        Model: mapModelToReplicate(openaiReq.Model),
        Input: input,
    }
}

// Replicate 响应 → OpenAI Images 响应
func ConvertFromReplicateResponse(resp *ReplicatePrediction) *OpenAIImageResponse {
    data := make([]ImageData, len(resp.Output))
    for i, url := range resp.Output {
        data[i] = ImageData{URL: url}
    }
    return &OpenAIImageResponse{
        Created: time.Now().Unix(),
        Data:    data,
    }
}

// 尺寸映射
func sizeToAspectRatio(size string) string {
    switch size {
    case "1024x1024": return "1:1"
    case "1792x1024": return "16:9"
    case "1024x1792": return "9:16"
    case "1536x1024": return "3:2"
    case "1024x1536": return "2:3"
    default:          return "1:1"
    }
}

// 模型映射
func mapModelToReplicate(model string) string {
    switch model {
    case "flux-schnell": return "black-forest-labs/flux-schnell"
    case "flux-pro":    return "black-forest-labs/flux-pro"
    case "sdxl":        return "stability-ai/sdxl"
    default:            return model
    }
}
```

### Replicate 异步轮询

```go
func PollReplicatePrediction(apiKey, predictionID string, timeout time.Duration) (*ReplicatePrediction, error) {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        pred, err := GetPrediction(apiKey, predictionID)
        if err != nil {
            return nil, err
        }
        switch pred.Status {
        case "succeeded":
            return pred, nil
        case "failed":
            return nil, fmt.Errorf("prediction failed: %s", pred.Error)
        case "canceled":
            return nil, fmt.Errorf("prediction was canceled")
        }
        time.Sleep(1 * time.Second)
    }
    return nil, fmt.Errorf("prediction timed out after %v", timeout)
}
```
