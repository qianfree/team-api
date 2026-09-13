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

## 大模型代理接口格式（`/v1/*`、`/v1beta/*`、`/v2/*`、`/suno/*`）

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
| POST | `/v1/videos` | OpenAI Videos 视频生成（multipart/JSON） | OpenAI | 否 |
| GET | `/v1/videos/{video_id}` | OpenAI Videos 任务查询 | — | 否 |
| GET | `/v1/videos/{video_id}/content` | OpenAI Videos 成品下载 | — | 否 |
| DELETE | `/v1/videos/{video_id}` | OpenAI Videos 任务删除（终态软删） | — | 否 |
| GET | `/v1beta/models` | Gemini 模型列表 | Gemini | 否 |
| GET | `/v1beta/models/{model}` | Gemini 模型详情 | Gemini | 否 |
| POST | `/v1beta/models/{model}` | Gemini 内容生成 | Gemini | SSE |
| POST | `/suno/submit/{action}` | Suno 音乐生成提交 | 自定义 | 否 |
| POST | `/suno/fetch` | Suno 批量查询 | 自定义 | 否 |
| GET | `/suno/fetch/{task_id}` | Suno 任务查询 | — | 否 |
| POST | `/v2/video_generation` | MiniMax H3 视频生成（官方协议 v2） | MiniMax | 否 |
| GET | `/v2/query/video_generation/{task_id}` | MiniMax H3 任务查询（v2） | — | 否 |
| DELETE | `/v2/video_generation/{task_id}` | MiniMax H3 任务取消（仅排队中，v2） | — | 否 |
| POST | `/v1/video_generation` | MiniMax Hailuo 视频生成（官方协议 v1，四形态共用） | MiniMax | 否 |
| GET | `/v1/query/video_generation?task_id=` | MiniMax Hailuo 任务查询（v1，query 参数） | — | 否 |

### MiniMax 官方视频协议（`/v2/*`，MiniMax-H3/H3-Max）

对 MiniMax v2 视频协议的原生接入：官方 SDK 把 base_url 指向网关即可直连。协议规格见 `docs/modeldocs/minimax/video/`。

**提交** `POST /v2/video_generation`（Bearer 平台 API Key）：

```json
{
  "model": "MiniMax-H3",
  "content": [{ "type": "text", "text": "海浪拍打礁石，慢镜头。" }],
  "resolution": "768P",
  "duration": 6,
  "ratio": "16:9"
}
```

- `model` / `content`（含非空 text 项）/ `resolution` / `duration` 四项必填；`ratio` 文生视频时必填（不能为 `adaptive`）
- `callback_url` **会被剥离**：网关按自身任务体系轮询计费，上游回调携带的上游 task_id 与网关公开 ID 不互通
- 提交响应为官方形态：`{"task_id": "task_xxx"}`（网关公开任务 ID，非上游 ID）

**查询** `GET /v2/query/video_generation/{task_id}` → `{"task": {官方 VideoTask}}`：
状态机 `queued/running/succeeded/failed`（上游取消的任务回放为 `failed`）。`content.url` 为上游限时直链；
`usage/resolution/duration/ratio` 从最近一次上游查询响应回放，任务提交后首拍轮询前（≤15s）可能缺失。

**取消** `DELETE /v2/video_generation/{task_id}`：仅支持取消**排队中**（queued）的任务，取消结果以远程返回为准——
网关调用上游 DELETE，上游确认 `action=cancelled` 才向客户端返回成功
（`{"task_id": "...", "action": "cancelled", "status": "cancelled"}`）；上游拒绝时（如任务已进入 running）
原样透传上游错误。**不支持删除任务记录**（终态任务不可删，软删也不做，任务数据保留用于计费与审计；错误码 `task_not_cancellable`）。
取消确认后本地状态由轮询循环在下一拍（≤15s）对账收敛为 failed，预扣随之退款。

错误响应为 OpenAI 风格 OaiError（`{"error": {"type": "...", "message": "..."}}`），与官方 v2 一致。

**OpenAI 协议互通**：同一渠道下 `POST /v1/videos`（OpenAI Videos 协议）与 `POST /v1/video/generations`（通用任务体）可直接调用 MiniMax 视频模型（H3 与 Hailuo 系列按模型自动分派 v2/v1 上游协议）。`size` 只借 OpenAI 协议的字段形态、**值原样透传不换算**——请直接传目标供应商的原生分辨率词汇（H3：`768P`/`2K`/`480P`；Hailuo 2.x：`768P`/`1080P`）；非官方档位值会原样送达上游（上游 400 则任务提交失败、预扣全额退款）。时长取 `seconds`/`duration`/`metadata.duration`（缺省 6 秒），`input_reference`/`images[0]` 作为首帧图（v1 转为 `first_frame_image`），纯文生视频时 v2 ratio 默认 `16:9`。

### MiniMax 官方视频协议 v1（`/v1/video_generation`，Hailuo 系列）

Hailuo 2.x 及旧型号（T2V/I2V/S2V-01，传入即转发不拦截）走 v1 扁平字段协议：

- **提交** `POST /v1/video_generation`：文生（`prompt` 必填）/ 图生（`first_frame_image` 必填，支持 URL 或 data URI）/ 首尾帧（`last_frame_image`，仅 Hailuo-02）/ 主体参考（`subject_reference`，S2V-01）四形态共用；`duration`（默认 6，768P 支持 10）/`resolution`（Hailuo 2.x 默认 `768P`，旧型号默认 `720P`）可选；`prompt_optimizer`/`fast_pretreatment`/`aigc_watermark` 透传；`callback_url` 剥离。响应为官方 `{task_id, base_resp}` 形态（task_id 为网关公开 ID）
- **查询** `GET /v1/query/video_generation?task_id=`：官方 `{task_id, status, file_id, video_width, video_height, base_resp}` 形态，状态机 `Preparing/Queueing/Processing/Success/Fail`；**增补字段** `download_url`（网关轮询时经官方 files/retrieve 二跳解析的限时直链，官方查询响应无此字段）与 `error`（失败原因）
- **成品二跳由网关代取**：官方流程「file_id → GET /v1/files/retrieve → download_url」在轮询时链式完成，`download_url` 落任务记录并回放到查询响应；不暴露 files/retrieve 代理端点
- **取消/删除**：v1 上游无此能力，端点不提供（v2 的 DELETE 端点对 v1 任务返回 400）
- v1 无 usage 返回，计费按请求规格（per_second 矩阵 × 时长），与 v2 口径一致

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
