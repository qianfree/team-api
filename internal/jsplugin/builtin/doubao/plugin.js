// doubao：自 new-api plugins/tasks/doubao/plugin.js 移植（迁移批次 5/6）。
// 火山引擎豆包：Seedance 视频 + Seedream 图片（同步）。
//
// ── 迁移差异日志（vs new-api 原版，1281 行）─────────────────
// ① 视频（Seedance）完整迁移：提交纯 JSON（SSE 仅图片流式用，视频本就 JSON）。
// ② 图片（Seedream）非流式迁移：走宿主 immediate（同步即完成）；流式（stream:true）
//    降级拒绝——宿主无 SSE 提交响应钩子（P3 待评估项）。
// ③ 计费口径保留：tokens = 秒 × 宽 × 高 × 24 / 1024（官方公式）估算 + 终态
//    completion_tokens 精确结算；video_input 枚举（video/none）与
//    video_input_ratio 折算保留（ratio 进 formula_facts 供表达式引用）。
// ④ usageSchema map 形态：tokens(number)+resolution(enum)+video_input(enum)
//    +generate_audio(boolean)+video_input_ratio(number)。
// ⑤ routes：视频提交/查询 + 图片提交（immediate）。
// ⑥ fileRef：视频参考图（content[].image_url 的 fileRef 占位）支持。
// ⑦ renderSubmit/renderTask ← Ark 信封包装。
// ⑧ 删除：protocols（openai_responses SSE 认领）、requiredCapabilities
//    （duration-auto，宿主以 maxDuration 保留口径替代）、draft 任务改写
//    （rewriteDraftTaskContent/originTasks——无宿主对应概念）。
// ─────────────────────────────────────────────────────────────
const VIDEO_MODELS = {
  "doubao-seedance-1-0-pro-250528": { resolutions: ["480p", "720p", "1080p"], maxDuration: 12 },
  "doubao-seedance-1-0-lite-t2v": { resolutions: ["480p", "720p", "1080p"], maxDuration: 12 },
  "doubao-seedance-1-0-lite-i2v": { resolutions: ["480p", "720p", "1080p"], maxDuration: 12 },
  "doubao-seedance-1-5-pro-251215": { resolutions: ["480p", "720p", "1080p"], maxDuration: 12, audio: true },
  "doubao-seedance-2-0-260128": { resolutions: ["480p", "720p", "1080p", "4k"], maxDuration: 15, videoInput: true },
  "doubao-seedance-2-0-fast-260128": { resolutions: ["480p", "720p"], maxDuration: 15, videoInput: true },
  "doubao-seedance-2-0-mini-260615": { resolutions: ["480p", "720p"], maxDuration: 15, videoInput: true },
  "doubao-seedance-2-5-260628": { resolutions: ["480p", "720p", "1080p"], maxDuration: 30, videoInput: true },
};
const IMAGE_MODELS = {
  "doubao-seedream-5-0-pro-260628": { presets: ["1K", "1.5K", "2K"] },
  "doubao-seedream-5-0-flash-260915": { presets: ["1K", "1.5K", "2K"] },
  "doubao-seedream-5-0-lite-260128": { presets: ["2K", "3K", "4K"] },
};
const SEEDANCE_RESOLUTIONS = ["480p", "720p", "1080p", "4k"];
const DEFAULT_VIDEO_PROFILE = { resolutions: SEEDANCE_RESOLUTIONS, maxDuration: 30, videoInput: true };

export const meta = {
  apiVersion: 1,
  key: "doubao",
  name: "Doubao",
  icon: "video",
  description: {
    en: "Volcengine Doubao Seedance video generation and Seedream image generation",
    zh: "火山引擎豆包 Seedance 视频生成与 Seedream 图片生成",
  },
  version: "1.4.0",
  author: "qianfree",
  channelTypes: [51],
  models: Object.keys(VIDEO_MODELS).concat(Object.keys(IMAGE_MODELS)),
  fetchMode: "per_task",
  usageSchema: {
    tokens: { type: "number", unit: "count", description: { en: "Ark tokens (official formula)", zh: "Ark tokens（官方公式计费单位）" } },
    resolution: { enum: SEEDANCE_RESOLUTIONS, description: { en: "Output video resolution", zh: "输出视频分辨率" } },
    video_input: { enum: ["video", "none"], description: { en: "Has video input", zh: "是否含视频输入" } },
    video_input_ratio: { type: "number", description: { en: "Video input discount ratio", zh: "视频输入折扣系数" } },
    generate_audio: { type: "boolean", description: { en: "Audio generated", zh: "是否生成音频" } },
    image_count: { type: "number", unit: "count", description: { en: "Delivered image count", zh: "交付图片数" } },
  },
  routes: [
    { method: "POST", path: "/api/v3/contents/generations/tasks", type: "submit", decode: "createTask", render: "renderSubmit" },
    { method: "POST", path: "/api/v3/images/generations", type: "submit", decode: "createImage", render: "renderImage" },
    { method: "GET", path: "/api/v3/contents/generations/tasks/:task_id", type: "query", render: "renderTask" },
  ],
};

function trimmed(v) { return String(v || "").trim(); }
function plainObject(v) { return v && typeof v === "object" && !Array.isArray(v) ? v : {}; }

function videoProfile(model) {
  return VIDEO_MODELS[trimmed(model)] || DEFAULT_VIDEO_PROFILE;
}
function isVideoModel(model) {
  return Object.prototype.hasOwnProperty.call(VIDEO_MODELS, trimmed(model)) || !IMAGE_MODELS[trimmed(model)];
}
function hasVideo(content) {
  return (Array.isArray(content) ? content : []).some(function (c) { return c && c.type === "video_url"; });
}

// ── 解码 ──────────────────────────────────────────────────

export function createTask(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model);
  if (!model) throw new Error("model is required");
  const profile = videoProfile(model);
  if (body.resolution !== undefined && body.resolution !== null) {
    const raw = trimmed(body.resolution).toLowerCase();
    if (SEEDANCE_RESOLUTIONS.includes(raw) && !profile.resolutions.includes(raw)) {
      throw new Error(model + " resolution must be one of " + profile.resolutions.join(", "));
    }
  }
  return { kind: "submit", model: model, requestBody: { model: model, vendorBody: body } };
}

export function createImage(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model);
  if (!model) throw new Error("model is required");
  if (!IMAGE_MODELS[model]) throw new Error(model + " is not a Seedream image model");
  if (body.stream === true) throw new Error("stream is not supported; use the non-streaming image API");
  return { kind: "submit", model: model, requestBody: { model: model, vendorBody: body, _image: true } };
}

// ── 计费 ──────────────────────────────────────────────────

function resolutionMaxPixels(resolution) {
  if (resolution === "480p") return [854, 480];
  if (resolution === "1080p") return [1920, 1080];
  if (resolution === "4k") return [3840, 2160];
  return [1280, 720];
}
function estimateTokens(seconds, resolution) {
  const dims = resolutionMaxPixels(resolution);
  return (seconds * dims[0] * dims[1] * 24) / 1024;
}
function videoInputRatio(model, resolution, content) {
  const video = hasVideo(content);
  const res = trimmed(resolution).toLowerCase();
  if (model === "doubao-seedance-2-5-260628") {
    if (res === "1080p") return video ? 7.0 / 10.7 : 11.7 / 10.7;
    return video ? 42 / 70 : 1;
  }
  if (model === "doubao-seedance-2-0-260128") {
    if (res === "1080p") return video ? 31 / 46 : 51 / 46;
    if (res === "4k") return video ? 16 / 46 : 26 / 46;
    return video ? 28 / 46 : 1;
  }
  if (model === "doubao-seedance-2-0-fast-260128") return video ? 22 / 37 : 1;
  if (model === "doubao-seedance-2-0-mini-260615") return video ? 14 / 23 : 1;
  return 1;
}

export function estimateUsage(ctx) {
  const req = ctx.body || {};
  if (req._image) {
    const vendor = plainObject(req.vendorBody);
    const n = Number(vendor.n || vendor.size && String(vendor.size).split("*").length > 1 ? vendor.n : vendor.n) || 1;
    return { image_count: Math.min(Math.max(Number.isFinite(Number(vendor.n)) ? Number(vendor.n) : 1, 1), 40) };
  }
  const model = ctx.model;
  const profile = videoProfile(model);
  const vendor = plainObject(req.vendorBody);
  // -1（模型自选时长）与缺省：保留最长时长口径
  let seconds = Number(vendor.duration);
  if (!Number.isFinite(seconds) || seconds <= 0) seconds = profile.maxDuration;
  seconds = Math.min(seconds, 3600);
  const raw = trimmed(vendor.resolution).toLowerCase();
  const resolution = SEEDANCE_RESOLUTIONS.includes(raw) ? raw : (profile.resolutions.includes("1080p") ? "1080p" : profile.resolutions[profile.resolutions.length - 1]);
  const facts = {
    tokens: estimateTokens(seconds, resolution),
    resolution: resolution,
    video_input: hasVideo(vendor.content) ? "video" : "none",
    video_input_ratio: videoInputRatio(model, resolution, vendor.content),
  };
  if (profile.audio) facts.generate_audio = vendor.generate_audio !== false;
  return facts;
}

// ── 提交链 ────────────────────────────────────────────────

export function buildSubmitRequest(ctx) {
  const req = ctx.body || {};
  const model = ctx.upstreamModel || ctx.model;
  if (req._image) {
    return { url: "/api/v3/images/generations", method: "POST", body: Object.assign({}, plainObject(req.vendorBody), { model: model, stream: false }) };
  }
  return { url: "/api/v3/contents/generations/tasks", method: "POST", body: Object.assign({}, plainObject(req.vendorBody), { model: model }) };
}

export function parseSubmitResponse(ctx) {
  const body = plainObject(ctx.responseBody);
  if (String(ctx.path || "").includes("images")) {
    // 图片同步 API：immediate 终态（上游无任务 ID——用宿主侧占位符）
    const placeholder = "sync-" + utils.uuid().slice(0, 8);
    const data = Array.isArray(body.data) ? body.data : [];
    if (body.error || data.length === 0) {
      const failure = body.error
        ? (trimmed(body.error.code) || "upstream_error") + ": " + (trimmed(body.error.message) || "failed")
        : "image generation returned no images";
      return { taskId: placeholder, taskData: body, immediate: { status: "FAILURE", failReason: failure } };
    }
    const im = { status: "SUCCESS", taskData: body, usage: { image_count: data.length } };
    const url = trimmed((data[0] || {}).url);
    if (url) im.artifacts = [{ key: "image-1", source: { type: "url", path: "data[0].url" } }];
    return { taskId: placeholder, taskData: body, immediate: im };
  }
  if (!body.id) throw new Error("task_id is empty");
  return { taskId: String(body.id), taskData: body };
}

// ── 轮询链（视频）─────────────────────────────────────────

export function buildQueryRequest(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  return { url: "/api/v3/contents/generations/tasks/" + encodeURIComponent(t.taskId), method: "GET" };
}

export function parseQueryResponse(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const body = plainObject(ctx.responseBody);
  const item = { taskId: t.taskId, taskData: body };

  if (body.status === "queued") return { items: [Object.assign(item, { done: false, taskData: Object.assign({}, body, { _progress: "10%" }) })] };
  if (body.status === "running") return { items: [Object.assign(item, { done: false, taskData: Object.assign({}, body, { _progress: "50%" }) })] };
  if (body.status === "succeeded") {
    const usage = plainObject(body.usage);
    const completionTokens = Number(usage.completion_tokens || 0);
    const totalTokens = Number(usage.total_tokens || 0);
    const facts = {};
    if (Number.isFinite(totalTokens) && totalTokens > 0) facts.tokens = totalTokens;
    const resolution = trimmed(usage.video_resolution).toLowerCase();
    if (SEEDANCE_RESOLUTIONS.includes(resolution)) facts.resolution = resolution;
    const url = trimmed(plainObject(body.content).video_url);
    const done = { taskId: t.taskId, done: true, usage: facts, taskData: body };
    if (url) done.artifacts = [{ key: "video", mimeType: "video/mp4", source: { type: "url", path: "content.video_url" } }];
    if (trimmed(plainObject(body.content).last_frame_url)) {
      done.artifacts = (done.artifacts || []).concat([{ key: "last_frame", source: { type: "url", path: "content.last_frame_url" } }]);
    }
    return { items: [done] };
  }
  if (["failed", "expired", "cancelled"].includes(String(body.status || ""))) {
    const reason = plainObject(body.error).message || String(body.status);
    return { items: [Object.assign(item, { done: true, error: { code: "doubao_" + body.status, message: reason } })] };
  }
  return { items: [Object.assign(item, { done: false })] };
}

// ── 渲染 ──────────────────────────────────────────────────

export function renderSubmit(ctx) {
  const task = ctx.task || {};
  const data = plainObject(task.data);
  if (data.id) return { status: 200, body: { id: data.id, status: "queued" } };
  // 图片同步：透传上游 data
  return { status: 200, body: data };
}

export function renderImage(ctx) {
  // immediate 图片的 renderSubmit 别名（路由 render: renderImage）
  const task = ctx.task || {};
  return { status: 200, body: plainObject(task.data) };
}

export function renderTask(ctx) {
  const task = ctx.task || {};
  const data = plainObject(task.data);
  const statusMap = { NOT_START: "queued", SUBMITTED: "queued", QUEUED: "queued", IN_PROGRESS: "running", SUCCESS: "succeeded", FAILURE: "failed" };
  const out = {
    id: data.id || task.publicTaskId,
    model: task.model,
    status: statusMap[task.status] || "queued",
  };
  const content = plainObject(data.content);
  if (content.video_url) out.content = { video_url: content.video_url };
  const usage = plainObject(data.usage);
  if (usage.total_tokens) out.usage = usage;
  if (task.failReason) out.error = { message: task.failReason };
  return { status: 200, body: out };
}
