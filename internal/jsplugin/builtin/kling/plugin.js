// kling：自 new-api plugins/tasks/kling/plugin.js 移植（迁移批次 2/6）。
// 可灵视频生成：kling-v2.5/v2.6/v3（/v1/videos/* API）+ kling-4.0（/{endpoint}/kling-4.0 API）。
//
// ── 迁移差异日志（vs new-api 原版，833 行）──────────────────
// ① meta：icon 改宿主图标名（video）；usageProfiles 删除（宿主无此机制）；
//    usageSchema 用 map 形态声明 units（渠道密钥 accessKey|secretKey 在管理端渠道配置）。
// ② routes：五条提交（v2/v3 两条 + kling-4.0 三条）+ 一条统一查询
//    （v2/v3 原版按 action 分两条查询路由，此处合并——action 从 taskData._action 恢复）。
// ③ tokenFor：去 viaGateway 分支（宿主无 new_api 链式渠道）；JWT 经
//    utils.crypto.jwtSignHS256（宿主 P0 对齐能力），Authorization 覆盖基座默认。
// ④ decodeSubmit：v2/v3 原生形态（model_name 必填）；kling-4.0 路由按 ctx.path
//    分流 vendorBody 透传（decodeKling4Submit 的宿主契约形态）。
// ⑤ buildSubmitRequest：相对 URL + Authorization 由插件签名计算。
// ⑥ parseSubmitResponse：kling-4.0 的 state.resolution 改为 taskData._kling4 分标
//    （宿主 v1 未持久化插件 state，按响应形态识别——与原版渲染器同判定）。
// ⑦ parseQueryResponse：包装 parseTaskResult（两套 API 状态映射 + units 结算 +
//    产物 URL 声明）；v2/v3 查询 URL 的 action 从 taskData._action 恢复。
// ⑧ extractUsage/extractUsageOnComplete → estimateUsage / items[].usage。
// ⑨ renderSubmit/renderTask：native.taskCreated/taskStatus 形态包装。
// ⑩ 删除：protocols（SSE 认领）、listArtifacts/buildContentRequest（声明式产物替代）。
// ─────────────────────────────────────────────────────────────
export const meta = {
  apiVersion: 1,
  key: "kling",
  name: "Kling",
  icon: "video",
  description: {
    en: "Kling video generation (text/image-to-video, v2.5-v4.0)",
    zh: "可灵视频生成（文生视频/图生视频，v2.5-v4.0）",
  },
  version: "1.2.0",
  channelTypes: [50],
  author: "qianfree",
  models: ["kling-v2-5-turbo", "kling-v2-6", "kling-v3", "kling-4.0"],
  fetchMode: "per_task",
  usageSchema: {
    units: { type: "number", unit: "count", description: { en: "Resource units deducted", zh: "资源点数消耗（可灵计费单位）" } },
  },
  routes: [
    { method: "POST", path: "/videos/text2video", type: "submit" },
    { method: "POST", path: "/videos/image2video", type: "submit" },
    { method: "POST", path: "/text-to-video/kling-4.0", type: "submit" },
    { method: "POST", path: "/image-to-video/kling-4.0", type: "submit" },
    { method: "POST", path: "/omni-video/kling-4.0", type: "submit" },
    { method: "GET", path: "/videos/:task_id", type: "query" },
  ],
};

const KLING_MODELS = {
  "kling-v2-5-turbo": { units: { std: 0.3, pro: 0.5 } },
  "kling-v2-6": { units: { std: 0.3, pro: 0.5 }, soundUnits: { pro: 1.0 }, voiceUnits: { pro: 1.2 } },
  "kling-v3": { units: { std: 0.6, pro: 0.8, "4k": 3.0 }, soundUnits: { std: 0.9, pro: 1.2, "4k": 3.0 } },
};
const DEFAULT_KLING_MODEL = { units: { std: 0.9, pro: 1.2, "4k": 3.0 }, soundUnits: { std: 0.9, pro: 1.2, "4k": 3.0 }, voiceUnits: { std: 1.2, pro: 1.2, "4k": 3.0 } };
const KLING4_UNITS = { "720p": 1.0, "1080p": 1.5 };
const KLING4_VIDEO_INPUT_UNITS = { "720p": 0.6, "1080p": 0.9 };
const KLING4_MIN_SECONDS = 3;
const KLING4_MAX_SECONDS = 30;
const KLING4_VIDEO_INPUTS = ["feature_video", "base_video", "element"];
const UNSUPPORTED_FIELDS = ["callback_url", "external_task_id"];

function trimmed(v) { return String(v || "").trim(); }
function plainObject(v) { return v && typeof v === "object" && !Array.isArray(v) ? v : {}; }
function isKling4(model) { return model === "kling-4.0"; }

// ── 鉴权（宿主契约：ctx.apiKey 为渠道密钥；ak|sk → JWT 签名）──────
function tokenFor(apiKey) {
  const key = trimmed(apiKey);
  if (!key) throw new Error("api_key is empty");
  if (key.startsWith("sk-")) return key; // 网关密钥原样
  const parts = key.split("|");
  if (parts.length === 1) return key;
  if (parts.length !== 2) throw new Error("invalid api_key, required format is accessKey|secretKey");
  const now = Math.floor(utils.now() / 1000);
  return utils.crypto.jwtSignHS256({ iss: parts[0].trim(), exp: now + 1800, nbf: now - 5 }, parts[1].trim());
}
function requestHeaders(ctx) {
  return { Accept: "application/json", Authorization: "Bearer " + tokenFor(ctx.apiKey), "User-Agent": "kling-sdk/1.0" };
}

function filePlaceholder(image) {
  if (!image || typeof image !== "object" || Array.isArray(image) || !image.__fileRef) return image;
  const p = { __fileRef: image.__fileRef, encoding: image.encoding };
  if (image.mimeType) p.mimeType = image.mimeType;
  if (image.maxBytes != null) p.maxBytes = image.maxBytes;
  return p;
}

// ── 计费（units：模型表 × 模式 × 时长；kling-4.0 按分辨率×秒）────────
function klingModel(model) { return KLING_MODELS[model] || DEFAULT_KLING_MODEL; }
function klingMode(mode) { return trimmed(mode).toLowerCase() || "std"; }
function validateKlingDuration(value) {
  const n = typeof value === "string" && value.trim() !== "" ? Number(value) : value;
  if (!Number.isInteger(n) || n <= 0 || n > 3600) throw new Error("duration must be a positive integer of at most 3600");
  return n;
}
function kling4Duration(value) {
  const n = typeof value === "string" && value.trim() !== "" ? Number(value) : value;
  if (!Number.isInteger(n) || n < KLING4_MIN_SECONDS || n > KLING4_MAX_SECONDS)
    throw new Error("duration must be an integer from " + KLING4_MIN_SECONDS + " to " + KLING4_MAX_SECONDS);
  return n;
}
function outboundDuration(req) {
  const metadata = plainObject(req.metadata);
  let v = metadata.duration;
  if (v == null) v = req.duration;
  if (v == null) return 5;
  return validateKlingDuration(v);
}
function unitsPerSecond(model, mode, metadata) {
  const profile = klingModel(model);
  let table = profile.units;
  if (trimmed(metadata.sound).toLowerCase() === "on") {
    const voice = Array.isArray(metadata.voice_list) && metadata.voice_list.length > 0;
    table = (voice && profile.voiceUnits) || profile.soundUnits || (voice ? DEFAULT_KLING_MODEL.voiceUnits : DEFAULT_KLING_MODEL.soundUnits);
  }
  if (Object.prototype.hasOwnProperty.call(table, mode)) return table[mode];
  return Object.keys(table).reduce((d, k) => Math.max(d, table[k]), 0);
}
function kling4Resolution(settings) { return trimmed(plainObject(settings).resolution).toLowerCase() === "720p" ? "720p" : "1080p"; }
function checkKling4Body(body) {
  const options = plainObject(body.options);
  for (const name of UNSUPPORTED_FIELDS) {
    const v = options[name];
    if (v !== undefined && v !== null && v !== "") throw new Error("options." + name + " is not supported");
  }
  const settings = plainObject(body.settings);
  if (settings.duration != null) kling4Duration(settings.duration);
}

// ── 解码（按路由分流：v2/v3 原生 / kling-4.0 透传）──────────────────
export function decodeRequest(ctx) {
  const path = String(ctx.path || "");
  const body = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  if (path.startsWith("/text-to-video/") || path.startsWith("/image-to-video/") || path.startsWith("/omni-video/")) {
    checkKling4Body(body);
    return { kind: "submit", model: "kling-4.0", requestBody: { model: "kling-4.0", vendorBody: body } };
  }
  const model = typeof body.model_name === "string" ? body.model_name.trim() : trimmed(body.model);
  if (!model) throw new Error("model_name is required");
  for (const name of UNSUPPORTED_FIELDS) {
    if (body[name] !== undefined && body[name] !== null) throw new Error(name + " is not supported");
  }
  if (body.duration != null) validateKlingDuration(body.duration);
  return { kind: "submit", model: model, requestBody: { model: model, vendorBody: body } };
}

// ── 提交链 ────────────────────────────────────────────────

export function estimateUsage(ctx) {
  const req = ctx.body || {};
  if (req.vendorBody && isKling4(ctx.model)) {
    const settings = plainObject(plainObject(req.vendorBody).settings);
    const resolution = kling4Resolution(settings);
    const seconds = settings.duration == null ? KLING4_MAX_SECONDS : kling4Duration(settings.duration);
    const contents = Array.isArray(plainObject(req.vendorBody).contents) ? req.vendorBody.contents : [];
    const videoInput = contents.some(function (c) { return c && KLING4_VIDEO_INPUTS.includes(c.type); });
    return { units: videoInput ? KLING4_VIDEO_INPUT_UNITS[resolution] * Math.max(seconds * 2, KLING4_MAX_SECONDS + seconds) : KLING4_UNITS[resolution] * seconds };
  }
  const source = req.vendorBody ? { metadata: req.vendorBody } : req;
  return { units: unitsPerSecond(ctx.model, klingMode(source.mode || plainObject(source.metadata).mode), plainObject(source.metadata)) * outboundDuration(source) };
}

export function buildSubmitRequest(ctx) {
  const req = ctx.body || {};
  const model = ctx.upstreamModel || ctx.model;
  const isK4 = req.vendorBody && isKling4(model);
  if (isK4) {
    // endpoint 由 kling-4.0 请求体自带形态判定：contents 带 url 即图生视频类
    const vendor = plainObject(req.vendorBody);
    const endpoint = Array.isArray(vendor.contents) && vendor.contents.some(function (c) { return c && c.type !== "prompt"; })
      ? (vendor.contents.some(function (c) { return c && KLING4_VIDEO_INPUTS.includes(c.type); }) ? "omni-video" : "image-to-video")
      : "text-to-video";
    return {
      url: "/" + endpoint + "/" + model,
      method: "POST",
      headers: Object.assign({ "Content-Type": "application/json" }, requestHeaders(ctx)),
      body: vendor,
    };
  }
  const vendor = req.vendorBody || {};
  const action = String(ctx.params && (ctx.params.action || "") || (trimmed(vendor.image) || plainObject(vendor).image ? "image_to_video" : "text_to_video"));
  const effAction = action === "image_to_video" ? "image_to_video" : "text_to_video";
  const body = Object.assign({}, vendor, { model_name: model });
  if (body.image && typeof body.image === "object") body.image = filePlaceholder(body.image);
  if (body.image_tail && typeof body.image_tail === "object") body.image_tail = filePlaceholder(body.image_tail);
  return {
    url: effAction === "image_to_video" ? "/v1/videos/image2video" : "/v1/videos/text2video",
    method: "POST",
    headers: Object.assign({ "Content-Type": "application/json" }, requestHeaders(ctx)),
    body: Object.assign({}, body, { _action: undefined }),
    // _action 标记进不了 body（上游会拒）——查一下放哪：parseSubmitResponse 写入 taskData
  };
}

export function parseSubmitResponse(ctx) {
  const result = ctx.responseBody || {};
  if (result.code !== 0) throw new Error(result.message || "kling submit failed");
  const action = String(ctx.path || "").includes("image2video") ? "image_to_video" : "text_to_video";
  if (result.data && result.data.id && !result.data.task_id) {
    // kling-4.0 回包：data.id；标记形态供轮询判定（宿主 v1 不持久化插件 state）
    return { taskId: String(result.data.id), taskData: Object.assign({}, result, { _kling4: true }) };
  }
  if (!result.data || !result.data.task_id) throw new Error("missing task_id");
  return { taskId: String(result.data.task_id), taskData: Object.assign({}, result, { _action: action }) };
}

// ── 轮询链 ────────────────────────────────────────────────

export function buildQueryRequest(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const td = t.taskData && typeof t.taskData === "object" ? t.taskData : {};
  if (td._kling4) {
    return { url: "/tasks?task_ids=" + encodeURIComponent(t.taskId), method: "GET" };
  }
  const action = td._action === "image_to_video" ? "image2video" : "text2video";
  return { url: "/v1/videos/" + action + "/" + encodeURIComponent(t.taskId), method: "GET" };
}

// kling-4.0 回包条目（data 为数组或单对象）
function kling4Entry(reply, taskId) {
  const data = plainObject(reply).data;
  if (!Array.isArray(data)) return plainObject(data);
  return data.find(function (e) { return e && taskId && e.id === taskId; }) || plainObject(data[0]);
}
function kling4Videos(entry) {
  return (Array.isArray(entry.outputs) ? entry.outputs : []).filter(function (o) { return o && o.type === "video"; });
}
function reportedSeconds(v) {
  const n = Number.parseFloat(v);
  return Number.isFinite(n) && n > 0 ? Math.min(n, 3600) : 0;
}
// kling-4.0 结算 units：优先 billing 扣减，回退 outputs 时长推算
function kling4CompletedUnits(entry) {
  if (entry.status !== "succeeded") return null;
  const deductions = Array.isArray(entry.billing) ? entry.billing : [];
  const deducted = deductions.reduce(function (sum, item) {
    const type = item && item.charge_type;
    const units = Number.parseFloat(type === "unit" ? item.amount : type === "cash" ? item.list_price : "");
    return units >= 0 ? sum + units : Number.NaN;
  }, 0);
  if (deductions.length && Number.isFinite(deducted)) return deducted;
  const videos = kling4Videos(entry);
  const output = reportedSeconds(videos.length ? videos[0].duration : "");
  if (!output) return null;
  const input = reportedSeconds(plainObject(entry.usage).input_duration);
  const resolution = kling4Resolution(entry.settings);
  if (!input) return KLING4_UNITS[resolution] * output;
  return KLING4_VIDEO_INPUT_UNITS[resolution] * (input <= output ? output * 2 : input + output);
}

export function parseQueryResponse(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const body = ctx.responseBody || {};
  const td = t.taskData && typeof t.taskData === "object" ? t.taskData : {};
  const isK4 = Boolean(td._kling4);

  let status, failReason, videoURL, units, outData;
  if (isK4) {
    const entry = kling4Entry(body, t.taskId);
    const statuses = { submitted: "SUBMITTED", processing: "IN_PROGRESS", succeeded: "SUCCESS", failed: "FAILURE" };
    status = statuses[entry.status];
    failReason = entry.message || "";
    const videos = entry.status === "succeeded" ? kling4Videos(entry) : [];
    videoURL = videos.length ? trimmed(videos[0].url) : "";
    const u = kling4CompletedUnits(entry);
    if (u && u > 0) units = u;
    outData = body;
  } else {
    const data = body.data || {};
    const statuses = { submitted: "SUBMITTED", processing: "IN_PROGRESS", succeed: "SUCCESS", failed: "FAILURE" };
    status = statuses[data.task_status];
    failReason = data.task_status_msg || "";
    const videos = status === "SUCCESS" && data.task_result && data.task_result.videos ? data.task_result.videos : [];
    videoURL = videos.length ? trimmed(videos[0].url) : "";
    const u = Number.parseFloat(data.final_unit_deduction || "");
    if (Number.isFinite(u) && u > 0) units = u;
    outData = body;
  }

  const item = { taskId: t.taskId, taskData: outData };
  if (!status) return { items: [Object.assign(item, { done: false })] };
  if (status === "SUCCESS") {
    const done = { taskId: t.taskId, done: true, taskData: outData };
    if (units !== undefined) done.usage = { units: units };
    if (videoURL) done.artifacts = [{ key: "video", mimeType: "video/mp4", source: { type: "url", path: "_video_url" } }];
    done.taskData = Object.assign({}, outData, { _video_url: videoURL });
    return { items: [done] };
  }
  if (status === "FAILURE") {
    return { items: [Object.assign(item, { done: true, error: { code: "kling_failed", message: failReason || "task failed" } })] };
  }
  return { items: [Object.assign(item, { done: false })] };
}

// ── 渲染（native.taskCreated / taskStatus 形态）───────────────

export function renderSubmit(ctx) {
  const task = ctx.task || {};
  const data = task.data && typeof task.data === "object" ? task.data : {};
  const inner = plainObject(data.data);
  const out = Object.assign({}, data);
  delete out._kling4;
  delete out._action;
  if (inner.task_id) {
    return { status: 200, body: Object.assign(out, { data: Object.assign({}, inner, { task_id: task.taskId }) }) };
  }
  return { status: 200, body: Object.assign(out, { data: Object.assign({}, inner, { id: task.taskId }) }) };
}

export function renderTask(ctx) {
  const task = ctx.task || {};
  const data = task.data && typeof task.data === "object" ? task.data : {};
  const statusMap = { NOT_START: "submitted", SUBMITTED: "submitted", QUEUED: "submitted", IN_PROGRESS: "processing", SUCCESS: "succeed", FAILURE: "failed" };
  if (data._kling4) {
    const entry = kling4Entry(data, task.taskId);
    const k4Status = { NOT_START: "submitted", SUBMITTED: "submitted", QUEUED: "submitted", IN_PROGRESS: "processing", SUCCESS: "succeeded", FAILURE: "failed" };
    const out = Object.assign({}, data);
    delete out._kling4;
    return { status: 200, body: Object.assign({ code: 0, message: "", request_id: "" }, out, {
      data: [Object.assign({ status: k4Status[task.status] || "submitted" }, entry, { id: task.taskId })],
    }) };
  }
  const inner = Object.assign({}, plainObject(data.data), { task_id: task.taskId });
  if (task.status === "FAILURE" && inner.task_status !== "failed") {
    inner.task_status = "failed";
    inner.task_status_msg = task.failReason || "";
  }
  const out = Object.assign({}, data);
  delete out._action;
  return { status: 200, body: Object.assign(out, { data: inner }) };
}
