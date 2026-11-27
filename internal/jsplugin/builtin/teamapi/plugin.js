// teamapi 插件：Team-API ↔ Team-API 级联。
//
// 场景：上游是另一台 Team-API 实例（或任何 OpenAI Videos 兼容端点）时，经其标准
// 视频协议快速打通上下游，无需为每个上游模型单独写插件：
//   POST {base}/v1/videos              → {id:"video_<32hex>", object:"video", status:"queued", ...}
//   GET  {base}/v1/videos/{id}         → {id, status: queued|in_progress|completed|failed,
//                                        progress:0-100, video_url?, usage?, error?}
//   GET  {base}/v1/videos/{id}/content → 成品视频字节（官方 OpenAI 上游无直链时的唯一内容来源）
// 渠道 baseUrl 填上游站点地址；鉴权 Authorization: Bearer <上游 API Key>（渠道密钥，
// 提交与轮询均由宿主自动注入）。上游模型名任意（其 mdl 对外名），经配置项
// extra_models 登记——上游新增模型只需改配置，无需更新本插件。
//
// 实现要点：
// ① 双入口：标准端点 /v1/videos、/v1/video/generations（meta.standardVideos →
//    decodeVideos，宿主入钩前已归一为通用任务体、file_id 引用已解析为 URL）与原生
//    路由 /pjs/teamapi/v1/videos（OpenAI 形状 JSON 直传）。decode 只做体形归一，
//    不校验模型——平台名由渠道映射决定，参数与模型合法性由上游自己校验。
// ② 产物三档取源：video_url（Team-API 完成响应的直链扩展）→ video:{url}（官方
//    Sora 兼容形态）→ channel-bearer 回源 /v1/videos/{id}/content（宿主注入渠道
//    密钥，纯 OpenAI 上游也能用）。下游 GET /v1/videos/{id}/content 由平台基座从
//    登记产物供给（OSS 转存/回源代理），插件不另设 content 原生路由——render 钩子
//    只能输出 JSON 文本，无法流式二进制。
// ③ 用量只报宿主固定词汇（计价口径由宿主模型定价决定）：预扣按请求估算（秒数/
//    图片角色数/分辨率）；终态映射上游 usage（output_video_seconds→output_seconds、
//    prompt_tokens→input_tokens、completion_tokens→output_tokens 等）多退少补。
export const meta = {
  apiVersion: 1,
  key: "teamapi",
  name: "Team-API",
  icon: "cloud",
  description: {
    en: "Chain an upstream Team-API instance via OpenAI-compatible /v1/videos endpoints",
    zh: "级联上游 Team-API 实例（OpenAI 兼容 /v1/videos 协议），上下游快速对接",
  },
  version: "1.0.0",
  author: "qianfree",
  // 占位基线（与 grsai/hailuo 等插件同名模型合法，归属由渠道绑定决定）；
  // 真实上游模型清单经 extra_models 配置扩展
  models: ["MiniMax-H3"],
  fetchMode: "per_task",
  standardVideos: "decodeVideos",
  referenceRoles: ["first_frame", "last_frame", "reference_image", "reference_video", "reference_audio", "file", "link"],
  modelsFromConfig: "extra_models",
  config: [
    {
      key: "extra_models", label: "扩展上游模型", type: "text", group: "模型",
      description: "每行一个上游 Team-API 对外模型名，保存后并入本插件支持的上游模型清单（渠道模型路由/映射配合使用）",
    },
  ],
  routes: [
    { method: "POST", path: "/v1/videos", type: "submit" },
    { method: "GET", path: "/v1/videos/:task_id", type: "query" },
  ],
};

// ── 工具函数 ──────────────────────────────────────────────

function trimmed(v) { return String(v === undefined || v === null ? "" : v).trim(); }
function plainObject(v) { return v && typeof v === "object" && !Array.isArray(v) ? v : null; }

// 通用任务体（标准端点）与 OpenAI 形状（原生路由）→ 内部 std 体。
// 只做字段搬运与归一，不校验模型/参数——上游 /v1/videos 是同一协议，由它校验。
function normalizeRequest(raw) {
  const metadata = plainObject(raw.metadata) || {};
  const std = { prompt: trimmed(raw.prompt) };

  // 时长：顶层 seconds（字符串/数字两形态都有）优先，回落通用体的 metadata.duration
  const seconds = raw.seconds != null && trimmed(raw.seconds) !== "" ? raw.seconds : metadata.duration;
  if (seconds != null && trimmed(seconds) !== "") std.seconds = seconds;

  const size = trimmed(raw.size || metadata.size);
  if (size) std.size = size;

  const ratio = trimmed(raw.aspect_ratio || raw.ratio || metadata.aspect_ratio || metadata.ratio);
  if (ratio) std.aspect_ratio = ratio;

  // 分辨率档位（通用端点词汇）：/v1/videos 无对应顶层字段，仅供用量估算
  const resolution = trimmed(metadata.resolution);
  if (resolution) std.resolution = resolution;

  // 参考素材：references[] 唯一权威（标准端点入钩前 file_id 已解析为 URL）；
  // 缺席时按兼容字段组装首/尾帧（metadata.image / images[0] / metadata.last_frame）
  const refs = [];
  if (Array.isArray(raw.references)) {
    for (const ref of raw.references) {
      const role = plainObject(ref) ? trimmed(ref.role) : "";
      const url = plainObject(ref) ? trimmed(ref.url) : "";
      if (role && url) refs.push({ role: role, url: url });
    }
  }
  if (!refs.length) {
    const images = Array.isArray(raw.images) ? raw.images : [];
    const first = trimmed(metadata.image || (typeof images[0] === "string" ? images[0] : ""));
    if (first) refs.push({ role: "first_frame", url: first });
    const last = trimmed(metadata.last_frame);
    if (last) refs.push({ role: "last_frame", url: last });
  }
  if (refs.length) std.references = refs;

  // extra：通用体收敛词汇（metadata.sound/watermark）并入，客户端显式 extra 键优先。
  // 上游 /v1/videos 顶层 sound/generate_audio/watermark 已废弃（400），只能走 extra
  const extra = {};
  const sound = trimmed(metadata.sound);
  if (sound) extra.sound = sound;
  if (metadata.watermark !== undefined && metadata.watermark !== null) extra.watermark = !!metadata.watermark;
  Object.assign(extra, plainObject(raw.extra) || {});
  if (Object.keys(extra).length) std.extra = extra;

  return std;
}

// ── 提交链 ────────────────────────────────────────────────

// 标准视频端点解码（meta.standardVideos 指向）：/v1/videos 与 /v1/video/generations
// 的请求体入钩前均已被网关归一为通用任务体。ctx.model 是客户端请求的【平台模型名】，
// 原样回显不做白名单校验——上游名由渠道映射决定，在 buildSubmitRequest 阶段生效。
export function decodeVideos(ctx) {
  const raw = plainObject(ctx.body) || {};
  const model = trimmed(ctx.model || raw.model);
  if (!model) throw new Error("field model is required");
  return { kind: "submit", model: model, requestBody: normalizeRequest(raw) };
}

// 原生路由解码（POST /pjs/teamapi/v1/videos）：body 已是 OpenAI 形状。
// multipart 请走标准端点 /v1/videos（宿主会解析文件引用）——原生路由收到文件
// 引用无法可靠还原角色，明确拒绝而不是静默丢素材。
export function decodeRequest(ctx) {
  if (Array.isArray(ctx.files) && ctx.files.length) {
    throw new Error("multipart file uploads are not supported on the native route; submit via /v1/videos");
  }
  const raw = plainObject(ctx.body) || {};
  const model = trimmed(raw.model);
  if (!model) throw new Error("field model is required");
  return { kind: "submit", model: model, requestBody: normalizeRequest(raw) };
}

// estimateUsage 预扣用量估算：秒数/图片角色数可估则报，估不出（纯文生视频无时长
// 词汇）返回空对象 = 未上报，预扣回落模型级默认价。
export function estimateUsage(ctx) {
  const body = plainObject(ctx.body) || {};
  const usage = {};
  const seconds = Number(body.seconds);
  if (Number.isFinite(seconds) && seconds > 0) usage.output_seconds = seconds;
  let images = 0;
  if (Array.isArray(body.references)) {
    for (const ref of body.references) {
      const role = plainObject(ref) ? trimmed(ref.role) : "";
      if (role === "first_frame" || role === "last_frame" || role === "reference_image") images++;
    }
  }
  if (images > 0) usage.input_image_count = images;
  if (trimmed(body.resolution)) usage.resolution = trimmed(body.resolution);
  return usage;
}

export function buildSubmitRequest(ctx) {
  const body = plainObject(ctx.body) || {};
  const vendor = {
    // 上游模型名：ctx.upstreamModel 是渠道映射后的上游名（平台名 ≠ 上游名时必须用它）
    model: trimmed(ctx.upstreamModel) || trimmed(ctx.model),
    prompt: body.prompt || "",
  };
  if (body.seconds !== undefined && body.seconds !== null && trimmed(body.seconds) !== "") vendor.seconds = body.seconds;
  if (trimmed(body.size)) vendor.size = trimmed(body.size);
  if (trimmed(body.aspect_ratio)) vendor.aspect_ratio = trimmed(body.aspect_ratio);
  if (Array.isArray(body.references) && body.references.length) vendor.references = body.references;
  if (plainObject(body.extra)) vendor.extra = body.extra;
  // 鉴权：不声明 Authorization，宿主自动注入 "Bearer <渠道密钥>"（轮询同理）
  return {
    url: "/v1/videos",
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: vendor,
  };
}

// 宿主只在上游返回 HTTP 200 时调用本钩子（非 200 已按上游错误透出）；
// 200 + 错误信封（理论不该出现）在此兜底抛错。
export function parseSubmitResponse(ctx) {
  const resp = plainObject(ctx.responseBody) || {};
  const errObj = plainObject(resp.error);
  const id = trimmed(resp.id);
  if (!id) {
    throw new Error((errObj && trimmed(errObj.message)) || "upstream returned no video id");
  }
  return { taskId: id, taskData: { id: id, status: trimmed(resp.status) || "queued" } };
}

// ── 轮询链 ────────────────────────────────────────────────

export function buildQueryRequest(ctx) {
  const t = plainObject((ctx.tasks || [])[0]) || {};
  return {
    url: "/v1/videos/" + encodeURIComponent(trimmed(t.taskId)),
    method: "GET",
  };
}

// 官方形态产物直链：video:{url} 或 video:[{url},...]
function videoURLFrom(video) {
  if (plainObject(video)) return trimmed(video.url);
  if (Array.isArray(video)) {
    for (const v of video) {
      const url = plainObject(v) ? trimmed(v.url) : "";
      if (url) return url;
    }
  }
  return "";
}

// 终态产物三档取源（写入 taskData 快照并返回声明，宿主按转存配置落 OSS 或回源代理）：
// ① video_url（Team-API 扩展直链）② video:{url}（官方 Sora 兼容）③ channel-bearer
// 回源上游 content 端点（宿主注入渠道密钥，相对路径按渠道 baseUrl 解析）
function terminalArtifacts(resp, snapshot) {
  const direct = trimmed(resp.video_url) || videoURLFrom(resp.video);
  if (direct) {
    snapshot._video_url = direct;
    return [{ key: "video", mimeType: "video/mp4", source: { type: "url", path: "_video_url" } }];
  }
  snapshot._content_path = "/v1/videos/" + encodeURIComponent(trimmed(resp.id) || "unknown") + "/content";
  return [{ key: "video", mimeType: "video/mp4", source: { type: "url", path: "_content_path", auth: "channel-bearer" } }];
}

function putUsageNumber(usage, key, value) {
  if (value === undefined || value === null) return;
  const n = Number(value);
  if (Number.isFinite(n) && n >= 0) usage[key] = n;
}

// 上游 TaskUsageSummary → 宿主固定词汇（仅报存在且数值合法的键）
function mapUsage(raw) {
  const u = plainObject(raw);
  if (!u) return undefined;
  const usage = {};
  putUsageNumber(usage, "output_seconds", u.output_video_seconds);
  putUsageNumber(usage, "input_video_seconds", u.input_video_seconds);
  putUsageNumber(usage, "input_audio_seconds", u.input_audio_seconds);
  putUsageNumber(usage, "input_image_count", u.input_image_count);
  putUsageNumber(usage, "input_tokens", u.prompt_tokens);
  putUsageNumber(usage, "output_tokens", u.completion_tokens);
  const resolution = trimmed(u.resolution);
  if (resolution) usage.resolution = resolution;
  return Object.keys(usage).length ? usage : undefined;
}

// ctx.status 恒为 200（宿主不透传轮询 HTTP 状态码），终态一律按响应体内容判定；
// 无法识别的状态按进行中返回，由宿主的任务超时兜底。
export function parseQueryResponse(ctx) {
  const t = plainObject((ctx.tasks || [])[0]) || {};
  const resp = plainObject(ctx.responseBody) || {};
  const statusText = trimmed(resp.status);
  const status = statusText.toLowerCase();
  // taskData 每轮被整体覆盖：只存小快照 + 产物取源字段（_video_url/_content_path）
  const snapshot = { id: trimmed(resp.id) || t.taskId, status: statusText || "queued" };

  if (status === "completed") {
    const item = {
      taskId: t.taskId,
      done: true,
      taskData: snapshot,
      artifacts: terminalArtifacts(resp, snapshot),
    };
    const usage = mapUsage(resp.usage);
    if (usage) item.usage = usage;
    return { items: [item] };
  }
  if (status === "failed") {
    const errObj = plainObject(resp.error);
    return {
      items: [{
        taskId: t.taskId,
        done: true,
        taskData: snapshot,
        error: {
          code: (errObj && trimmed(errObj.code)) || "video_generation_failed",
          message: (errObj && trimmed(errObj.message)) || "upstream video generation failed",
        },
      }],
    };
  }
  // 进行中：上游 progress 为 0-100 整数（转存期固定 99），上报后宿主归一化透出
  const item = { taskId: t.taskId, done: false, taskData: snapshot };
  const progress = Number(resp.progress);
  if (Number.isFinite(progress) && progress >= 0) item.progress = Math.min(Math.round(progress), 100);
  return { items: [item] };
}

// ── 渲染（原生路由，面向客户端；标准端点的响应由平台基座渲染）────────

// 平台状态枚举 → OpenAI Videos 状态词
const OPENAI_STATUS = { SUBMITTED: "queued", IN_PROGRESS: "in_progress", SUCCESS: "completed", FAILURE: "failed" };

function openaiProgress(task, status) {
  const m = String(trimmed(task.progress)).match(/^(\d+(?:\.\d+)?)\s*%?$/);
  if (m) return Math.min(Math.round(Number(m[1])), 100);
  if (status === "completed") return 100;
  if (status === "in_progress") return 50;
  return 0;
}

// renderSubmit 只收到 publicTaskId / model / status / contentUrl
export function renderSubmit(ctx) {
  const task = plainObject(ctx.task) || {};
  return {
    status: 200,
    body: {
      id: trimmed(task.publicTaskId),
      object: "video",
      created_at: Math.floor(utils.now() / 1000),
      status: "queued",
      progress: 0,
      model: trimmed(task.model),
    },
  };
}

export function renderTask(ctx) {
  const task = plainObject(ctx.task) || {};
  const status = OPENAI_STATUS[trimmed(task.status)] || "queued";
  const body = {
    id: trimmed(task.publicTaskId),
    object: "video",
    status: status,
    progress: openaiProgress(task, status),
    model: trimmed(task.model),
  };
  if (status === "completed" && trimmed(task.contentUrl)) body.video_url = trimmed(task.contentUrl);
  if (status === "failed") {
    body.error = { code: "video_generation_failed", message: trimmed(task.failReason) || "video generation failed" };
  }
  return { status: 200, body: body };
}
