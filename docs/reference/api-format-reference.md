# API 接口格式参考

本文档是 CLAUDE.md 中 API 接口规范的详细参考资料，包含完整的 JSON 示例、响应格式、错误类型映射等。开发时按需查阅。

## 统一响应格式详细示例

**适用范围**：`/api/admin/*`、`/api/tenant/*`、`/api/payment/*`、`/api/open/*`、`/api/status`。

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | int | `0` = 成功，非 `0` = 错误。标准 HTTP 错误直接用状态码（400/401/403/404/409/500），业务错误用 >= 10000 的自定义码 |
| `message` | string | 用户可读的中文提示。成功时固定 `"ok"`，错误时描述具体原因，禁止暴露技术细节 |
| `data` | any | 成功时为业务数据（对象/数组/字符串），错误时为 `null` |
| `request_id` | string | 每个请求的唯一标识，贯穿全链路，用于日志追踪 |

### 响应示例

```json
// 成功（HTTP 200）
{"code": 0, "message": "ok", "data": {"id": 1, "name": "张三"}, "request_id": "req_abc123"}

// 参数错误（HTTP 400）
{"code": 400, "message": "用户名不能为空", "data": null, "request_id": "req_def456"}

// 业务错误（HTTP 422）
{"code": 10001, "message": "余额不足", "data": null, "request_id": "req_ghi789"}
```

### data 字段格式规范

| 接口类型 | data 结构 | 示例 |
|----------|----------|------|
| 分页列表 | `{list, total, page, page_size}` | `{"list": [...], "total": 100, "page": 1, "page_size": 20}` |
| 不分页列表 | `{list}` | `{"list": [...]}` |
| 创建资源 | `{id}` | `{"id": 42}` |
| 资源详情 | 直接返回对象 | `{"id": 1, "name": "张三", ...}` |
| 更新/删除 | `null` | `null` |

**规则**：
- 列表数据的数组字段名统一用 `list`，禁止使用 `data`、`items`、`records` 等其他名称
- 分页列表必须返回 `total`（总条数）和 `page`（当前页码），可选返回 `page_size`
- 创建操作只返回新资源的 `id`，不返回完整对象
- 更新/删除操作返回 `null`，前端通过 HTTP 200 + `code: 0` 判断成功
- 如果 list 内部可以为空，但是不能有 null 出现

### HTTP 状态码映射规则

| 业务场景 | HTTP 状态码 | code 值 | 说明 |
|----------|------------|---------|------|
| 成功 | 200 | `0` | 请求处理成功 |
| 参数校验失败 | 400 | `400` | 请求体格式错误、必填字段缺失、值不合法 |
| 未认证 | 401 | `401` | Token 缺失、过期、无效 |
| 无权限 | 403 | `403` | 已认证但无权访问该资源 |
| 资源不存在 | 404 | `404` | 查询的对象不存在 |
| 请求频率超限 | 429 | `429` | 触发限流 |
| 业务规则错误 | 422 | `>= 10000` | 业务逻辑不满足，使用自定义错误码（见 `consts.go`） |
| 服务器内部错误 | 500 | `500` | 未预期的异常 |

### 业务错误码定义（>= 10000）

业务错误码定义在 `internal/consts/consts.go` 中，每个错误码有对应的中文消息常量。新增业务错误时必须同时在 `consts.go` 中添加 `Code` 和 `Msg` 常量。

| 错误码 | 常量名 | 默认消息 |
|--------|--------|---------|
| 10001 | `CodeInsufficientBalance` | 余额不足 |
| 10002 | `CodeQuotaExceeded` | 额度已用完 |
| 10003 | `CodeChannelUnavailable` | 没有可用的渠道 |
| ... | 更多见 `consts.go` | ... |

## 大模型代理接口格式（`/v1/*`、`/v1beta/*`、`/suno/*`）

### 已注册的代理端点

| 方法 | 路径 | 功能 | 请求格式 | 流式支持 |
|------|------|------|---------|---------|
| POST | `/v1/chat/completions` | Chat Completions | OpenAI | SSE |
| POST | `/v1/completions` | Text Completions | OpenAI | SSE |
| POST | `/v1/embeddings` | 文本向量 | OpenAI | 否 |
| POST | `/v1/images/generations` | 图像生成 | OpenAI | 否 |
| POST | `/v1/images/edits` | 图像编辑 | OpenAI | 否 |
| POST | `/v1/messages` | Claude Messages | Claude | SSE |
| POST | `/v1/responses` | OpenAI Responses | OpenAI | SSE |
| POST | `/v1/audio/speech` | 语音合成 | OpenAI | 否 |
| POST | `/v1/audio/transcriptions` | 语音转文字 | OpenAI | 否 |
| POST | `/v1/audio/translations` | 语音翻译 | OpenAI | 否 |
| POST | `/v1/rerank` | 重排序 | OpenAI | 否 |
| POST | `/v1/moderations` | 内容审核 | OpenAI | 否 |
| GET | `/v1/models` | 模型列表 | — | 否 |
| GET | `/v1/models/{model_id}` | 模型详情 | — | 否 |
| GET | `/v1/realtime` | 实时对话（WebSocket） | OpenAI | WebSocket |
| POST | `/v1/video/generations` | 视频生成（异步任务） | 自定义 | 否 |
| GET | `/v1/video/generations/{task_id}` | 视频生成任务查询 | — | 否 |
| GET | `/v1beta/models` | Gemini 模型列表 | Gemini | 否 |
| GET | `/v1beta/models/{model}` | Gemini 模型详情 | Gemini | 否 |
| POST | `/v1beta/models/{model}` | Gemini 内容生成 | Gemini | SSE |
| POST | `/suno/submit/{action}` | Suno 音乐生成提交 | 自定义 | 否 |
| POST | `/suno/fetch` | Suno 批量查询 | 自定义 | 否 |
| GET | `/suno/fetch/{task_id}` | Suno 任务查询 | — | 否 |

### OpenAI 格式端点响应

**非流式响应**直接透传上游 JSON，典型结构：

```json
// Chat Completions 响应
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1677652288,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {"role": "assistant", "content": "Hello!"},
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 9,
    "completion_tokens": 12,
    "total_tokens": 21
  }
}
```

```json
// Embeddings 响应
{
  "object": "list",
  "data": [
    {"object": "embedding", "embedding": [0.1, 0.2, ...], "index": 0}
  ],
  "model": "text-embedding-3-small",
  "usage": {"prompt_tokens": 5, "total_tokens": 5}
}
```

**流式响应**使用 SSE（Server-Sent Events），`Content-Type: text/event-stream`：

```
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1677652288,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":12,"total_tokens":21}}

data: [DONE]
```

**OpenAI 格式错误响应**：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "余额不足，请联系管理员充值",
    "param": null,
    "code": null
  }
}
```

**错误类型映射**：

| 平台错误场景 | error.type | HTTP 状态码 |
|-------------|-----------|------------|
| 认证失败（API Key 无效） | `authentication_error` | 401 |
| 权限不足（Key 无权访问模型） | `permission_error` | 403 |
| 余额不足 / 额度耗尽 | `insufficient_quota` | 402 |
| 模型不存在 / 参数错误 | `invalid_request_error` | 400 |
| 请求频率超限 | `rate_limit_error` | 429 |
| 没有可用渠道 | `server_error` | 503 |
| 上游供应商错误 | 原样透传上游错误类型 | 上游状态码 |
| 平台内部错误 | `internal_error` | 500 |

### Claude 格式端点响应（`/messages`）

**非流式响应**：

```json
{
  "id": "msg_abc123",
  "type": "message",
  "role": "assistant",
  "content": [{"type": "text", "text": "Hello!"}],
  "model": "claude-sonnet-4-20250514",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 25,
    "output_tokens": 10
  }
}
```

**流式响应（SSE）**：

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_abc123","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4-20250514","usage":{"input_tokens":25,"output_tokens":0}}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello!"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}

event: message_stop
data: {"type":"message_stop"}
```

**Claude 格式错误响应**：

```json
{
  "type": "error",
  "error": {
    "type": "authentication_error",
    "message": "余额不足，请联系管理员充值"
  }
}
```

### Gemini 格式端点响应（`/v1beta/models/*`）

使用 Google Gemini API 原生格式透传，错误格式：

```json
{
  "error": {
    "code": 400,
    "message": "请求参数错误",
    "status": "INVALID_ARGUMENT"
  }
}
```

### 错误处理实现规范

- `WriteRelayError(w, err)` — 写入 OpenAI 格式错误，用于 `/v1/chat/completions`、`/v1/embeddings` 等 OpenAI 格式端点
- `WriteClaudeRelayError(w, err)` — 写入 Claude 格式错误，用于 `/v1/messages` 端点
- 平台级错误（余额不足、渠道不可用、频率限制）转换为供应商原生格式的错误类型，消息使用中文
- 上游供应商错误原样透传，不做二次包装
- 限流错误额外设置 `X-RateLimit-Limit`、`X-RateLimit-Remaining`、`X-RateLimit-Reset` 响应头

## 中间件配置差异

| 中间件 | 管理接口 | 代理接口 |
|--------|---------|---------|
| AdminAuth / TenantAuth（JWT） | 使用 | 不使用 |
| ApiKeyAuth（Bearer Token） | 不使用 | 使用 |
| MiddlewareHandlerResponse（统一响应） | 使用 | 不使用 |
| ErrorHandler | 使用（支付回调） | 不使用 |
| MaintenanceMode（维护模式） | 使用（租户） | 使用 |
| ContentFilter（内容过滤） | 不使用 | 使用 |
| OperationLog（操作日志） | 使用（管理后台） | 不使用 |
| OpenPlatformAuth（HMAC-SHA256） | 仅开放平台 | 不使用 |
| RBAC（权限校验） | 使用 | 不使用 |
| Idempotency（幂等性） | 按需使用 | 不使用 |
| RequestID 注入 | 使用 | 使用 |
