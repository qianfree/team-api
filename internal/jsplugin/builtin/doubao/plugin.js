// doubao：火山引擎豆包插件，Seedance 视频生成 + Seedream 图片生成（同步）。
//
// 上游协议：
//   POST {base}/api/v3/contents/generations/tasks → {id,status}
//   GET  {base}/api/v3/contents/generations/tasks/{id} → {status,usage,content}
//   POST {base}/api/v3/images/generations → {data:[{url}]}
//
// 实现要点：
// ① 视频（Seedance）：提交纯 JSON（SSE 只用于图片流式，视频本就是 JSON）。
// ② 图片（Seedream）：仅非流式——走宿主 immediate 同步链路，提交时强制
//    stream=false，请求带 stream:true 直接拒绝（宿主无 SSE 提交响应钩子）；
//    上游不给任务 ID，用 "sync-xxxx" 占位。
// ③ 用量（宿主固定词汇，计价口径由宿主模型定价决定）：output_tokens 按
//    秒 × 宽 × 高 × 24 / 1024 估算，终态用响应 usage.total_tokens 回填并取
//    usage.video_resolution；含参考视频时 input_video_seconds 按 Seedance 参考视频
//    总时长上限 15 秒报（Ark 不回报实际值，公式可据 u("input_video_seconds") > 0
//    判定「含视频输入」档）；generate_audio 布尔；图片按 output_count 报张数。
// ④ 1.8.0：契约升级——meta.models 为支持的上游模型（跨插件可重名，去掉 sharedModels）；
//    按型号的档位/类型校验移到 validateRequest（ctx.upstreamModel），decode 只做体形归一；
//    原 video_input_ratio 等价格系数不再由插件计算，归宿主计费公式。
// ⑤ routes：视频提交 + 视频查询 + 图片提交（immediate）。
// ⑥ fileRef：视频参考图支持 content[].image_url 的 fileRef 占位。
// ⑦ renderSubmit/renderImage/renderTask：Ark 信封形态的 {status, body} 渲染。
// ⑧ 标准端点参考素材（1.7.0）：通用任务体 references[]（role 词汇与 Ark content[].role
//    一致）按原序映射为带 role 的 content 项；Seedance 模型层约束在此校验（见
//    referenceContent）。无 references 时回落兼容字段 images[]/metadata.image。
// ⑨ 1.9.0：扩展模型——meta.modelsFromConfig=extra_models，上游新型号（新 Seedance/
//    Seedream）经配置登记即并入有效清单，无需更新插件；未收录型号走宽容路径：图像
//    路由仅拒绝已知视频型号（扩展型号可能是新 Seedream），视频路由按 DEFAULT_VIDEO_PROFILE
//    默认档（全档分辨率/30s/多模态参考），参数合法性由上游校验。
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
  version: "2.0.0",
  author: "qianfree",
  channelTypes: [51],
  models: Object.keys(VIDEO_MODELS).concat(Object.keys(IMAGE_MODELS)),
  fetchMode: "per_task",
  standardVideos: "decodeVideos",
  // 标准端点 references[] 可承接的角色（模型层约束在 referenceContent 校验）
  referenceRoles: ["first_frame", "last_frame", "reference_image", "reference_video", "reference_audio"],
  // 扩展模型：上游新型号（新 Seedance/Seedream）无需更新插件——配置登记即并入有效
  // 清单，配合渠道模型映射使用；未收录型号按默认档透传，参数合法性由上游校验
  modelsFromConfig: "extra_models",
  config: [
    {
      key: "extra_models", label: "扩展上游模型", type: "text", group: "模型",
      description: "每行一个火山方舟上游模型名（如新发布的 Seedance/Seedream 型号），保存后并入本插件支持的上游模型清单（渠道模型映射配合使用）；未收录型号按默认档位计费预估，参数合法性由上游校验",
    },
  ],
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

// ── 参考素材（标准端点 references[]）────────────────────────

// Seedance 多模态参考上限（官方：图 0-9 / 视频 0-3 / 音频 0-3）
const MAX_REF_IMAGES = 9;
const MAX_REF_VIDEOS = 3;
const MAX_REF_AUDIOS = 3;
const REF_ITEM_TYPES = {
  first_frame: "image_url", last_frame: "image_url", reference_image: "image_url",
  reference_video: "video_url", reference_audio: "audio_url",
};

// referenceContent references[] → Ark content 媒体项（保持原序：提示词「图片N/视频N/音频N」
// 按同类素材在 content 中的顺序编号）。网关已做协议层校验（role/来源/首尾帧与参考互斥），
// 这里只校验 Seedance 模型层约束：
//   - 多模态参考（reference_*）仅 2.x（profile.videoInput）支持
//   - 图 ≤9、视频 ≤3、音频 ≤3；音频须搭配至少 1 张图或 1 段视频
//   - 参考视频不收 base64（官方仅 URL / asset://）；尾帧须搭配首帧
function referenceContent(refs) {
  let hasFirst = false;
  let hasLast = false;
  const items = [];
  for (let i = 0; i < refs.length; i++) {
    const ref = plainObject(refs[i]);
    const role = trimmed(ref.role);
    const url = trimmed(ref.url);
    const type = REF_ITEM_TYPES[role];
    if (!type) throw new Error("references[" + i + "].role " + JSON.stringify(role) + " is not supported");
    if (!url) {
      // /v1/video/generations 不经网关解析 file_id
      throw new Error("references[" + i + "] must provide url" + (trimmed(ref.file_id) ? " (file_id is only supported on /v1/videos)" : ""));
    }
    if (role === "first_frame") hasFirst = true;
    if (role === "last_frame") hasLast = true;
    if (role === "reference_video" && url.toLowerCase().indexOf("data:") === 0) {
      throw new Error("references[" + i + "]: reference_video does not accept base64 data URLs; upload via /v1/files and use file_id, or pass a public URL");
    }
    const item = { type: type, role: role };
    item[type] = { url: url };
    items.push(item);
  }
  if (hasLast && !hasFirst) throw new Error("last_frame requires a first_frame");
  return items;
}

// referenceRules 按上游型号校验 content[] 多模态参考约束（带 role=reference_* 的项）：
// 多模态仅 Seedance 2.x；图 ≤9、视频 ≤3、音频 ≤3；音频须搭配至少 1 张图或 1 段视频。
function referenceRules(model, content) {
  const profile = videoProfile(model);
  const counts = { image_url: 0, video_url: 0, audio_url: 0 };
  let multimodal = false;
  for (const c of Array.isArray(content) ? content : []) {
    const role = trimmed(plainObject(c).role);
    if (role.indexOf("reference_") !== 0) continue;
    multimodal = true;
    if (Object.prototype.hasOwnProperty.call(counts, c.type)) counts[c.type]++;
  }
  if (multimodal && !profile.videoInput) {
    throw new Error("reference_not_supported: " + model + " does not support reference_image/reference_video/reference_audio (Seedance 2.x only)");
  }
  if (multimodal) {
    if (counts.image_url > MAX_REF_IMAGES) throw new Error("reference images must not exceed " + MAX_REF_IMAGES);
    if (counts.video_url > MAX_REF_VIDEOS) throw new Error("reference videos must not exceed " + MAX_REF_VIDEOS);
    if (counts.audio_url > MAX_REF_AUDIOS) throw new Error("reference audios must not exceed " + MAX_REF_AUDIOS);
    if (counts.audio_url > 0 && counts.image_url === 0 && counts.video_url === 0) {
      throw new Error("reference_audio requires at least one reference_image or reference_video");
    }
  }
}

// ── 解码 ──────────────────────────────────────────────────

// 解码只做体形归一，不按型号校验：body.model / ctx.model 是平台模型名（可能是渠道
// 映射别名），上游型号约束在 validateRequest（ctx.upstreamModel）校验。
export function createTask(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model);
  if (!model) throw new Error("model is required");
  return { kind: "submit", model: model, requestBody: { vendorBody: body } };
}

export function createImage(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model);
  if (!model) throw new Error("model is required");
  if (body.stream === true) throw new Error("stream is not supported; use the non-streaming image API");
  return { kind: "submit", model: model, requestBody: { vendorBody: body, _image: true } };
}

// decodeVideos 标准视频端点解码（meta.standardVideos 指向）：通用任务体 → Ark
// vendorBody（content[] 多模态）。prompt → text 项；references[] → 带 role 的
// image_url/video_url/audio_url 项（无 references 时 images[]/metadata.image →
// image_url 项）；seconds/metadata.duration → duration；metadata.resolution 按档位校验
// （与 createTask 同规则）；ratio（顶层/metadata.ratio）与 generate_audio 透传。
// 词汇映射与 Go 版 volcengine 适配器同口径。
export function decodeVideos(ctx) {
  const raw = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const metadata = plainObject(raw.metadata);
  const model = trimmed(ctx.model || raw.model);
  if (!model) throw new Error("model is required");

  const prompt = String(raw.prompt || "").trim();
  if (!prompt) throw new Error("prompt is required");
  const content = [{ type: "text", text: prompt }];
  if (Array.isArray(raw.references) && raw.references.length) {
    // references 为唯一权威来源（首帧 = role first_frame），忽略兼容字段防重复；
    // Seedance 型号层约束（多模态仅 2.x 等）在 validateRequest 按上游型号校验
    Array.prototype.push.apply(content, referenceContent(raw.references));
  } else {
    const images = Array.isArray(raw.images) && raw.images.length
      ? raw.images.filter(function (u) { return typeof u === "string" && u.trim(); })
      : (trimmed(metadata.image) ? [trimmed(metadata.image)] : []);
    for (const url of images) content.push({ type: "image_url", image_url: { url: url.trim() } });
  }

  const vendorBody = { content: content };
  const seconds = raw.seconds != null ? raw.seconds : metadata.duration;
  const n = typeof seconds === "string" && seconds.trim() !== "" ? Number(seconds) : seconds;
  if (Number.isFinite(n) && n > 0) vendorBody.duration = Math.round(n);
  const resolution = trimmed(metadata.resolution).toLowerCase();
  if (resolution) vendorBody.resolution = resolution;
  const ratio = trimmed(raw.ratio) || trimmed(metadata.ratio);
  if (ratio) vendorBody.ratio = ratio;
  if (metadata.generate_audio != null) vendorBody.generate_audio = metadata.generate_audio;
  return { kind: "submit", model: model, requestBody: { vendorBody: vendorBody } };
}

// validateRequest 渠道选定后按上游型号校验：图片路由仅拒绝已知 Seedance 视频型号
// （扩展型号可能是新 Seedream，放行由上游判定）、视频请求不得指向已知 Seedream；
// 视频分辨率按型号档位校验（扩展型号走 DEFAULT_VIDEO_PROFILE 全档）；多模态参考
// 约束（referenceRules）按型号校验。
export function validateRequest(ctx) {
  const req = plainObject(ctx.body);
  const upstream = trimmed(ctx.upstreamModel || ctx.model);
  if (req._image) {
    if (VIDEO_MODELS[upstream]) throw new Error(upstream + " is a Seedance video model; image endpoints are not supported");
    return {};
  }
  if (IMAGE_MODELS[upstream]) throw new Error(upstream + " is an image model; video endpoints are not supported");
  const profile = videoProfile(upstream);
  const vendor = plainObject(req.vendorBody);
  const raw = trimmed(vendor.resolution).toLowerCase();
  if (SEEDANCE_RESOLUTIONS.includes(raw) && !profile.resolutions.includes(raw)) {
    throw new Error(upstream + " resolution must be one of " + profile.resolutions.join(", "));
  }
  referenceRules(upstream, vendor.content);
  return {};
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
// Seedance 参考视频总时长上限（秒）：Ark 不回报实际输入视频时长，含参考视频时按上限报
const MAX_INPUT_VIDEO_SECONDS = 15;

export function estimateUsage(ctx) {
  const req = ctx.body || {};
  if (req._image) {
    const vendor = plainObject(req.vendorBody);
    return { output_count: Math.min(Math.max(Number.isFinite(Number(vendor.n)) ? Number(vendor.n) : 1, 1), 40) };
  }
  const model = trimmed(ctx.upstreamModel || ctx.model);
  const profile = videoProfile(model);
  const vendor = plainObject(req.vendorBody);
  // -1（模型自选时长）与缺省：保留最长时长口径
  let seconds = Number(vendor.duration);
  if (!Number.isFinite(seconds) || seconds <= 0) seconds = profile.maxDuration;
  seconds = Math.min(seconds, 3600);
  const raw = trimmed(vendor.resolution).toLowerCase();
  const resolution = SEEDANCE_RESOLUTIONS.includes(raw) ? raw : (profile.resolutions.includes("1080p") ? "1080p" : profile.resolutions[profile.resolutions.length - 1]);
  const usage = {
    output_tokens: estimateTokens(seconds, resolution),
    output_seconds: seconds,
    resolution: resolution,
    input_video_seconds: hasVideo(vendor.content) ? MAX_INPUT_VIDEO_SECONDS : 0,
  };
  if (profile.audio) usage.generate_audio = vendor.generate_audio !== false;
  return usage;
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
    const im = { status: "SUCCESS", taskData: body, usage: { output_count: data.length } };
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
    const totalTokens = Number(usage.total_tokens || 0);
    const facts = {};
    if (Number.isFinite(totalTokens) && totalTokens > 0) facts.output_tokens = totalTokens;
    const resolution = trimmed(usage.video_resolution).toLowerCase();
    if (SEEDANCE_RESOLUTIONS.includes(resolution)) facts.resolution = resolution;
    const duration = Number(body.duration);
    if (Number.isFinite(duration) && duration > 0) facts.output_seconds = duration;
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
