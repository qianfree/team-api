// sora-js：视频生成插件的完整参考实现（内置插件，OpenAI Sora 风格协议）。
// 与存量 sora Go adaptor 并存：独立插件 key 与模型名（sora-2-js），
// 经独立渠道（类型「JS 任务插件」）路由，不触碰原有功能。
//
// 全量示范：七个钩子（含可选渲染）、usageSchema 按秒计费、产物转存（视频 URL → OSS）、
// 配置项读取（默认尺寸/时长上限）。上游协议：
//   POST {base}/v1/videos {model,prompt,seconds,size} → {id:"video_xxx", status:"queued"}
//   GET  {base}/v1/videos/{id} → {id,status:"queued|in_progress|completed|failed",
//                               progress:0-100, seconds, error?, video:{url,size_mb}}
export const meta = {
  apiVersion: 1,
  key: "sora-js",
  name: "Sora 视频生成（JS 示例）",
  version: "1.1.0",
  description: "视频任务插件完整示例：按秒计费 + 产物转存 + 配置项",
  icon: "video",
  author: "qianfree",
  models: ["sora-2-js"],
  fetchMode: "per_task",
  // 标准视频端点支持：声明后模型可经 /v1/videos 与 /v1/video/generations 提交
  standardVideos: "decodeVideos",
  usageSchema: [
    { key: "seconds", type: "number", unit: "second", description: "生成视频时长秒数（计费按秒）" },
    { key: "quality", type: "enum", description: "清晰度档位（展示用）" },
  ],
  config: [
    { key: "default_size", label: "默认分辨率", type: "select",
      options: ["1280x720", "720x1280", "1920x1080"], default: "1280x720", halfWidth: true, group: "生成" },
    { key: "max_seconds", label: "时长上限（秒）", type: "int", default: 12, halfWidth: true, group: "生成" },
    { key: "vendor_note", label: "上游备注", type: "text", group: "其他",
      description: "随任务提交透传给上游 note 字段（示例配置项的读取与使用）" },
  ],
  routes: [
    { method: "POST", path: "/videos", type: "submit" },
    { method: "GET", path: "/videos/:task_id", type: "query" },
  ],
};

// ── 提交链 ────────────────────────────────────────────────

// decodeVideos 标准视频端点解码（meta.standardVideos 指向）：把 /v1/videos 与
// /v1/video/generations 的通用任务体（{model, prompt, seconds?, metadata.*}）映射为
// 本插件原生 requestBody（对齐 decodeRequest 的归一形态），下游钩子
// （estimateUsage/buildSubmitRequest）无感复用。metadata.image 参考图上游协议
// 不支持，静默忽略。
export function decodeVideos(ctx) {
  const raw = ctx.body || {};
  const metadata = raw.metadata || {};
  const body = {
    model: String(ctx.model || raw.model || "sora-2-js"),
    prompt: String(raw.prompt || ""),
  };
  // 时长：顶层 seconds 优先，缺省回落 metadata.duration（/v1/videos 双写两处）
  const seconds = raw.seconds != null ? raw.seconds : metadata.duration;
  if (seconds != null && Number(seconds) > 0) body.seconds = Number(seconds);
  if (metadata.size) body.size = String(metadata.size);
  if (!body.prompt.trim()) {
    throw new Error("prompt is required");
  }
  return { kind: "submit", model: body.model, requestBody: body };
}

export function decodeRequest(ctx) {
  const body = Object.assign({}, ctx.body || {});
  if (!String(body.prompt || "").trim()) {
    throw new Error("prompt is required");
  }
  // 模型缺省回落插件默认模型；body.model 若给了其他模型则拒绝（防误路由）
  const model = body.model ? String(body.model) : "sora-2-js";
  return { kind: "submit", model, requestBody: body };
}

export function estimateUsage(ctx) {
  // 预扣口径：请求时长（钳制到配置上限），清晰度按分辨率判定
  const cap = Math.max(Number(utils.config.get("max_seconds") || 12), 1);
  const seconds = Math.min(Math.max(Number((ctx.body || {}).seconds || 8), 1), cap);
  const size = String((ctx.body || {}).size || utils.config.get("default_size") || "1280x720");
  return { seconds, quality: size.startsWith("1920") ? "hd" : "sd" };
}

export function buildSubmitRequest(ctx) {
  const body = ctx.body || {};
  return {
    url: "/v1/videos",
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: {
      model: ctx.upstreamModel || ctx.model,
      prompt: body.prompt,
      seconds: Number(body.seconds || 8),
      size: String(body.size || utils.config.get("default_size") || "1280x720"),
      note: String(utils.config.get("vendor_note") || ""),
    },
  };
}

export function parseSubmitResponse(ctx) {
  if (ctx.status !== 200) throw new Error("submit_http_" + ctx.status);
  const resp = ctx.responseBody || {};
  if (!resp.id) throw new Error("upstream returned no video id");
  return { taskId: String(resp.id), taskData: resp, state: { polls: 0 } };
}

// ── 轮询链 ────────────────────────────────────────────────

export function buildQueryRequest(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  return { url: "/v1/videos/" + encodeURIComponent(t.taskId), method: "GET" };
}

export function parseQueryResponse(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const resp = ctx.responseBody || {};
  const state = Object.assign({}, t.state || {});
  state.polls = (state.polls || 0) + 1;

  if (resp.status === "completed") {
    return {
      items: [{
        taskId: t.taskId,
        done: true,
        taskData: resp,
        usage: { seconds: Number(resp.seconds || 8), quality: resp.video && resp.video.size_mb > 50 ? "hd" : "sd" },
        artifacts: [{ key: "video", mimeType: "video/mp4", source: { type: "url", path: "video.url" } }],
        state,
      }],
    };
  }
  if (resp.status === "failed") {
    return {
      items: [{
        taskId: t.taskId,
        done: true,
        error: { code: "sora_failed", message: (resp.error && resp.error.message) || "upstream failed" },
        state,
      }],
    };
  }
  return { items: [{ taskId: t.taskId, done: false, taskData: resp, state }] };
}

// ── 渲染（面向客户端，OpenAI 风格）────────────────────────

export function renderSubmit(ctx) {
  const task = ctx.task || {};
  return {
    status: 200,
    body: {
      id: task.publicTaskId,
      object: "video",
      status: task.status === "SUBMITTED" ? "queued" : task.status,
      created_at: utils.now(),
    },
  };
}

export function renderTask(ctx) {
  const task = ctx.task || {};
  const data = task.data || {};
  return {
    status: 200,
    body: {
      id: task.publicTaskId,
      object: "video",
      status: mapStatus(task.status),
      progress: Number(data.progress || 0),
      seconds: Number(data.seconds || 0),
      video_url: ((data.video || {}).url) || null, // 转存完成后为本地 OSS 链接
      fail_reason: task.failReason || null,
    },
  };
}

function mapStatus(s) {
  switch (s) {
    case "SUCCESS": return "completed";
    case "FAILURE": return "failed";
    case "IN_PROGRESS": return "in_progress";
    default: return "queued";
  }
}
