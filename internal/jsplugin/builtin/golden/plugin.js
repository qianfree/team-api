// golden 插件：契约 v1 的参考实现，同时是引擎单测与 E2E 的夹具。
// 作者模板：抄这个文件起步，删掉不需要的钩子。
// 双能力示范：golden_music（音乐，仅原生路由入口）+ golden_video（视频，原生路由
// 与标准端点双入口——视频插件必须声明 meta.standardVideos 并导出对应解码钩子，
// 否则模型无法经 /v1/videos 与 /v1/video/generations 提交，调用方只能走 /pjs 原生路由）。
//
// 上游协议（mock vendor）：
//   POST {base}/v1/golden/submit          → {code:"success", data:"<task_id>"}
//   GET  {base}/v1/golden/fetch?task_id=  → 音乐 {status, data:{audio_url}, clips, tier, error?}
//                                           视频 {status, data:{video_url}, seconds, size, error?}
//
// 契约要点（1.4.0）：
//   - meta.models = 插件支持的【上游模型名】。平台模型名由渠道模型路由/映射决定，
//     同一上游模型可被多个插件声明（跨插件可重名），请求归哪个插件由渠道绑定决定。
//   - 用量只报宿主固定词汇（output_seconds/resolution/output_count/input_image_count/
//     input_video_seconds/input_audio_seconds/input_tokens/output_tokens/generate_audio），
//     怎么计价由宿主模型定价决定——插件对价格一无所知。
//   - 按上游型号的参数校验放 validateRequest（选定渠道后调用，带 ctx.upstreamModel）。
export const meta = {
  apiVersion: 1,
  key: "golden",
  name: "Golden 示例插件",
  version: "1.4.0",
  description: "契约 v1 参考实现：音乐/视频双模型、原生路由 + 标准视频端点双入口、上游模型校验、宿主用量词汇、产物声明、配置项",
  icon: "text",
  author: "qianfree",
  models: ["golden_music", "golden_video"],
  fetchMode: "per_task",
  // 视频模型的标准端点支持：声明后 golden_video 可经 POST /v1/videos（OpenAI Videos
  // 协议）与 POST /v1/video/generations（通用任务端点）提交；查询侧 GET 同路径 /{id}
  // 通用。未声明的插件这两个端点对插件模型直接 400
  standardVideos: "decodeVideos",
  // 标准端点可承接的参考素材角色（references[].role）：网关在调用解码钩子前拦截
  // 未声明角色；缺省仅 first_frame。视频插件按上游能力声明
  referenceRoles: ["first_frame", "last_frame"],
  config: [
    { key: "fallback_url", label: "备用上游地址", type: "string", group: "上游", description: "主渠道失败时兜底" },
    { key: "api_key", label: "厂商密钥", type: "string", encrypted: true, group: "凭据" },
    { key: "max_clips", label: "单次上限", type: "int", default: 4, halfWidth: true },
    { key: "tier_mode", label: "档位", type: "select", options: ["standard", "pro"], default: "standard", halfWidth: true },
  ],
  routes: [
    { method: "POST", path: "/submit/:action", type: "submit" },
    { method: "GET", path: "/fetch/:task_id", type: "query" },
  ],
};

// ── 提交链 ────────────────────────────────────────────────

// 原生路由解码：action 决定模型（MUSIC/LYRICS → 音乐，VIDEO → 视频）
export function decodeRequest(ctx) {
  const action = String(((ctx.params || {}).action || "")).toUpperCase();
  if (action === "VIDEO") {
    const body = Object.assign({}, ctx.body || {});
    if (!String(body.prompt || "").trim()) throw new Error("field prompt is required");
    body.action = "VIDEO";
    return { kind: "submit", model: "golden_video", requestBody: body };
  }
  if (action !== "MUSIC" && action !== "LYRICS") {
    throw new Error("invalid_action: " + action);
  }
  const body = Object.assign({}, ctx.body || {});
  body.action = action;
  return { kind: "submit", model: "golden_music", requestBody: body };
}

// 标准视频端点解码（meta.standardVideos 指向）：/v1/videos 与 /v1/video/generations
// 的请求体入钩子前均已被网关归一为通用任务体 {model, prompt, seconds?, references?,
// images?, metadata:{duration, size, resolution, aspect_ratio, sound, image, last_frame,
// ...}, extra?}——本钩子把它映射为插件原生 requestBody，下游 estimateUsage /
// buildSubmitRequest / 轮询链与原生路由入口完全复用。返回与 decodeRequest 同形态。
//
// 模型名分工（重要）：ctx.model 是客户端请求的【平台模型名】，原样回显即可，不要做
// 白名单校验——meta.models 是上游模型清单，平台名由渠道映射决定、可以是任意别名。
// 上游名在渠道选定后出现（validateRequest / estimateUsage / buildSubmitRequest 的
// ctx.upstreamModel）；按上游型号的校验放 validateRequest。
export function decodeVideos(ctx) {
  const raw = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const metadata = raw.metadata && typeof raw.metadata === "object" && !Array.isArray(raw.metadata) ? raw.metadata : {};
  const model = String(ctx.model || raw.model || "").trim() || "golden_video";
  // 多形态插件用排除法挡住已知的非视频模型（白名单会误拒平台别名）
  if (model === "golden_music") throw new Error("model golden_music does not support video endpoints");
  if (!String(raw.prompt || "").trim()) throw new Error("field prompt is required");

  const body = { action: "VIDEO", model: model, prompt: String(raw.prompt) };
  // 时长：顶层 seconds（字符串/数字）或 metadata.duration（归一层双写，读一处即可）
  let seconds = raw.seconds != null ? raw.seconds : metadata.duration;
  if (typeof seconds === "string") seconds = Number(seconds);
  body.seconds = Number.isFinite(seconds) && seconds > 0 ? seconds : 4;
  // 分辨率：metadata.size（OpenAI 词汇）/ metadata.resolution（通用端点词汇）均承接
  const size = String(metadata.size || metadata.resolution || "").trim();
  if (size) body.size = size;

  // 参考素材：references[] 是唯一权威来源（存在时不再读兼容字段）
  const refs = Array.isArray(raw.references) ? raw.references : [];
  let firstFrame = "";
  let lastFrame = "";
  for (const ref of refs) {
    if (!ref || typeof ref !== "object") continue;
    const url = String(ref.url || "").trim();
    if (!url) continue;
    if (ref.role === "first_frame") firstFrame = url;
    else if (ref.role === "last_frame") lastFrame = url;
    else throw new Error("reference role not supported: " + ref.role);
  }
  // 兼容字段兜底（references 缺席时的旧客户端形态）：首帧 metadata.image / images[0]
  if (!firstFrame) {
    const image = Array.isArray(raw.images) && typeof raw.images[0] === "string" ? raw.images[0] : "";
    firstFrame = String(metadata.image || image || "").trim();
  }
  if (!lastFrame) lastFrame = String(metadata.last_frame || "").trim();
  if (firstFrame) body.first_frame = firstFrame;
  if (lastFrame) body.last_frame = lastFrame;

  return { kind: "submit", model: model, requestBody: body };
}

// validateRequest（可选）：渠道选定后、预扣前调用，ctx.upstreamModel 是渠道映射后的
// 上游模型名（宿主已校验它 ∈ meta.models）。按上游型号的参数约束在此校验——decode
// 阶段只知道平台模型名。抛错 = 400（未预扣）；返回 {requestBody} 可修正归一请求体。
export function validateRequest(ctx) {
  const body = ctx.body || {};
  const upstream = String(ctx.upstreamModel || ctx.model || "");
  if (body.action === "VIDEO" && upstream === "golden_music") {
    throw new Error("upstream model golden_music does not support video generation");
  }
  if (body.action !== "VIDEO" && upstream === "golden_video") {
    throw new Error("upstream model golden_video does not support music generation");
  }
  return {};
}

// estimateUsage 预扣用量估算：只报宿主固定词汇，计价口径由宿主模型定价决定。
export function estimateUsage(ctx) {
  const body = ctx.body || {};
  // 形态判定读归一后的 body（decode 阶段写入的 action），不依赖模型名——模型可能是平台别名
  if (body.action === "VIDEO") {
    let seconds = Number(body.seconds || 4);
    if (!Number.isFinite(seconds) || seconds <= 0) seconds = 4;
    return { output_seconds: Math.min(seconds, 60) };
  }
  // 音乐按段：output_count = 请求段数（缺省 1），钳制到配置的单次上限
  const cap = Number(utils.config.get("max_clips") || 4);
  return { output_count: Math.min(Math.max(Number(body.clips || 1), 1), cap) };
}

export function buildSubmitRequest(ctx) {
  const body = ctx.body || {};
  // 上游模型名：ctx.upstreamModel 是渠道模型映射后的上游名（平台名 ≠ 上游名时必须用它）
  const upstream = {
    prompt: body.prompt || "",
    action: body.action,
    model: ctx.upstreamModel || ctx.model,
  };
  if (body.action === "VIDEO") {
    upstream.seconds = Number(body.seconds || 4);
    if (body.size) upstream.size = body.size;
    if (body.first_frame) upstream.first_frame = body.first_frame;
    if (body.last_frame) upstream.last_frame = body.last_frame;
  } else {
    upstream.clips = body.clips || 1;
  }
  // 鉴权：不声明 Authorization 时宿主自动注入 "Bearer <渠道密钥>"（提交与轮询同理），
  // 无需为密钥另设配置项；签名类厂商才需要用 ctx.apiKey 自行计算并声明该头覆盖默认值
  return {
    url: "/v1/golden/submit",
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: upstream,
  };
}

// 宿主只在上游返回 HTTP 200 时调用本钩子（非 200 已由宿主按上游错误处理），
// 无需判断 ctx.status；业务层的拒绝（200 + 错误码）在此抛错。
export function parseSubmitResponse(ctx) {
  const resp = ctx.responseBody || {};
  if (resp.code !== "success" || !resp.data) throw new Error("submit_rejected");
  // state 跨轮存活（宿主持久化，轮询时经 tasks[].state 回带）：kind 标记提交形态
  // （视频/音乐），轮询链 usage/artifacts 按形态分流。taskData 每轮被整体覆盖，放不住
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const kind = body.action === "VIDEO" ? "video" : "music";
  return { taskId: String(resp.data), taskData: resp, state: { polls: 0, kind: kind } };
}

// ── 轮询链 ────────────────────────────────────────────────

export function buildQueryRequest(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  return {
    url: "/v1/golden/fetch?task_id=" + encodeURIComponent(t.taskId),
    method: "GET",
  };
}

// ctx.status 恒为 200（宿主不透传轮询的 HTTP 状态码），终态一律按响应体内容判定；
// 无法识别的响应按进行中返回，由宿主的任务超时兜底。
export function parseQueryResponse(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const resp = ctx.responseBody || {};
  const state = Object.assign({}, t.state || {});
  state.polls = (state.polls || 0) + 1;
  const isVideo = state.kind === "video";

  if (resp.status === "success") {
    return {
      items: [{
        taskId: t.taskId,
        done: true,
        taskData: resp,
        usage: isVideo
          ? { output_seconds: Number(resp.seconds || 4) }
          : { output_count: Number(resp.clips || 1) },
        artifacts: isVideo
          ? [{ key: "video", source: { type: "url", path: "data.video_url" } }]
          : [{ key: "audio", source: { type: "url", path: "data.audio_url" } }],
        state,
      }],
    };
  }
  if (resp.status === "failed") {
    return {
      items: [{
        taskId: t.taskId,
        done: true,
        error: { code: "golden_failed", message: resp.error || "upstream failed" },
        state,
      }],
    };
  }
  // 进行中：上游带进度（number 0~100 或 "42%"）时上报，宿主归一化落库并透出给查询端点
  const item = { taskId: t.taskId, done: false, taskData: resp, state };
  const progress = Number(resp.progress);
  if (Number.isFinite(progress) && progress >= 0) item.progress = Math.min(Math.round(progress), 100);
  return { items: [item] };
}

// ── 渲染（面向客户端）─────────────────────────────────────
// 入参 ctx.task 字段（仅这些，没有 error 字段）：
//   publicTaskId 平台任务 ID；taskId 上游任务 ID；model 平台模型名；
//   status 平台状态枚举 SUBMITTED / IN_PROGRESS / SUCCESS / FAILURE（不是上游状态词，
//     要对外呈现厂商/OpenAI 状态词时在此自行映射）；
//   progress 百分比字符串（如 "50%"，可能为空）；failReason 失败原因（仅失败时非空）；
//   data 最新上游快照（成功任务的产物路径值已替换为平台内容链接）；contentUrl 主产物链接。
// renderSubmit 只收到 publicTaskId / model / status / contentUrl。

export function renderSubmit(ctx) {
  const task = ctx.task || {};
  return { status: 200, body: { code: "success", task_id: task.publicTaskId, status: task.status } };
}

export function renderTask(ctx) {
  const task = ctx.task || {};
  const data = (task.data || {}).data || {};
  const body = {
    task_id: task.publicTaskId,
    status: task.status,
    progress: task.progress || "",
    audio_url: data.audio_url || null,
  };
  if (task.model === "golden_video" || data.video_url !== undefined) body.video_url = data.video_url || null;
  if (task.failReason) body.fail_reason = task.failReason;
  return { status: 200, body };
}
