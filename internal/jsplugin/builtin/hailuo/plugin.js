// hailuo：MiniMax 海螺视频生成插件，v1 API（2.3 / 02 / 01 系列九模型）+ H3 多模态参考（/v2）。
//
// 上游协议：
//   POST {base}/v1/video_generation → {base_resp,task_id}
//   GET  {base}/v1/query/video_generation?task_id= → {base_resp,status,file_id}
//   GET  {base}/v1/files/retrieve?file_id= → {file:{download_url}}
//   POST {base}/v2/video_generation → {task_id}
//   GET  {base}/v2/query/video_generation/{id} → {task_id,status,content}
//
// 实现要点：
// ① meta：icon 用宿主图标名 video；usageSchema 为 map 形态（秒数 + 分辨率联合档位）。
// ② routes：两条提交（/v1 与 /v2 按模型分流）+ 一条统一查询，
//    查询地址按 taskData._isH3 判定。
// ③ 解码：createTask/createH3Task 校验模型归属后原样透传 vendorBody；
//    H3 时长限制在 4–15 秒。
// ④ buildSubmitRequest：相对 URL，鉴权走渠道默认 Bearer（MiniMax 无需签名）。
// ⑤ parseSubmitResponse：v1 校验 base_resp 信封，v2 取裸 task_id。
// ⑥ parseQueryResponse：两套 API 的状态映射。v1 只给 file_id，需再调
//    /v1/files/retrieve 换签名 download_url 并写入 taskData._video_url
//    （供产物直链下载）；H3 直接取 content.url。
// ⑦ 用量：H3 按响应 usage 精确结算；v1 上游不返回用量，预扣即终价。
//    estimateUsage 按秒预扣，H3 另计 input_images / input_video_seconds。
// ⑧ renderSubmit/renderTask：taskCreated/taskStatus 形态的 {status, body} 渲染。
// ⑨ 产物仅支持直连渠道形态：经网关中转的 v1 产物路径未实现。
const VIDEO_SECONDS_FIELD = { type: "number", unit: "second", description: { en: "Video generation unit price", zh: "视频生成单价" } };
const RESOLUTION_DESCRIPTION = { en: "Output video resolution", zh: "输出视频分辨率" };

const H3_MODEL = "MiniMax-H3";
const V1_MODELS = ["MiniMax-Hailuo-2.3", "MiniMax-Hailuo-2.3-Fast", "MiniMax-Hailuo-02", "T2V-01-Director", "T2V-01", "I2V-01-Director", "I2V-01-live", "I2V-01", "S2V-01"];
const V1_METADATA_FIELDS = ["prompt_optimizer", "fast_pretreatment", "aigc_watermark", "first_frame_image", "last_frame_image", "subject_reference"];
const H3_METADATA_FIELDS = ["aigc_watermark"];

const H3_MIN_DURATION = 4;
const H3_MAX_DURATION = 15;
const H3_DEFAULT_DURATION = 5;
const H3_MAX_INPUT_VIDEO_SECONDS = 15;

export const meta = {
  apiVersion: 1,
  key: "hailuo",
  name: "Hailuo Video",
  icon: "video",
  description: {
    en: "MiniMax Hailuo video generation (text-to-video, image-to-video, and MiniMax-H3 multimodal reference)",
    zh: "MiniMax 海螺视频生成（文生视频、图生视频、MiniMax-H3 多模态参考生视频）",
  },
  version: "1.5.0",
  author: "qianfree",
  channelTypes: [35],
  models: [H3_MODEL].concat(V1_MODELS),
  fetchMode: "per_task",
  standardVideos: "decodeVideos",
  usageSchema: {
    seconds: VIDEO_SECONDS_FIELD,
    resolution: { enum: ["512P", "768P", "720P", "1080P", "2K"], description: RESOLUTION_DESCRIPTION },
    input_images: { type: "number", unit: "count", description: { en: "Input image unit price", zh: "输入图片单价" } },
    input_video_seconds: { type: "number", unit: "second", description: { en: "Input video unit price", zh: "输入视频单价" } },
  },
  routes: [
    { method: "POST", path: "/v1/video_generation", type: "submit", decode: "createTask", render: "renderSubmit" },
    { method: "POST", path: "/v2/video_generation", type: "submit", decode: "createH3Task", render: "renderSubmit" },
    { method: "GET", path: "/videos/:task_id", type: "query", render: "renderTask" },
  ],
};

function trimmed(v) { return String(v || "").trim(); }
function plainObject(v) { return v && typeof v === "object" && !Array.isArray(v) ? v : {}; }
function isH3(model) { return model === H3_MODEL; }
function isModernHailuo(model) { return ["MiniMax-Hailuo-2.3", "MiniMax-Hailuo-2.3-Fast", "MiniMax-Hailuo-02"].includes(model); }
function defaultResolution(model) { return isModernHailuo(model) ? "768P" : "720P"; }
function resolutionFor(size, model) {
  const v = String(size || "");
  if (v.includes("1080")) return "1080P";
  if (v.includes("768")) return "768P";
  if (v.includes("720")) return isModernHailuo(model) ? "768P" : "720P";
  if (v.includes("512")) return "512P";
  return defaultResolution(model);
}
function outboundDuration(req) {
  const n = Number(req && req.duration);
  return Number.isFinite(n) && n > 0 ? n : 6;
}
function outboundResolution(req, model) {
  if (req && req.resolution) return resolutionFor(req.resolution, model);
  const metadata = plainObject(req && req.metadata);
  if (metadata.resolution) return resolutionFor(metadata.resolution, model);
  if (req && req.size) return resolutionFor(req.size, model);
  return defaultResolution(model);
}
function definedFields(source, keys) {
  const out = {};
  for (const k of keys) if (source[k] !== undefined && source[k] !== null) out[k] = source[k];
  return out;
}
function hasHailuoImage(body) {
  const metadata = plainObject(body.metadata);
  return Boolean(trimmed(body.input_reference) || trimmed(body.image) || (Array.isArray(body.images) && body.images.length) || metadata.first_frame_image || metadata.last_frame_image || metadata.subject_reference);
}

// ── 解码 ──────────────────────────────────────────────────

export function createTask(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model);
  if (!V1_MODELS.includes(model)) throw new Error("model must be one of " + V1_MODELS.join(", "));
  if (body.duration !== undefined && body.duration !== null && (!Number.isFinite(Number(body.duration)) || Number(body.duration) <= 0)) throw new Error("duration must be positive");
  return { kind: "submit", model: model, requestBody: { model: model, vendorBody: body } };
}

export function createH3Task(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model) || H3_MODEL;
  if (!isH3(model)) throw new Error("/v2 requires " + H3_MODEL);
  if (body.duration !== undefined && body.duration !== null) {
    const n = Number(body.duration);
    if (!Number.isFinite(n) || n < H3_MIN_DURATION || n > H3_MAX_DURATION) throw new Error("duration must be between " + H3_MIN_DURATION + " and " + H3_MAX_DURATION);
  }
  return { kind: "submit", model: H3_MODEL, requestBody: { model: H3_MODEL, vendorBody: body } };
}

// decodeVideos 标准视频端点解码（meta.standardVideos 指向）：通用任务体 → 各协议
// vendorBody。分辨率经 resolutionFor 归一（metadata.size 支持 720/768/1080 等数字词），
// 时长 seconds 优先、回落 metadata.duration；参考图 images[0]/metadata.image →
// v1 first_frame_image / H3 content[] 首帧项；尾帧 metadata.last_frame 同理。
// 词汇映射与 Go 版 minimax 适配器同口径（resolveDuration/resolveResolution/buildGenericContent）。
export function decodeVideos(ctx) {
  const raw = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const metadata = plainObject(raw.metadata);
  const model = trimmed(ctx.model || raw.model);
  if (!model) throw new Error("model is required");
  if (!trimmed(raw.prompt)) throw new Error("prompt is required");

  const seconds = raw.seconds != null ? raw.seconds : metadata.duration;
  const n = typeof seconds === "string" && seconds.trim() !== "" ? Number(seconds) : seconds;
  const duration = Number.isFinite(n) && n > 0 ? Math.round(n) : null;
  // 分辨率：显式输入（resolution/size）经 resolutionFor 归一；缺省按协议默认——
  // H3 与 estimateUsage 的 H3 分支同口径（768P），v1 系列走 defaultResolution
  const rawResolution = trimmed(metadata.resolution) || trimmed(metadata.size);
  const resolution = rawResolution ? resolutionFor(rawResolution, model) : (isH3(model) ? "768P" : defaultResolution(model));
  const image = Array.isArray(raw.images) && raw.images.length && typeof raw.images[0] === "string" ? raw.images[0].trim() : trimmed(metadata.image);
  const lastFrame = trimmed(metadata.last_frame);

  if (isH3(model)) {
    if (duration != null && (duration < H3_MIN_DURATION || duration > H3_MAX_DURATION)) {
      throw new Error("duration must be between " + H3_MIN_DURATION + " and " + H3_MAX_DURATION);
    }
    const content = [{ type: "text", text: String(raw.prompt) }];
    if (image) content.push({ type: "image_url", image_url: { url: image }, role: "first_frame" });
    if (lastFrame) content.push({ type: "image_url", image_url: { url: lastFrame }, role: "last_frame" });
    const vendorBody = { content: content, resolution: resolution };
    if (duration != null) vendorBody.duration = duration;
    const ratio = trimmed(metadata.ratio) || trimmed(metadata.aspect_ratio);
    if (ratio) vendorBody.ratio = ratio;
    return { kind: "submit", model: model, requestBody: { model: model, vendorBody: vendorBody } };
  }

  if (!V1_MODELS.includes(model)) throw new Error("model must be " + H3_MODEL + " or one of " + V1_MODELS.join(", "));
  const vendorBody = { prompt: String(raw.prompt), resolution: resolution };
  if (duration != null) vendorBody.duration = duration;
  if (image) vendorBody.first_frame_image = image;
  if (lastFrame) vendorBody.last_frame_image = lastFrame;
  return { kind: "submit", model: model, requestBody: { model: model, vendorBody: vendorBody } };
}

// ── 提交链 ────────────────────────────────────────────────

export function estimateUsage(ctx) {
  const req = ctx.body || {};
  const model = ctx.model;
  if (isH3(model)) {
    const vendor = plainObject(req.vendorBody);
    const content = Array.isArray(vendor.content) ? vendor.content : [];
    const duration = vendor.duration == null ? H3_DEFAULT_DURATION : Number(vendor.duration);
    return {
      seconds: duration,
      resolution: trimmed(vendor.resolution).toUpperCase() === "2K" ? "2K" : "768P",
      input_images: content.filter(function (c) { return c && c.type === "image_url"; }).length,
      input_video_seconds: content.some(function (c) { return c && c.type === "video_url"; }) ? H3_MAX_INPUT_VIDEO_SECONDS : 0,
    };
  }
  const vendor = plainObject(req.vendorBody);
  return { seconds: outboundDuration(vendor), resolution: outboundResolution(vendor, model), input_images: 0, input_video_seconds: 0 };
}

export function buildSubmitRequest(ctx) {
  const req = ctx.body || {};
  const model = ctx.upstreamModel || ctx.model;
  if (isH3(model)) {
    const vendor = plainObject(req.vendorBody);
    const body = Object.assign({}, definedFields(vendor, ["model", "content", "resolution", "duration", "ratio"]), definedFields(vendor.metadata || {}, H3_METADATA_FIELDS), { model: model });
    return { url: "/v2/video_generation", method: "POST", body: body };
  }
  const vendor = plainObject(req.vendorBody);
  const body = Object.assign({}, vendor, definedFields(vendor.metadata || {}, V1_METADATA_FIELDS), { model: model });
  return { url: "/v1/video_generation", method: "POST", body: body };
}

// /v2 错误形态（base_resp 或裸 status_code/message）
function h3APIError(body) {
  const b = plainObject(body);
  if (b.base_resp && b.base_resp.status_code) return { statusCode: b.base_resp.status_code, message: trimmed(b.base_resp.status_msg) || "hailuo error" };
  return null;
}

export function parseSubmitResponse(ctx) {
  const body = plainObject(ctx.responseBody);
  const apiError = isH3(ctx.model) ? h3APIError(body) : null;
  if (apiError) throw new Error(apiError.message);
  const base = body.base_resp;
  if (base) {
    if (base.status_code !== 0) throw new Error(base.status_msg || "hailuo submit failed");
  } else if (!isH3(ctx.model)) {
    throw new Error("hailuo submit failed");
  }
  if (!body.task_id) throw new Error("missing task_id");
  const taskData = Object.assign({}, body, { _isH3: isH3(ctx.model) });
  return { taskId: String(body.task_id), taskData: taskData };
}

// ── 轮询链 ────────────────────────────────────────────────

export function buildQueryRequest(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const td = plainObject(t.taskData);
  const id = encodeURIComponent(t.taskId);
  if (td._isH3) return { url: "/v2/query/video_generation/" + id, method: "GET" };
  return { url: "/v1/query/video_generation?task_id=" + id, method: "GET" };
}

// /v2 任务形态判定（task_id + queued/running/... 状态）
function h3QueryTask(body) {
  const b = plainObject(body);
  if (!b.task_id && !b.status) return null;
  if (["queued", "running", "succeeded", "failed", "cancelled"].includes(String(b.status || ""))) return b;
  return null;
}

// v1 file_id → 签名 download_url（utils.fetch，渠道域内）
function resolveV1DownloadURL(fileID) {
  const response = utils.fetch({
    url: "/v1/files/retrieve?file_id=" + encodeURIComponent(fileID),
    headers: { Accept: "application/json" },
  });
  const base = plainObject(plainObject(response.body).base_resp);
  if (response.status !== 200 || base.status_code) {
    throw new Error("MiniMax file retrieve failed: " + (base.status_msg || "HTTP " + response.status));
  }
  return trimmed(plainObject(plainObject(response.body).file).download_url);
}

export function parseQueryResponse(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const td = plainObject(t.taskData);
  const body = plainObject(ctx.responseBody);
  const item = { taskId: t.taskId, taskData: body };

  const apiError = h3APIError(body);
  if (apiError && apiError.statusCode !== 408 && apiError.statusCode !== 429 && apiError.statusCode < 500) {
    return { items: [Object.assign(item, { done: true, error: { code: "hailuo_error", message: apiError.message } })] };
  }

  const h3Task = h3QueryTask(body);
  if (h3Task || td._isH3) {
    const task = h3Task || {};
    const statuses = { queued: "SUBMITTED", running: "IN_PROGRESS", succeeded: "SUCCESS", failed: "FAILURE", cancelled: "FAILURE" };
    const status = statuses[String(task.status || "")];
    if (!status) return { items: [Object.assign(item, { done: false })] };
    if (status === "SUCCESS") {
      const url = trimmed(plainObject(task.content).url);
      const usage = {};
      const resolution = trimmed(task.resolution).toUpperCase();
      if (resolution === "2K" || resolution === "768P") usage.resolution = resolution;
      const u = plainObject(task.usage);
      if (Number.isFinite(Number(u.output_seconds)) && Number(u.output_seconds) > 0) usage.seconds = Number(u.output_seconds);
      if (Number.isFinite(Number(u.input_images)) && u.input_images > 0) usage.input_images = Number(u.input_images);
      if (Number.isFinite(Number(u.input_video_seconds)) && u.input_video_seconds > 0) usage.input_video_seconds = Number(u.input_video_seconds);
      const done = { taskId: t.taskId, done: true, usage: usage, taskData: body };
      if (url) done.artifacts = [{ key: "video", mimeType: "video/mp4", source: { type: "url", path: "_video_url" } }];
      done.taskData = Object.assign({}, body, { _video_url: url });
      return { items: [done] };
    }
    if (status === "FAILURE") {
      return { items: [Object.assign(item, { done: true, error: { code: "hailuo_failed", message: trimmed(plainObject(task.error).message) || "task failed" } })] };
    }
    return { items: [Object.assign(item, { done: false })] };
  }

  // v1：base_resp 信封 + Preparing/Queueing/Processing/Success/Fail
  if (body.base_resp && body.base_resp.status_code !== 0) {
    return { items: [Object.assign(item, { done: true, error: { code: "hailuo_error", message: body.base_resp.status_msg || "" } })] };
  }
  const statuses = { Preparing: "IN_PROGRESS", Queueing: "IN_PROGRESS", Processing: "IN_PROGRESS", Success: "SUCCESS", Fail: "FAILURE" };
  const status = statuses[String(body.status || "")];
  if (!status) return { items: [Object.assign(item, { done: false })] };
  if (status === "SUCCESS") {
    const fileID = trimmed(body.file_id);
    let url = "";
    try {
      if (fileID) url = resolveV1DownloadURL(fileID);
    } catch (_e) {
      url = ""; // 解析失败：产物不转存，任务仍成功（客户端可经任务数据取 file_id）
    }
    const done = { taskId: t.taskId, done: true, taskData: body };
    if (url) {
      done.artifacts = [{ key: "video", mimeType: "video/mp4", source: { type: "url", path: "_video_url" } }];
      done.taskData = Object.assign({}, body, { _video_url: url });
    }
    return { items: [done] };
  }
  if (status === "FAILURE") {
    return { items: [Object.assign(item, { done: true, error: { code: "hailuo_failed", message: "task failed" } })] };
  }
  return { items: [Object.assign(item, { done: false })] };
}

// ── 渲染 ──────────────────────────────────────────────────

export function renderSubmit(ctx) {
  const task = ctx.task || {};
  const data = plainObject(task.data);
  const out = Object.assign({}, data);
  delete out._isH3;
  if (out.base_resp) {
    return { status: 200, body: Object.assign(out, { task_id: task.taskId || out.task_id }) };
  }
  return { status: 200, body: Object.assign(out, { task_id: task.taskId || out.task_id }) };
}

export function renderTask(ctx) {
  const task = ctx.task || {};
  const data = plainObject(task.data);
  const statusMap = { NOT_START: "Preparing", SUBMITTED: "Queueing", QUEUED: "Queueing", IN_PROGRESS: "Processing", SUCCESS: "Success", FAILURE: "Fail" };
  const out = Object.assign({}, data);
  delete out._isH3;
  delete out._video_url;
  out.status = statusMap[task.status] || String(task.status || "");
  if (task.failReason) out.base_resp = Object.assign({}, plainObject(out.base_resp), { status_code: -1, status_msg: task.failReason });
  return { status: 200, body: out };
}
