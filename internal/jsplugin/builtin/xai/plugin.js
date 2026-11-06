// xai：自 new-api plugins/tasks/xai/plugin.js 移植（迁移批次 3/6）。
// xAI Grok Imagine 视频生成（文生/图生/参考生视频、视频编辑、视频延长）。
//
// ── 迁移差异日志（vs new-api 原版，787 行）──────────────────
// ① meta：icon 改宿主名（video）；usageExamples/usageProfiles 保留（宽容忽略）；
//    usageSchema map 形态含 boolean 型（moderated——宿主信息性声明）。
// ② routes：4 条原样（查询路由参数名 :request_id——宿主已支持任意参数名）。
// ③ 解码：createGeneration/createEdit/createExtension 别名（generationRequest/
//    videoInputRequest 校验逻辑原样，身份判定改 ctx.model）。
// ④ buildSubmitRequest：相对 URL；鉴权走宿主默认 Bearer（xai 无签名）。
// ⑤ parseSubmitResponse：双参→单参；extension 的 state.extension_seconds
//    改存 taskData._extension_seconds（宿主 v1 不持久化插件 state）。
// ⑥ parseQueryResponse：包装 parseTaskResult + extractUsageOnComplete（含
//    MP4 moov/tkhd 头探测输出分辨率——utils.fetch Range 读，宿主支持 bytes）。
// ⑦ estimateUsage ← extractUsage（去 billing_ratios 分支）。
// ⑧ renderSubmit/renderTask ← videoCreated/videoStatus 包装。
// ⑨ 删除：protocols（SSE 认领）、listArtifacts/buildContentRequest（声明式产物）。
// ─────────────────────────────────────────────────────────────
const VIDEO_MODELS = {
  "grok-imagine-video": { resolutions: ["480p", "720p"], videoInput: true },
  "grok-imagine-video-1.5": { resolutions: ["480p", "720p", "1080p"] },
  "grok-imagine-video-1.5-lite": { resolutions: ["480p", "720p", "1080p"] },
};
const ASPECT_RATIOS = ["1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "21:9", "5:2"];
const DEFAULT_RESOLUTION = "480p";
const DEFAULT_DURATION = 8;
const MAX_DURATION = 15;
const DEFAULT_EXTENSION_SECONDS = 6;
const MIN_EXTENSION_SECONDS = 2;
const MAX_EXTENSION_SECONDS = 10;
const MAX_EDIT_SECONDS = 9;
const MAX_EXTENSION_INPUT_SECONDS = 15;
const EDIT_RESOLUTION_CAP = "720p";
const RESOLUTION_DESCRIPTION = { en: "Output video resolution", zh: "输出视频分辨率" };

export const meta = {
  apiVersion: 1,
  key: "xai",
  name: "xAI",
  icon: "video",
  description: {
    en: "xAI Grok Imagine video generation (text-to-video, image-to-video, reference-to-video, video editing, and video extension)",
    zh: "xAI Grok Imagine 视频生成（文生视频、图生视频、参考生视频、视频编辑、视频延长）",
  },
  version: "1.2.0",
  author: "qianfree",
  channelTypes: [48],
  baseUrl: "https://api.x.ai",
  allowedHosts: ["vidgen.x.ai"],
  models: Object.keys(VIDEO_MODELS),
  fetchMode: "per_task",
  usageSchema: {
    seconds: { type: "number", unit: "second", description: { en: "Video generation unit price", zh: "视频生成单价" } },
    resolution: { enum: ["480p", "720p", "1080p"], description: RESOLUTION_DESCRIPTION },
    input_images: { type: "number", unit: "count", description: { en: "Input image unit price", zh: "输入图片单价" } },
    input_video_seconds: { type: "number", unit: "second", description: { en: "Input video unit price", zh: "输入视频单价" } },
    moderated: { type: "boolean", description: { en: "Blocked by content moderation", zh: "被内容审核拦截" } },
  },
  routes: [
    { method: "POST", path: "/videos/generations", type: "submit", decode: "createGeneration", render: "renderSubmit" },
    { method: "POST", path: "/videos/edits", type: "submit", decode: "createEdit", render: "renderSubmit" },
    { method: "POST", path: "/videos/extensions", type: "submit", decode: "createExtension", render: "renderSubmit" },
    { method: "GET", path: "/videos/:request_id", type: "query", render: "renderTask" },
  ],
};

function trimmed(v) { return String(v || "").trim(); }
function plainObject(v) { return v && typeof v === "object" && !Array.isArray(v) ? v : null; }

function videoModel(model) {
  const name = trimmed(model);
  if (VIDEO_MODELS[name]) return name;
  throw new Error((name || "model") + " is not a supported xAI video model");
}
function integerField(value, name, min, max) {
  const n = typeof value === "string" && value.trim() !== "" ? Number(value) : value;
  if (typeof n !== "number" || !Number.isInteger(n) || n < min || n > max)
    throw new Error(name + " must be an integer between " + min + " and " + max);
  return n;
}
function mediaInput(value, name) {
  const object = plainObject(value);
  if (object && object.file_id !== undefined) throw new Error(name + " must be a URL; file_id is not supported");
  const url = object ? (object.url !== undefined ? object.url : object.image_url) : value;
  if (plainObject(url) && url.__fileRef) return { url: url };
  if (typeof url === "string" && url.trim()) return { url: url.trim() };
  throw new Error(name + " must be a URL or {url}");
}
function promptField(source, required) {
  if (source.prompt !== undefined && source.prompt !== null && typeof source.prompt !== "string") throw new Error("prompt must be a string");
  const prompt = trimmed(source.prompt);
  if (required && !prompt) throw new Error("prompt is required");
  return prompt;
}
function commonFields(source, body) {
  if (source.storage_options !== undefined && source.storage_options !== null) throw new Error("storage_options is not supported");
  if (source.user !== undefined && source.user !== null) {
    if (typeof source.user !== "string") throw new Error("user must be a string");
    body.user = source.user;
  }
  return body;
}

// ── 解码（三条路由共享校验）────────────────────────────────

function generationRequest(source, model) {
  const image = source.image == null ? null : mediaInput(source.image, "image");
  const referenceImages = Array.isArray(source.reference_images) ? source.reference_images : [];
  const lastFrame = source.last_frame == null ? null : mediaInput(source.last_frame, "last_frame");
  let action = "text_to_video";
  if (referenceImages.length) action = "reference_to_video";
  else if (image || lastFrame) action = "image_to_video";
  const prompt = promptField(source, action !== "image_to_video");
  const resolution = source.resolution == null ? DEFAULT_RESOLUTION : trimmed(source.resolution).toLowerCase();
  const resolutions = VIDEO_MODELS[model].resolutions;
  if (!resolutions.includes(resolution)) throw new Error(model + " resolution must be one of " + resolutions.join(", "));
  if (action === "reference_to_video" && resolution === "1080p") throw new Error("reference-to-video supports resolution 480p or 720p");
  const duration = source.duration == null ? DEFAULT_DURATION : integerField(source.duration, "duration", 1, MAX_DURATION);
  const body = { duration: duration, resolution: resolution };
  if (prompt) body.prompt = prompt;
  if (source.aspect_ratio != null) {
    if (!ASPECT_RATIOS.includes(source.aspect_ratio)) throw new Error("aspect_ratio must be one of " + ASPECT_RATIOS.join(", "));
    body.aspect_ratio = source.aspect_ratio;
  }
  if (image) body.image = image;
  if (referenceImages.length) body.reference_images = referenceImages.map(mediaInput);
  if (lastFrame) body.last_frame = lastFrame;
  if (source.generate_audio != null) body.generate_audio = source.generate_audio;
  return { action: action, body: commonFields(source, body) };
}

function videoInputRequest(action, source, model) {
  if (!VIDEO_MODELS[model].videoInput) throw new Error("video editing and extension require grok-imagine-video");
  const body = { prompt: promptField(source, true), video: mediaInput(source.video, "video") };
  if (action === "video_extension") {
    body.duration = source.duration == null ? DEFAULT_EXTENSION_SECONDS : integerField(source.duration, "duration", MIN_EXTENSION_SECONDS, MAX_EXTENSION_SECONDS);
  }
  return { action: action, body: commonFields(source, body) };
}

function decodeByPath(kind) {
  return function (ctx) {
    const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
    const model = trimmed(body.model) || "grok-imagine-video";
    videoModel(model);
    const request = kind === "generation" ? generationRequest(body, model) : videoInputRequest(kind === "edit" ? "video_edit" : "video_extension", body, model);
    return { kind: "submit", model: model, requestBody: Object.assign({}, request.body, { _action: request.action }) };
  };
}

export const createGeneration = decodeByPath("generation");
export const createEdit = decodeByPath("edit");
export const createExtension = decodeByPath("extension");

// ── 提交链 ────────────────────────────────────────────────

export function estimateUsage(ctx) {
  const req = ctx.body || {};
  const model = videoModel(ctx.model);
  const action = req._action;
  let facts;
  if (action === "video_edit") {
    facts = { seconds: MAX_EDIT_SECONDS, resolution: EDIT_RESOLUTION_CAP, input_images: 0, input_video_seconds: MAX_EDIT_SECONDS };
  } else if (action === "video_extension") {
    facts = { seconds: req.duration || DEFAULT_EXTENSION_SECONDS, resolution: EDIT_RESOLUTION_CAP, input_images: 0, input_video_seconds: MAX_EXTENSION_INPUT_SECONDS };
  } else {
    const images = (req.image ? 1 : 0) + ((req.reference_images || []).length) + (req.last_frame ? 1 : 0);
    facts = { seconds: req.duration, resolution: req.resolution, input_images: images, input_video_seconds: 0 };
  }
  if (!VIDEO_MODELS[model].videoInput) delete facts.input_video_seconds;
  facts.moderated = false;
  return facts;
}

export function buildSubmitRequest(ctx) {
  const req = Object.assign({}, ctx.body || {});
  const action = req._action;
  delete req._action;
  const paths = { video_edit: "/v1/videos/edits", video_extension: "/v1/videos/extensions" };
  return {
    url: paths[action] || "/v1/videos/generations",
    method: "POST",
    body: Object.assign({}, req, { model: ctx.upstreamModel || ctx.model }),
  };
}

export function parseSubmitResponse(ctx) {
  const body = plainObject(ctx.responseBody) || {};
  const taskId = typeof body.request_id === "string" ? body.request_id.trim() : "";
  if (!taskId) throw new Error("xAI did not return a request_id");
  const taskData = Object.assign({}, body);
  if (String(ctx.path || "").includes("extensions")) {
    taskData._extension_seconds = (ctx.body && ctx.body.duration) || DEFAULT_EXTENSION_SECONDS;
  }
  return { taskId: taskId, taskData: taskData };
}

// ── 轮询链 ────────────────────────────────────────────────

export function buildQueryRequest(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  return { url: "/v1/videos/" + encodeURIComponent(t.taskId), method: "GET" };
}

function videoURL(data) {
  const video = plainObject(data) && plainObject(data.video);
  return video && typeof video.url === "string" ? video.url.trim() : "";
}
function filteredOutput(data) {
  return Boolean(plainObject(data) && plainObject(data.filtered_output));
}
function chargedFailure(data) {
  const failure = plainObject(data) && plainObject(data.failure);
  return Boolean(failure && failure.billed);
}
function failureReason(data) {
  const failure = plainObject(data) && plainObject(data.failure);
  if (failure && typeof failure.message === "string" && failure.message.trim()) return failure.message.trim();
  return "task failed";
}

// MP4 moov/tkhd 头探测输出分辨率（utils.fetch Range 读，≤3 次）
const PROBE_READ_BYTES = 64 * 1024;
const PROBE_MAX_READS = 3;

function readBytes(url, start, length) {
  const response = utils.fetch({ url: url, headers: { Range: "bytes=" + start + "-" + (start + length - 1) }, responseType: "bytes" });
  if (response.status !== 206 && response.status !== 200) throw new Error("HTTP " + response.status);
  let first = 0;
  if (response.status === 206) {
    const range = /^bytes (\d+)-/.exec(response.headers["Content-Range"] || "");
    first = range ? Number(range[1]) : start;
  }
  return { start: first, bytes: new Uint8Array(response.body) };
}
function be32(b, o) { return b[o] * 16777216 + b[o + 1] * 65536 + b[o + 2] * 256 + b[o + 3]; }
function boxHeader(bytes, offset, end) {
  if (offset + 8 > end) return null;
  let size = be32(bytes, offset);
  let header = 8;
  if (size === 1) {
    if (offset + 16 > end) return null;
    size = be32(bytes, offset + 8) * 4294967296 + be32(bytes, offset + 12);
    header = 16;
  }
  const open = size === 0;
  if (open) size = end - offset;
  if (size < header) return null;
  return { type: String.fromCharCode(bytes[offset + 4], bytes[offset + 5], bytes[offset + 6], bytes[offset + 7]), size: size, header: header };
}
function videoTrack(url) {
  let reads = 0;
  let chunk = { start: 0, bytes: new Uint8Array(0) };
  const read = function (start, length) {
    if (reads >= PROBE_MAX_READS) throw new Error("too many reads");
    reads++;
    chunk = readBytes(url, start, length);
  };
  let offset = 0;
  for (let i = 0; i < 32; i++) {
    if (offset < chunk.start || offset + 16 > chunk.start + chunk.bytes.length) read(offset, PROBE_READ_BYTES);
    const local = offset - chunk.start;
    const box = local >= 0 ? boxHeader(chunk.bytes, local, chunk.bytes.length) : null;
    if (!box) return null;
    if (box.type !== "moov") { offset += box.size; continue; }
    // moov 内遍历子盒找 tkhd（版本 0：偏移 84/88 处 width/height 为 16.16 定点）
    let sub = offset + box.header;
    const subEnd = offset + box.size;
    while (sub + 8 <= subEnd) {
      if (sub < chunk.start || sub + 92 > chunk.start + chunk.bytes.length) read(sub, PROBE_READ_BYTES);
      const localSub = sub - chunk.start;
      const subBox = boxHeader(chunk.bytes, localSub, chunk.bytes.length);
      if (!subBox) return null;
      if (subBox.type === "tkhd") {
        const version = chunk.bytes[localSub + 8];
        const wOff = localSub + (version === 1 ? 96 : 84);
        if (wOff + 8 <= chunk.bytes.length) {
          return { width: chunk.bytes[wOff] >> 8 ? chunk.bytes[wOff] / 256 : be32(chunk.bytes, wOff) / 65536, height: be32(chunk.bytes, wOff + 4) / 65536 };
        }
        return null;
      }
      sub += subBox.size;
    }
    return null;
  }
  return null;
}
function outputResolution(url) {
  try {
    const track = videoTrack(url);
    if (!track) return "";
    return Math.min(track.width, track.height) > 480 ? "720p" : "480p";
  } catch (_e) {
    return "";
  }
}

export function parseQueryResponse(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const body = plainObject(ctx.responseBody) || {};
  const status = body.status;
  const item = { taskId: t.taskId, taskData: body };

  if (ctx.status === 202 || status === "pending") {
    const pending = Object.assign({}, item, { done: false, taskData: body });
    const progress = Number(body.progress);
    if (progress > 0 && progress < 100) pending.taskData = Object.assign({}, body, { _progress: progress + "%" });
    return { items: [pending] };
  }
  if (status === "done") {
    const done = { taskId: t.taskId, done: true };
    const url = videoURL(body);
    const td = Object.assign({}, body);
    const usage = {};
    // 结算用量：时长/输入视频秒数/审核标记；编辑延长类探测输出分辨率
    const video = plainObject(body.video) || {};
    const duration = Number(video.duration);
    const td0 = t.taskData && typeof t.taskData === "object" ? t.taskData : {};
    const action = td0._action || String(ctx.path || "").includes("edits") ? td0._action : td0._action;
    if (filteredOutput(body) || chargedFailure(body)) usage.moderated = true;
    if (Number.isFinite(duration) && duration > 0) {
      if (action === "video_edit") {
        usage.seconds = Math.min(duration, MAX_EDIT_SECONDS);
        usage.input_video_seconds = usage.seconds;
      } else if (action === "video_extension") {
        usage.seconds = Math.min(Number(td0._extension_seconds) || DEFAULT_EXTENSION_SECONDS, duration);
        usage.input_video_seconds = Math.min(Math.max(duration - (Number(td0._extension_seconds) || DEFAULT_EXTENSION_SECONDS), 0), MAX_EXTENSION_INPUT_SECONDS);
      } else {
        usage.seconds = Math.min(duration, MAX_DURATION);
      }
    }
    if (action === "video_edit" || action === "video_extension") {
      if (url) {
        const resolution = outputResolution(url);
        if (resolution) usage.resolution = resolution;
        else usage.resolution = EDIT_RESOLUTION_CAP;
      } else usage.resolution = EDIT_RESOLUTION_CAP;
    } else if (body.resolution) usage.resolution = body.resolution;
    done.usage = usage;
    if (url) {
      done.artifacts = [{ key: "video", mimeType: "video/mp4", source: { type: "url", path: "video.url" } }];
    }
    done.taskData = td;
    return { items: [done] };
  }
  if (status === "failed") {
    if (chargedFailure(body)) {
      // 审核拦截仍计费：按成功结算但标记 moderated
      return { items: [{ taskId: t.taskId, done: true, usage: { moderated: true }, taskData: body }] };
    }
    return { items: [Object.assign(item, { done: true, error: { code: "xai_failed", message: failureReason(body) } })] };
  }
  if (status === "expired") {
    return { items: [Object.assign(item, { done: true, error: { code: "xai_expired", message: "the video expired before it was retrieved" } })] };
  }
  return { items: [Object.assign(item, { done: false })] };
}

// ── 渲染 ──────────────────────────────────────────────────

export function renderSubmit(ctx) {
  const task = ctx.task || {};
  const data = plainObject(task.data) || {};
  const out = Object.assign({}, data);
  delete out._extension_seconds;
  return { status: 200, body: Object.assign(out, { request_id: data.request_id || task.publicTaskId }) };
}

export function renderTask(ctx) {
  const task = ctx.task || {};
  const data = plainObject(task.data) || {};
  const statusMap = { NOT_START: "pending", SUBMITTED: "pending", QUEUED: "pending", IN_PROGRESS: "pending", SUCCESS: "done", FAILURE: "failed" };
  const out = {
    request_id: task.publicTaskId,
    status: statusMap[task.status] || "pending",
  };
  if (data.video && data.video.url) out.video = { url: data.video.url, duration: data.video.duration };
  if (task.status === "FAILURE") out.failure = { message: task.failReason || "task failed" };
  const progress = Number(String(task.progress || data.progress || "0").replace("%", ""));
  if (progress > 0 && progress < 100) out.progress = progress;
  return { status: 200, body: out };
}
