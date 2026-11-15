// alibaba：阿里云百炼 DashScope 插件，万相视频（wan2.2–wan3.0 全系）+ 千问/Z-Image 图像。
//
// 上游协议（除同步图像外均带请求头 X-DashScope-Async: enable）：
//   POST {base}/api/v1/services/aigc/video-generation/video-synthesis  → {output:{task_id},usage}
//   POST {base}/api/v1/services/aigc/image2video/video-synthesis       → {output:{task_id},usage}
//        （kf2v / s2v 系走 image2video，其余视频模型走 video-generation）
//   POST {base}/api/v1/services/aigc/text2image/image-synthesis        → {output:{task_id}}（异步）
//   POST {base}/api/v1/services/aigc/multimodal-generation/generation  → {output:{choices}}（同步）
//   GET  {base}/api/v1/tasks/{id}                                      → {output:{task_status,...},usage}
//
// 实现要点：
// ① 视频（万相全系）：X-DashScope-Async 异步提交 + /api/v1/tasks/{id} 轮询；
//    计费 seconds/resolution/audio 沿用 wan3.0 的 input+output 双段口径。
// ② 图像：千问系（multimodal-generation）同步出图，immediate 终态，上游无
//    任务 ID 用 "sync-xxxx" 占位；wan2.x 异步图像系（t2i / legacy / image26 等）
//    走异步任务，与视频共用同一个轮询端点。交错流式（enable_interleave/stream）
//    宿主无 SSE 提交钩子，直接拒绝。
// ③ models 表保留全系（视频 + 图像）；dated snapshot 按前缀归并到基础模型
//    （wan2.7-t2v-2026-04-25 → wan2.7-t2v）。
// ④ fileRef：视频参考图与图像输入的 url 位置支持 fileRef 占位。
// ⑤ renderSubmit/renderImage/renderTask：DashScope 信封形态的 {status, body} 渲染。
const WAN_MODELS = {
  "wan3.0-video": { resolutions: ["480P", "720P", "1080P"], defaultResolution: "1080P", maxDuration: 30 },
  "wan3.0-video-prime": { resolutions: ["480P", "720P", "1080P"], defaultResolution: "1080P", maxDuration: 30 },
  "wan2.7-t2v": { resolutions: ["720P", "1080P"], defaultResolution: "1080P", maxDuration: 15 },
  "wan2.7-i2v": { resolutions: ["720P", "1080P"], defaultResolution: "1080P", maxDuration: 15 },
  "wan2.6-t2v": { resolutions: ["720P", "1080P"], defaultResolution: "1080P", maxDuration: 15 },
  "wan2.6-t2v-us": { resolutions: ["720P", "1080P"], defaultResolution: "1080P", maxDuration: 15 },
  "wan2.6-i2v": { resolutions: ["720P", "1080P"], defaultResolution: "1080P", maxDuration: 15 },
  "wan2.6-i2v-flash": { resolutions: ["720P", "1080P"], defaultResolution: "1080P", maxDuration: 15 },
  "wan2.6-i2v-us": { resolutions: ["720P", "1080P"], defaultResolution: "1080P", maxDuration: 15 },
  "wan2.5-t2v-preview": { resolutions: ["480P", "720P", "1080P"], defaultResolution: "1080P", maxDuration: 10 },
  "wan2.5-i2v-preview": { resolutions: ["480P", "720P", "1080P"], defaultResolution: "1080P", maxDuration: 10 },
  "wan2.2-t2v-plus": { resolutions: ["480P", "1080P"], defaultResolution: "1080P", maxDuration: 5 },
  "wan2.2-i2v-flash": { resolutions: ["480P", "720P", "1080P"], defaultResolution: "720P", maxDuration: 5 },
  "wan2.2-i2v-plus": { resolutions: ["480P", "1080P"], defaultResolution: "1080P", maxDuration: 5 },
  "wan2.2-kf2v-flash": { resolutions: ["480P", "720P", "1080P"], defaultResolution: "720P", maxDuration: 5 },
  "wan2.2-s2v": { resolutions: ["480P", "720P"], defaultResolution: "480P", maxDuration: 10 },
};
const WAN_AUDIO_VIDEO_MODELS = ["wan3.0-video", "wan3.0-video-prime"];
// 同步图像（multimodal-generation，immediate）；其余图像系（t2i/legacy 等）异步任务
const SYNC_IMAGE_MODELS = ["qwen-image", "qwen-image-plus", "qwen-image-max", "qwen-image-2.0", "qwen-image-2.0-pro", "qwen-image-3.0", "qwen-image-3.0-pro", "qwen-image-edit", "qwen-image-edit-plus", "qwen-image-edit-max", "z-image-turbo"];
const ASYNC_IMAGE_MODELS = ["wan2.7-image-pro", "wan2.7-image", "wan2.6-image", "wan2.6-t2i", "wan2.5-t2i-preview", "wan2.2-t2i-flash", "wan2.2-t2i-plus", "wanx2.1-t2i-turbo", "wanx2.1-t2i-plus", "wanx2.0-t2i-turbo", "wan2.5-i2i-preview", "wanx2.1-imageedit"];

function trimmed(v) { return String(v || "").trim(); }
function plainObject(v) { return v && typeof v === "object" && !Array.isArray(v) ? v : {}; }

// dated snapshot → 基础模型（wan2.7-t2v-2026-04-25 → wan2.7-t2v）
function baseModel(model) {
  const name = trimmed(model);
  if (WAN_MODELS[name]) return name;
  const m = name.match(/^(wan[0-9.]+-(?:t2v|i2v|s2v|kf2v))-\d{4}-\d{2}-\d{2}$/);
  if (m && WAN_MODELS[m[1]]) return m[1];
  return name;
}
function isVideoModel(model) { return Boolean(WAN_MODELS[baseModel(model)]); }
function isSyncImage(model) { return SYNC_IMAGE_MODELS.includes(trimmed(model)); }
function isAsyncImage(model) { return ASYNC_IMAGE_MODELS.includes(trimmed(model)); }

function videoService(model) {
  const base = baseModel(model);
  return base === "wan2.2-kf2v-flash" || base === "wan2.2-s2v" ? "image2video" : "video-generation";
}

export const meta = {
  apiVersion: 1,
  key: "alibaba",
  name: "Alibaba Bailian",
  icon: "video",
  description: {
    en: "Alibaba Cloud Bailian image and video generation (Wan, Qwen-Image, Z-Image)",
    zh: "阿里云百炼图片与视频生成（万相、千问图像、Z-Image）",
  },
  version: "1.6.0",
  author: "qianfree",
  channelTypes: [79],
  models: Object.keys(WAN_MODELS)
    .concat(["wan2.7-t2v-2026-04-25", "wan2.7-t2v-2026-06-12", "wan2.7-i2v-2026-04-25"])
    .concat(SYNC_IMAGE_MODELS)
    .concat(ASYNC_IMAGE_MODELS),
  fetchMode: "per_task",
  standardVideos: "decodeVideos",
  usageSchema: {
    seconds: { type: "number", unit: "second", description: { en: "Video generation unit price", zh: "视频生成单价" } },
    resolution: { enum: ["480P", "720P", "1080P"], description: { en: "Output video resolution", zh: "输出视频分辨率" } },
    audio: { type: "boolean", description: { en: "Audio generated", zh: "是否生成音频" } },
    image_count: { type: "number", unit: "count", description: { en: "Delivered image count", zh: "交付图片数" } },
  },
  routes: [
    { method: "POST", path: "/api/v1/services/aigc/video-generation/video-synthesis", type: "submit", decode: "createTask", render: "renderSubmit" },
    { method: "POST", path: "/api/v1/services/aigc/image2video/video-synthesis", type: "submit", decode: "createTask", render: "renderSubmit" },
    { method: "POST", path: "/api/v1/services/aigc/multimodal-generation/generation", type: "submit", decode: "createImage", render: "renderImage" },
    { method: "POST", path: "/api/v1/services/aigc/text2image/image-synthesis", type: "submit", decode: "createImage", render: "renderSubmit" },
    { method: "GET", path: "/api/v1/tasks/:task_id", type: "query", render: "renderTask" },
  ],
};

// ── 解码 ──────────────────────────────────────────────────

export function createTask(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model);
  if (!model) throw new Error("model is required");
  if (!isVideoModel(model)) throw new Error(model + " is not a Wan video model");
  const profile = WAN_MODELS[baseModel(model)];
  const params = plainObject(body.parameters);
  if (params.resolution !== undefined && !profile.resolutions.includes(trimmed(params.resolution).toUpperCase())) {
    throw new Error(model + " resolution must be one of " + profile.resolutions.join(", "));
  }
  return { kind: "submit", model: model, requestBody: { model: model, vendorBody: body } };
}

export function createImage(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model);
  if (!model) throw new Error("model is required");
  if (!isSyncImage(model) && !isAsyncImage(model)) throw new Error(model + " is not a Bailian image model");
  const params = plainObject(body.parameters);
  if (params.stream === true || params.enable_interleave === true) {
    throw new Error("stream/interleave is not supported; use the non-streaming API");
  }
  return { kind: "submit", model: model, requestBody: { model: model, vendorBody: body, _image: true, _sync: isSyncImage(model) } };
}

// decodeVideos 标准视频端点解码（meta.standardVideos 指向）：通用任务体 → DashScope
// vendorBody（input/parameters 双段）。prompt → input.prompt；参考图（images[0]/
// metadata.image）→ wan3.0 input.media 数组 / wan2.x input.img_url；seconds/
// metadata.duration → parameters.duration；metadata.resolution 大写归一按档位校验
// （与 createTask 同规则）；ratio（metadata.ratio/aspect_ratio）透传；音频开关
// metadata.generate_audio（bool）优先、回落 metadata.sound（"on"/"off"）。
// 词汇映射对齐 Go 版 ali 适配器（不含 wan3.0 size 档位折算——请直接配 resolution）。
export function decodeVideos(ctx) {
  const raw = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const metadata = plainObject(raw.metadata);
  const model = trimmed(ctx.model || raw.model);
  if (!model) throw new Error("model is required");
  if (!isVideoModel(model)) throw new Error(model + " is not a Wan video model");
  if (!trimmed(raw.prompt)) throw new Error("prompt is required");
  const profile = WAN_MODELS[baseModel(model)];
  if (!profile) throw new Error(model + " is not a Wan video model");

  const input = { prompt: String(raw.prompt) };
  const image = Array.isArray(raw.images) && raw.images.length && typeof raw.images[0] === "string" ? raw.images[0].trim() : trimmed(metadata.image);
  if (image) {
    if (baseModel(model).startsWith("wan3.")) input.media = [{ type: "image", url: image }];
    else input.img_url = image;
  }

  const parameters = {};
  const seconds = raw.seconds != null ? raw.seconds : metadata.duration;
  const n = typeof seconds === "string" && seconds.trim() !== "" ? Number(seconds) : seconds;
  if (Number.isFinite(n) && n > 0) parameters.duration = Math.round(n);
  const resolution = trimmed(metadata.resolution).toUpperCase();
  if (resolution) {
    if (!profile.resolutions.includes(resolution)) {
      throw new Error(model + " resolution must be one of " + profile.resolutions.join(", "));
    }
    parameters.resolution = resolution;
  }
  const ratio = trimmed(metadata.ratio) || trimmed(metadata.aspect_ratio);
  if (ratio) parameters.ratio = ratio;
  if (WAN_AUDIO_VIDEO_MODELS.includes(baseModel(model))) {
    const audio = metadata.generate_audio != null
      ? metadata.generate_audio
      : (trimmed(metadata.sound) ? trimmed(metadata.sound) === "on" : null);
    if (audio != null) parameters.audio = audio;
  }

  const vendorBody = { input: input };
  if (Object.keys(parameters).length) vendorBody.parameters = parameters;
  return { kind: "submit", model: model, requestBody: { model: model, vendorBody: vendorBody } };
}

// ── 计费 ──────────────────────────────────────────────────

export function estimateUsage(ctx) {
  const req = ctx.body || {};
  if (req._image) {
    const params = plainObject(plainObject(req.vendorBody).parameters);
    const n = Number(params.n);
    return { image_count: Number.isFinite(n) && n > 0 ? Math.min(n, 40) : 1 };
  }
  const model = ctx.model;
  const profile = WAN_MODELS[baseModel(model)];
  const params = plainObject(plainObject(req.vendorBody).parameters);
  let seconds = Number(params.duration);
  if (!Number.isFinite(seconds) || seconds <= 0) seconds = profile.maxDuration; // -1（自选）保留最长
  const base = baseModel(model);
  const facts = {
    seconds: Math.min(seconds, profile.maxDuration),
    resolution: profile.resolutions.includes(trimmed(params.resolution).toUpperCase()) ? trimmed(params.resolution).toUpperCase() : profile.defaultResolution,
  };
  if (WAN_AUDIO_VIDEO_MODELS.includes(base)) facts.audio = params.audio !== false;
  return facts;
}

// ── 提交链 ────────────────────────────────────────────────

export function buildSubmitRequest(ctx) {
  const req = ctx.body || {};
  const model = ctx.upstreamModel || ctx.model;
  if (req._image) {
    const vendor = plainObject(req.vendorBody);
    if (req._sync) {
      return { url: "/api/v1/services/aigc/multimodal-generation/generation", method: "POST", body: Object.assign({}, vendor, { model: model }) };
    }
    return {
      url: "/api/v1/services/aigc/text2image/image-synthesis",
      method: "POST",
      headers: { "X-DashScope-Async": "enable" },
      body: Object.assign({}, vendor, { model: model }),
    };
  }
  return {
    url: "/api/v1/services/aigc/" + videoService(model) + "/video-synthesis",
    method: "POST",
    headers: { "X-DashScope-Async": "enable" },
    body: Object.assign({}, plainObject(req.vendorBody), { model: model }),
  };
}

// 图像内容提取（同步 multimodal 的 choices 结构）
function imageContent(body) {
  const output = plainObject(plainObject(body).output);
  const choices = Array.isArray(output.choices) ? output.choices : [];
  const parts = [];
  for (const choice of choices) {
    const content = plainObject(choice.message).content;
    if (Array.isArray(content)) {
      for (const part of content) {
        if (part && typeof part === "object") parts.push({ text: trimmed(part.text), image: trimmed(part.image) });
      }
    }
  }
  return parts;
}
function videoURL(body) {
  const output = plainObject(plainObject(body).output);
  return trimmed(output.video_url) || trimmed(plainObject(output.results).video_url);
}

export function parseSubmitResponse(ctx) {
  const body = plainObject(ctx.responseBody);
  if (String(ctx.path || "").includes("multimodal")) {
    // 同步图像：immediate 终态
    const placeholder = "sync-" + utils.uuid().slice(0, 8);
    if (body.code) {
      return { taskId: placeholder, taskData: body, immediate: { status: "FAILURE", failReason: body.code + ": " + (body.message || "") } };
    }
    const content = imageContent(body);
    const images = content.filter(function (p) { return p.image; });
    if (!images.length) {
      return { taskId: placeholder, taskData: body, immediate: { status: "FAILURE", failReason: "image generation returned no images" } };
    }
    const im = { status: "SUCCESS", taskData: body, usage: { image_count: images.length } };
    if (images[0].image) im.artifacts = [{ key: "image-1", source: { type: "url", path: "_image_urls[0]" } }];
    if (images.length) {
      im.taskData = Object.assign({}, body, { _image_urls: images.map(function (p) { return p.image; }) });
    }
    return { taskId: placeholder, taskData: im.taskData, immediate: im };
  }
  if (body.code) throw new Error(body.code + ": " + (body.message || ""));
  if (!plainObject(body.output).task_id) throw new Error("task_id is empty");
  return { taskId: String(body.output.task_id), taskData: Object.assign({}, body, { _model: ctx.model || "" }) };
}

// ── 轮询链（视频 + 异步图像共用 /api/v1/tasks/{id}）────────────

export function buildQueryRequest(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  return { url: "/api/v1/tasks/" + encodeURIComponent(t.taskId), method: "GET" };
}

export function parseQueryResponse(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const body = plainObject(ctx.responseBody);
  const output = plainObject(body.output);
  const item = { taskId: t.taskId, taskData: body };

  if (output.task_status === "PENDING") return { items: [Object.assign(item, { done: false })] };
  if (output.task_status === "RUNNING") return { items: [Object.assign(item, { done: false, taskData: Object.assign({}, body, { _progress: "50%" }) })] };
  if (output.task_status === "SUCCEEDED") {
    const usage = plainObject(body.usage);
    const facts = {};
    const tdModel = plainObject(t.taskData);
    const model = baseModel(trimmed(ctx.model) || trimmed(tdModel._model) || trimmed(tdModel.model));
    if (model === "wan3.0-video" || model === "wan3.0-video-prime") {
      // wan3.0：计费时长 = 输入 + 输出（官方 usage 双段）
      const inputSeconds = Number(usage.input_video_duration);
      const outputSeconds = Number(usage.output_video_duration != null ? usage.output_video_duration : usage.duration);
      if (Number.isFinite(inputSeconds) && Number.isFinite(outputSeconds)) facts.seconds = inputSeconds + outputSeconds;
      else if (Number.isFinite(outputSeconds)) facts.seconds = outputSeconds;
    } else {
      const seconds = Number(usage.duration != null ? usage.duration : usage.output_video_duration);
      if (Number.isFinite(seconds) && seconds > 0) facts.seconds = seconds;
    }
    const resolution = trimmed(usage.SR || output.resolution).toUpperCase();
    if (["480P", "720P", "1080P"].includes(resolution)) facts.resolution = resolution;
    if (WAN_AUDIO_VIDEO_MODELS.includes(model) && typeof usage.audio === "boolean") facts.audio = usage.audio;

    const done = { taskId: t.taskId, done: true, usage: facts, taskData: body };
    // 产物：视频 URL / 异步图像 results[].url
    const vurl = videoURL(body);
    if (vurl) done.artifacts = [{ key: "video", mimeType: "video/mp4", source: { type: "url", path: "_video_url" } }];
    const results = Array.isArray(output.results) ? output.results : [];
    const imgURLs = results.map(function (r) { return trimmed(plainObject(r).url); }).filter(Boolean);
    if (imgURLs.length) {
      done.artifacts = (done.artifacts || []).concat([{ key: "image-1", source: { type: "url", path: "_image_urls[0]" } }]);
      facts.image_count = imgURLs.length;
    }
    if (vurl) done.taskData = Object.assign({}, body, { _video_url: vurl, _image_urls: imgURLs });
    else if (imgURLs.length) done.taskData = Object.assign({}, body, { _image_urls: imgURLs });
    return { items: [done] };
  }
  if (["FAILED", "CANCELED", "UNKNOWN"].includes(String(output.task_status || ""))) {
    let reason = trimmed(body.message);
    if (!reason && output.message) reason = "task failed, code: " + (output.code || "") + " , message: " + output.message;
    return { items: [Object.assign(item, { done: true, error: { code: "dashscope_" + output.task_status, message: reason || "task failed" } })] };
  }
  return { items: [Object.assign(item, { done: false })] };
}

// ── 渲染 ──────────────────────────────────────────────────

export function renderSubmit(ctx) {
  const task = ctx.task || {};
  const data = plainObject(task.data);
  const out = Object.assign({}, data);
  delete out._video_url;
  delete out._image_urls;
  delete out._model;
  if (plainObject(data.output).task_id) {
    return { status: 200, body: { output: { task_id: data.output.task_id, task_status: data.output.task_status || "PENDING" }, request_id: data.request_id || "" } };
  }
  return { status: 200, body: out };
}

export function renderImage(ctx) {
  const task = ctx.task || {};
  return { status: 200, body: plainObject(task.data) };
}

export function renderTask(ctx) {
  const task = ctx.task || {};
  const data = plainObject(task.data);
  const statusMap = { NOT_START: "PENDING", SUBMITTED: "PENDING", QUEUED: "PENDING", IN_PROGRESS: "RUNNING", SUCCESS: "SUCCEEDED", FAILURE: "FAILED" };
  const out = {
    output: {
      task_id: plainObject(data.output).task_id || task.taskId || "",
      task_status: statusMap[task.status] || "PENDING",
    },
    request_id: data.request_id || "",
  };
  if (data._model) out.model = data._model;
  if (plainObject(data.output).video_url) out.output.video_url = data.output.video_url;
  if (task.failReason) out.message = task.failReason;
  return { status: 200, body: out };
}
