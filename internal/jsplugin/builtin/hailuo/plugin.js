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
// ⑪ 1.7.0：契约升级——meta.models 为支持的上游模型（平台名由渠道映射决定，跨插件
//    可重名）；用量改报宿主固定词汇（output_seconds/resolution/input_image_count/
//    input_video_seconds）；型号分支一律按 ctx.upstreamModel 判定，按型号的强校验
//    （H3 时长/分辨率档位）移到 validateRequest，decode 只按请求体形态分流。
// ⑩ 1.6.0：H3 家族（H3 / H3-Max / H3-Lite / H3-Highspeed）按型号识别与透传（此前仅认
//    MiniMax-H3 且强制改写型号）；分辨率/时长按型号档位校验；结算读官方 usage 字段
//    input_image_count / input_seconds（此前读错字段名，素材用量恒回退预扣口径）；
//    标准端点 references[]（role 词汇与 MiniMax content[].role 一致）按原序映射。
// ⑫ 1.8.0：扩展模型——meta.modelsFromConfig=extra_models，上游新型号经配置登记即并入
//    有效清单，无需更新插件；未收录型号默认按 H3/v2 形态处理（MiniMax 新型号均上 /v2），
//    不校验分辨率/时长档位（无法归一的原样透传），原生路由按客户端所选协议放行，
//    参数合法性由上游校验。

const H3_MODEL = "MiniMax-H3";
// H3 家族型号档位（官方 /v2/video_generation）：H3 为 768P/2K，其余 480P/768P；Max 时长 5–15
const H3_PROFILES = {
  "MiniMax-H3": { resolutions: ["768P", "2K"], minDuration: 4 },
  "MiniMax-H3-Max": { resolutions: ["480P", "768P"], minDuration: 5 },
  "MiniMax-H3-Lite": { resolutions: ["480P", "768P"], minDuration: 4, explicitRatioForReference: true },
  "MiniMax-H3-Highspeed": { resolutions: ["480P", "768P"], minDuration: 4 },
};
const H3_MODELS = Object.keys(H3_PROFILES);
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
  version: "2.0.0",
  author: "qianfree",
  channelTypes: [35],
  models: H3_MODELS.concat(V1_MODELS),
  fetchMode: "per_task",
  standardVideos: "decodeVideos",
  // 标准端点 references[] 可承接的角色（按型号的模型层约束在 decodeVideos 校验）
  referenceRoles: ["first_frame", "last_frame", "reference_image", "reference_video", "reference_audio"],
  // 扩展模型：上游新型号无需更新插件——配置登记即并入有效清单，配合渠道模型映射
  // 使用；未收录型号默认按 H3/v2 形态处理（不校验分辨率/时长档位），由上游校验
  modelsFromConfig: "extra_models",
  config: [
    {
      key: "extra_models", label: "扩展上游模型", type: "text", group: "模型",
      description: "每行一个 MiniMax 上游模型名（如新发布的海螺型号），保存后并入本插件支持的上游模型清单（渠道模型映射配合使用）；未收录型号默认走 /v2 协议且不校验分辨率/时长档位，参数合法性由上游校验",
    },
  ],
  routes: [
    { method: "POST", path: "/v1/video_generation", type: "submit", decode: "createTask", render: "renderSubmit" },
    { method: "POST", path: "/v2/video_generation", type: "submit", decode: "createH3Task", render: "renderSubmit" },
    { method: "GET", path: "/videos/:task_id", type: "query", render: "renderTask" },
  ],
};

function trimmed(v) { return String(v || "").trim(); }
function plainObject(v) { return v && typeof v === "object" && !Array.isArray(v) ? v : {}; }
function isH3(model) { return Object.prototype.hasOwnProperty.call(H3_PROFILES, model); }
function isV1(model) { return V1_MODELS.includes(trimmed(model)); }
// 扩展型号（不在内置两清单，经 extra_models 登记）默认按 H3/v2 形态处理——MiniMax
// 新型号均上 /v2；原生路由上客户端选定的协议形态优先（仅已知型号跨族映射报错）
function familyIsH3(model) { return isH3(model) || !isV1(model); }
function h3Profile(model) { return H3_PROFILES[model] || H3_PROFILES[H3_MODEL]; }
// h3ResolutionFor H3 家族分辨率归一（480P/768P/2K）；内置型号不在档位内即报错，
// 扩展型号不设档（无法归一时原样透传，由上游校验）；空 = 默认 768P
function h3ResolutionFor(raw, model) {
  const v = trimmed(raw).toUpperCase();
  if (!v) return "768P";
  let resolution = "";
  if (v.includes("2K") || v.includes("1440") || v.includes("2160")) resolution = "2K";
  else if (v.includes("768") || v.includes("720")) resolution = "768P";
  else if (v.includes("480")) resolution = "480P";
  if (!H3_PROFILES[model]) return resolution || v;
  const allowed = h3Profile(model).resolutions;
  if (!allowed.includes(resolution)) throw new Error(model + " resolution must be one of " + allowed.join(", "));
  return resolution;
}
// 时长档位（型号下限–15s）仅对内置 H3 型号校验；扩展型号由上游校验
function checkH3Duration(model, duration) {
  if (!H3_PROFILES[model]) return;
  if (duration === undefined || duration === null) return;
  const n = Number(duration);
  const min = h3Profile(model).minDuration;
  if (!Number.isFinite(n) || n < min || n > H3_MAX_DURATION) throw new Error(model + " duration must be between " + min + " and " + H3_MAX_DURATION);
}
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

// 解码只按请求体形态分流（/v1 → v1 协议、/v2 → H3 协议），不校验型号：body.model
// 是平台模型名（可能是渠道映射别名），上游型号在 validateRequest（ctx.upstreamModel）校验。
export function createTask(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model);
  if (!model) throw new Error("model is required");
  if (body.duration !== undefined && body.duration !== null && (!Number.isFinite(Number(body.duration)) || Number(body.duration) <= 0)) throw new Error("duration must be positive");
  return { kind: "submit", model: model, requestBody: { family: "v1", vendorBody: body } };
}

export function createH3Task(ctx) {
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(body.model) || H3_MODEL;
  return { kind: "submit", model: model, requestBody: { family: "h3", vendorBody: body } };
}

// ── 参考素材（标准端点 references[]）────────────────────────

const H3_MAX_REF_TOTAL = 12; // 官方：混合参考合计 ≤12
const H3_MAX_REF = { reference_image: 9, reference_video: 3, reference_audio: 3 };
const REF_ITEM_TYPES = {
  first_frame: "image_url", last_frame: "image_url", reference_image: "image_url",
  reference_video: "video_url", reference_audio: "audio_url",
};

function refURL(ref, i) {
  const url = trimmed(ref.url);
  if (!url) throw new Error("references[" + i + "] must provide url" + (trimmed(ref.file_id) ? " (file_id is only supported on /v1/videos)" : ""));
  return url;
}

// h3ReferenceContent references[] → H3 content 媒体项（role 词汇与官方一致，保持原序：
// 提示词 "reference image 1 / 图片1" 按同类素材顺序编号）。模型层约束：参考合计 ≤12、
// 图 ≤9 / 视频 ≤3 / 音频 ≤3；H3-Lite 参考模式须显式比例（不支持 adaptive）。
function h3ReferenceContent(model, refs, ratio) {
  const items = [];
  const counts = {};
  let referenceTotal = 0;
  for (let i = 0; i < refs.length; i++) {
    const ref = plainObject(refs[i]);
    const role = trimmed(ref.role);
    const type = REF_ITEM_TYPES[role];
    if (!type) throw new Error("references[" + i + "].role " + JSON.stringify(role) + " is not supported");
    const item = { type: type, role: role };
    item[type] = { url: refURL(ref, i) };
    items.push(item);
    counts[role] = (counts[role] || 0) + 1;
    if (H3_MAX_REF[role]) {
      referenceTotal++;
      if (counts[role] > H3_MAX_REF[role]) throw new Error("at most " + H3_MAX_REF[role] + " " + role);
    }
  }
  if (referenceTotal > H3_MAX_REF_TOTAL) throw new Error("at most " + H3_MAX_REF_TOTAL + " references in total");
  if (referenceTotal > 0 && h3Profile(model).explicitRatioForReference && (!ratio || ratio === "adaptive")) {
    throw new Error(model + " requires an explicit aspect_ratio (not adaptive) in reference mode");
  }
  return items;
}

// v1ReferenceFields references[] → Hailuo 2.x/01 扁平字段：首/尾帧 → first/last_frame_image；
// S2V-01（主体参考模型）的单张 reference_image → subject_reference；其余角色不支持。
function v1ReferenceFields(model, refs) {
  const out = {};
  for (let i = 0; i < refs.length; i++) {
    const ref = plainObject(refs[i]);
    const role = trimmed(ref.role);
    if (role === "first_frame") out.first_frame_image = refURL(ref, i);
    else if (role === "last_frame") out.last_frame_image = refURL(ref, i);
    else if (role === "reference_image" && model === "S2V-01" && !out.subject_reference) {
      out.subject_reference = [{ type: "character", image: [refURL(ref, i)] }];
    } else {
      const what = role === "reference_image" && model === "S2V-01" ? "more than one reference_image" : JSON.stringify(role);
      throw new Error("reference_not_supported: " + model + " does not support " + what);
    }
  }
  return out;
}

// decodeVideos 标准视频端点解码（meta.standardVideos 指向）：通用任务体 → 协议中立的
// std 中间体（时长 seconds 优先、回落 metadata.duration；分辨率 metadata.resolution/size；
// references[] 为唯一权威来源，缺席时回落 images[0]/metadata.image 首帧与
// metadata.last_frame 尾帧）。上游型号未知（ctx.model 是平台名），由 validateRequest
// 按 ctx.upstreamModel 映射为 H3 content[] 或 v1 扁平字段。
export function decodeVideos(ctx) {
  const raw = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const metadata = plainObject(raw.metadata);
  const model = trimmed(ctx.model || raw.model);
  if (!model) throw new Error("model is required");
  if (!trimmed(raw.prompt)) throw new Error("prompt is required");

  const seconds = raw.seconds != null ? raw.seconds : metadata.duration;
  const n = typeof seconds === "string" && seconds.trim() !== "" ? Number(seconds) : seconds;
  const std = {
    prompt: String(raw.prompt),
    duration: Number.isFinite(n) && n > 0 ? Math.round(n) : null,
    resolution: trimmed(metadata.resolution) || trimmed(metadata.size),
    ratio: trimmed(raw.ratio) || trimmed(metadata.ratio) || trimmed(metadata.aspect_ratio),
    references: Array.isArray(raw.references) && raw.references.length ? raw.references : null,
    image: Array.isArray(raw.images) && raw.images.length && typeof raw.images[0] === "string" ? raw.images[0].trim() : trimmed(metadata.image),
    lastFrame: trimmed(metadata.last_frame),
  };
  return { kind: "submit", model: model, requestBody: { family: "std", std: std } };
}

// stdToH3Vendor std 中间体 → H3 /v2 请求体（H3 家族独立档位 480P/768P/2K）。
function stdToH3Vendor(model, std) {
  checkH3Duration(model, std.duration);
  const resolution = h3ResolutionFor(std.resolution, model);
  const content = [{ type: "text", text: std.prompt }];
  if (std.references) {
    Array.prototype.push.apply(content, h3ReferenceContent(model, std.references, std.ratio));
  } else {
    if (std.image) content.push({ type: "image_url", image_url: { url: std.image }, role: "first_frame" });
    if (std.lastFrame) content.push({ type: "image_url", image_url: { url: std.lastFrame }, role: "last_frame" });
  }
  const vendorBody = { content: content, resolution: resolution };
  if (std.duration != null) vendorBody.duration = std.duration;
  if (std.ratio) vendorBody.ratio = std.ratio;
  return vendorBody;
}

// stdToV1Vendor std 中间体 → v1 /v1 请求体（分辨率经 resolutionFor 归一）。
function stdToV1Vendor(model, std) {
  const resolution = std.resolution ? resolutionFor(std.resolution, model) : defaultResolution(model);
  const vendorBody = { prompt: std.prompt, resolution: resolution };
  if (std.duration != null) vendorBody.duration = std.duration;
  if (std.references) {
    Object.assign(vendorBody, v1ReferenceFields(model, std.references));
  } else {
    if (std.image) vendorBody.first_frame_image = std.image;
    if (std.lastFrame) vendorBody.last_frame_image = std.lastFrame;
  }
  return vendorBody;
}

// validateRequest 渠道选定后按上游型号校验并定稿请求体：标准端点的 std 中间体在此
// 映射为对应协议体（扩展型号默认 H3/v2）；/v2 路由仅已知 v1 型号报错、/v1 路由仅已知
// H3 型号报错（扩展型号按客户端所选路由放行，由上游校验）。
export function validateRequest(ctx) {
  const req = plainObject(ctx.body);
  const upstream = trimmed(ctx.upstreamModel || ctx.model);
  const h3 = familyIsH3(upstream);
  if (req.family === "std") {
    const std = plainObject(req.std);
    const vendorBody = h3 ? stdToH3Vendor(upstream, std) : stdToV1Vendor(upstream, std);
    return { requestBody: { family: h3 ? "h3" : "v1", vendorBody: vendorBody } };
  }
  if (req.family === "h3" && isV1(upstream)) throw new Error("/v2/video_generation requires an H3 model (" + H3_MODELS.join(", ") + "), channel upstream model is " + upstream);
  if (req.family === "v1" && isH3(upstream)) throw new Error("/v1/video_generation does not support " + upstream + "; use /v2/video_generation");
  if (h3) checkH3Duration(upstream, plainObject(req.vendorBody).duration);
  return {};
}

// ── 提交链 ────────────────────────────────────────────────

// isH3Request 提交形态判定：validateRequest 定稿的 family 优先，回落上游型号
//（扩展型号默认 H3/v2 形态）
function isH3Request(req, model) {
  if (req.family === "h3") return true;
  if (req.family === "v1") return false;
  return familyIsH3(model);
}

export function estimateUsage(ctx) {
  const req = plainObject(ctx.body);
  const model = trimmed(ctx.upstreamModel || ctx.model);
  const vendor = plainObject(req.vendorBody);
  if (isH3Request(req, model)) {
    const content = Array.isArray(vendor.content) ? vendor.content : [];
    const duration = vendor.duration == null ? H3_DEFAULT_DURATION : Number(vendor.duration);
    return {
      output_seconds: duration,
      resolution: ["2K", "480P"].includes(trimmed(vendor.resolution).toUpperCase()) ? trimmed(vendor.resolution).toUpperCase() : "768P",
      input_image_count: content.filter(function (c) { return c && c.type === "image_url"; }).length,
      input_video_seconds: content.some(function (c) { return c && c.type === "video_url"; }) ? H3_MAX_INPUT_VIDEO_SECONDS : 0,
    };
  }
  return { output_seconds: outboundDuration(vendor), resolution: outboundResolution(vendor, model), input_image_count: 0, input_video_seconds: 0 };
}

export function buildSubmitRequest(ctx) {
  const req = plainObject(ctx.body);
  const model = ctx.upstreamModel || ctx.model;
  if (isH3Request(req, model)) {
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
  // 形态取归一请求体的 family（ctx.model 是平台名，可能是映射别名）
  const h3 = isH3Request(plainObject(ctx.body), ctx.model);
  const apiError = h3 ? h3APIError(body) : null;
  if (apiError) throw new Error(apiError.message);
  const base = body.base_resp;
  if (base) {
    if (base.status_code !== 0) throw new Error(base.status_msg || "hailuo submit failed");
  } else if (!h3) {
    throw new Error("hailuo submit failed");
  }
  if (!body.task_id) throw new Error("missing task_id");
  const taskData = Object.assign({}, body, { _isH3: h3 });
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
      if (resolution === "2K" || resolution === "768P" || resolution === "480P") usage.resolution = resolution;
      // 官方 VideoTaskUsage 字段：input_image_count（首帧+尾帧+参考图）、input_seconds（参考视频秒数）
      const u = plainObject(task.usage);
      if (Number.isFinite(Number(u.output_seconds)) && Number(u.output_seconds) > 0) usage.output_seconds = Number(u.output_seconds);
      if (Number.isFinite(Number(u.input_image_count)) && Number(u.input_image_count) > 0) usage.input_image_count = Number(u.input_image_count);
      if (Number.isFinite(Number(u.input_seconds)) && Number(u.input_seconds) > 0) usage.input_video_seconds = Number(u.input_seconds);
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
