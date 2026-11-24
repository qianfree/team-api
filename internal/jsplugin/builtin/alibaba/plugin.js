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
// ⑥ 1.7.0：契约升级——meta.models 为支持的上游模型（平台名由渠道映射决定，跨插件
//    可重名）；用量改报宿主固定词汇：output_seconds / input_video_seconds（wan3.0 官方
//    usage 双段分别上报，不再由插件相加——是否合并计价由宿主模型定价决定）/
//    resolution / generate_audio / output_count；按型号的校验（视频/图像类型、分辨率
//    档位）移到 validateRequest（ctx.upstreamModel），decode 只按路由与体形分流。
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
  version: "1.7.0",
  author: "qianfree",
  channelTypes: [79],
  models: Object.keys(WAN_MODELS)
    .concat(["wan2.7-t2v-2026-04-25", "wan2.7-t2v-2026-06-12", "wan2.7-i2v-2026-04-25"])
    .concat(SYNC_IMAGE_MODELS)
    .concat(ASYNC_IMAGE_MODELS),
  fetchMode: "per_task",
  standardVideos: "decodeVideos",
  routes: [
    { method: "POST", path: "/api/v1/services/aigc/video-generation/video-synthesis", type: "submit", decode: "createTask", render: "renderSubmit" },
    { method: "POST", path: "/api/v1/services/aigc/image2video/video-synthesis", type: "submit", decode: "createTask", render: "renderSubmit" },
    { method: "POST", path: "/api/v1/services/aigc/multimodal-generation/generation", type: "submit", decode: "createImage", render: "renderImage" },
    { method: "POST", path: "/api/v1/services/aigc/text2image/image-synthesis", type: "submit", decode: "createImage", render: "renderSubmit" },
    { method: "GET", path: "/api/v1/tasks/:task_id", type: "query", render: "renderTask" },
  ],
};

// ── 解码 ──────────────────────────────────────────────────

// 解码只按路由与体形分流，不按型号校验：body.model / ctx.model 是平台模型名（可能是
// 渠道映射别名），上游型号约束在 validateRequest（ctx.upstreamModel）校验。
export function createTask(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model);
  if (!model) throw new Error("model is required");
  return { kind: "submit", model: model, requestBody: { vendorBody: body } };
}

// createImage 同步（multimodal-generation）与异步（text2image）两条图像路由共用：
// 同步/异步由请求路由决定（_sync），validateRequest 再核对上游型号与路由一致
export function createImage(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model);
  if (!model) throw new Error("model is required");
  const params = plainObject(body.parameters);
  if (params.stream === true || params.enable_interleave === true) {
    throw new Error("stream/interleave is not supported; use the non-streaming API");
  }
  const sync = String(ctx.path || "").includes("multimodal");
  return { kind: "submit", model: model, requestBody: { vendorBody: body, _image: true, _sync: sync } };
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
  if (!trimmed(raw.prompt)) throw new Error("prompt is required");

  // 参考图字段形态（wan3.0 input.media / wan2.x input.img_url）依赖上游型号，
  // decode 先记在 _image，validateRequest 按 ctx.upstreamModel 落位
  const input = { prompt: String(raw.prompt) };
  const image = Array.isArray(raw.images) && raw.images.length && typeof raw.images[0] === "string" ? raw.images[0].trim() : trimmed(metadata.image);

  const parameters = {};
  const seconds = raw.seconds != null ? raw.seconds : metadata.duration;
  const n = typeof seconds === "string" && seconds.trim() !== "" ? Number(seconds) : seconds;
  if (Number.isFinite(n) && n > 0) parameters.duration = Math.round(n);
  const resolution = trimmed(metadata.resolution).toUpperCase();
  if (resolution) parameters.resolution = resolution;
  const ratio = trimmed(metadata.ratio) || trimmed(metadata.aspect_ratio);
  if (ratio) parameters.ratio = ratio;
  const audio = metadata.generate_audio != null
    ? metadata.generate_audio
    : (trimmed(metadata.sound) ? trimmed(metadata.sound) === "on" : null);

  const vendorBody = { input: input };
  if (Object.keys(parameters).length) vendorBody.parameters = parameters;
  const requestBody = { vendorBody: vendorBody, _std: true };
  if (image) requestBody._image_ref = image;
  if (audio != null) requestBody._audio = audio;
  return { kind: "submit", model: model, requestBody: requestBody };
}

// validateRequest 渠道选定后按上游型号校验并定稿：
//   - 图像路由：上游须为对应形态（同步 multimodal / 异步 text2image）的百炼图像模型
//   - 视频：上游须为万相视频模型，分辨率按型号档位校验；标准端点的参考图按型号
//     落位（wan3.x input.media / wan2.x input.img_url），音频开关仅 wan3.0 系透传
export function validateRequest(ctx) {
  const req = plainObject(ctx.body);
  const upstream = trimmed(ctx.upstreamModel || ctx.model);
  if (req._image) {
    if (req._sync && !isSyncImage(upstream)) throw new Error(upstream + " is not a Bailian synchronous image model (multimodal-generation)");
    if (!req._sync && !isAsyncImage(upstream)) throw new Error(upstream + " is not a Bailian asynchronous image model (text2image)");
    return {};
  }
  if (!isVideoModel(upstream)) throw new Error(upstream + " is not a Wan video model");
  const base = baseModel(upstream);
  const profile = WAN_MODELS[base];
  const vendor = plainObject(req.vendorBody);
  const params = plainObject(vendor.parameters);
  if (params.resolution !== undefined && !profile.resolutions.includes(trimmed(params.resolution).toUpperCase())) {
    throw new Error(upstream + " resolution must be one of " + profile.resolutions.join(", "));
  }
  if (!req._std) return { requestBody: Object.assign({}, req, { _upstream: upstream }) };
  // 标准端点：按型号定稿参考图与音频开关
  const input = Object.assign({}, plainObject(vendor.input));
  if (req._image_ref) {
    if (base.startsWith("wan3.")) input.media = [{ type: "image", url: req._image_ref }];
    else input.img_url = req._image_ref;
  }
  const parameters = Object.assign({}, params);
  if (WAN_AUDIO_VIDEO_MODELS.includes(base) && req._audio != null) parameters.audio = req._audio;
  const finalBody = { input: input };
  if (Object.keys(parameters).length) finalBody.parameters = parameters;
  return { requestBody: { vendorBody: finalBody, _upstream: upstream } };
}

// ── 计费 ──────────────────────────────────────────────────

export function estimateUsage(ctx) {
  const req = ctx.body || {};
  if (req._image) {
    const params = plainObject(plainObject(req.vendorBody).parameters);
    const n = Number(params.n);
    return { output_count: Number.isFinite(n) && n > 0 ? Math.min(n, 40) : 1 };
  }
  const model = trimmed(ctx.upstreamModel || ctx.model);
  const base = baseModel(model);
  const profile = WAN_MODELS[base] || WAN_MODELS["wan3.0-video"];
  const params = plainObject(plainObject(req.vendorBody).parameters);
  let seconds = Number(params.duration);
  if (!Number.isFinite(seconds) || seconds <= 0) seconds = profile.maxDuration; // -1（自选）保留最长
  const usage = {
    output_seconds: Math.min(seconds, profile.maxDuration),
    resolution: profile.resolutions.includes(trimmed(params.resolution).toUpperCase()) ? trimmed(params.resolution).toUpperCase() : profile.defaultResolution,
  };
  if (WAN_AUDIO_VIDEO_MODELS.includes(base)) usage.generate_audio = params.audio !== false;
  return usage;
}

// ── 提交链 ────────────────────────────────────────────────

export function buildSubmitRequest(ctx) {
  const req = ctx.body || {};
  const model = ctx.upstreamModel || ctx.model;
  if (req._image) {
    const vendor = plainObject(req.vendorBody);
    // 同步/异步取路由口径（_sync），不按平台模型名推断
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
    const im = { status: "SUCCESS", taskData: body, usage: { output_count: images.length } };
    if (images[0].image) im.artifacts = [{ key: "image-1", source: { type: "url", path: "_image_urls[0]" } }];
    if (images.length) {
      im.taskData = Object.assign({}, body, { _image_urls: images.map(function (p) { return p.image; }) });
    }
    return { taskId: placeholder, taskData: im.taskData, immediate: im };
  }
  if (body.code) throw new Error(body.code + ": " + (body.message || ""));
  if (!plainObject(body.output).task_id) throw new Error("task_id is empty");
  // 轮询期按上游型号分支（wan3.0 usage 双段）：记下上游模型名（平台名可能是映射别名）
  const upstream = trimmed(plainObject(ctx.body)._upstream) || trimmed(ctx.model);
  return { taskId: String(body.output.task_id), taskData: Object.assign({}, body, { _model: upstream }) };
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
    const model = baseModel(trimmed(tdModel._model) || trimmed(tdModel.model) || trimmed(ctx.model));
    if (model === "wan3.0-video" || model === "wan3.0-video-prime") {
      // wan3.0：官方 usage 双段（输出时长 + 输入参考视频时长）分别上报，合并计价归宿主定价
      const inputSeconds = Number(usage.input_video_duration);
      const outputSeconds = Number(usage.output_video_duration != null ? usage.output_video_duration : usage.duration);
      if (Number.isFinite(outputSeconds) && outputSeconds > 0) facts.output_seconds = outputSeconds;
      if (Number.isFinite(inputSeconds) && inputSeconds >= 0) facts.input_video_seconds = inputSeconds;
    } else {
      const seconds = Number(usage.duration != null ? usage.duration : usage.output_video_duration);
      if (Number.isFinite(seconds) && seconds > 0) facts.output_seconds = seconds;
    }
    const resolution = trimmed(usage.SR || output.resolution).toUpperCase();
    if (["480P", "720P", "1080P"].includes(resolution)) facts.resolution = resolution;
    if (WAN_AUDIO_VIDEO_MODELS.includes(model) && typeof usage.audio === "boolean") facts.generate_audio = usage.audio;

    const done = { taskId: t.taskId, done: true, usage: facts, taskData: body };
    // 产物：视频 URL / 异步图像 results[].url
    const vurl = videoURL(body);
    if (vurl) done.artifacts = [{ key: "video", mimeType: "video/mp4", source: { type: "url", path: "_video_url" } }];
    const results = Array.isArray(output.results) ? output.results : [];
    const imgURLs = results.map(function (r) { return trimmed(plainObject(r).url); }).filter(Boolean);
    if (imgURLs.length) {
      done.artifacts = (done.artifacts || []).concat([{ key: "image-1", source: { type: "url", path: "_image_urls[0]" } }]);
      facts.output_count = imgURLs.length;
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
