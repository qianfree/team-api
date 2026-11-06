// sora：自 new-api plugins/tasks/sora/plugin.js 移植（迁移批次 1/6）。
// OpenAI Sora 视频生成（文生视频/图生视频 URL/remix）。
//
// ── 迁移差异日志（vs new-api 原版）────────────────────────────
// ① meta：icon 改宿主图标名（video）；channelTypes 保留（宽容忽略）；
//    usageSchema map 形态/description 本地化对象直接沿用（宿主垫片）。
// ② routes：新增三条（原版无原生路由，走宿主协议）；remix 经路径参数。
// ③ decodeRequest：openai_video 协议解码的宿主契约形态（body 为原始 JSON）。
// ④ buildSubmitRequest：相对 URL + 去 Authorization（宿主注入）；
//    multipart parts 形态不支持（宿主未实现 bodyType multipart）——
//    图生视频走 input_reference URL 形态，文件上传形态暂缺。
// ⑤ parseSubmitResponse：双参→单参（ctx.responseBody）。
// ⑥ parseQueryResponse：包装 parseTaskResult 状态映射 + 终态 usage/artifacts；
//    产物为需渠道鉴权的相对路径 /v1/videos/{id}/content（source.auth=channel-bearer）。
// ⑦ estimateUsage：extractUsage 的宿主契约名。
// ⑧ renderSubmit/renderTask：openai_video 形态包装 {status, body}。
// ⑨ 删除：protocols 对象（SSE 认领）、listArtifacts/buildContentRequest
//    （宿主产物走声明式 items[].artifacts）。
// ─────────────────────────────────────────────────────────────
export const meta = {
  apiVersion: 1,
  key: "sora",
  name: "Sora",
  icon: "video",
  description: {
    en: "OpenAI Sora video generation (text-to-video, image-to-video, and remix)",
    zh: "OpenAI Sora 视频生成（文生视频、图生视频、remix）",
  },
  version: "1.1.0",
  channelTypes: [55, 1],
  author: "qianfree",
  models: ["sora-2", "sora-2-pro"],
  fetchMode: "per_task",
  usageSchema: {
    seconds: { type: "number", unit: "second", description: { en: "Video generation unit price", zh: "视频生成单价" } },
    size: {
      enum: ["720x1280", "1280x720", "1792x1024", "1024x1792"],
      description: { en: "Output video dimensions", zh: "输出视频尺寸" },
    },
  },
  routes: [
    { method: "POST", path: "/videos", type: "submit" },
    { method: "POST", path: "/videos/:task_id/remix", type: "submit" },
    { method: "GET", path: "/videos/:task_id", type: "query" },
  ],
};

function trimmed(value) {
  return String(value || "").trim();
}

// ── 提交链 ────────────────────────────────────────────────

export function decodeRequest(ctx) {
  const req = ctx.body && typeof ctx.body === "object" && !Array.isArray(ctx.body) ? ctx.body : {};
  const model = trimmed(req.model) || "sora-2";
  if (!["sora-2", "sora-2-pro"].includes(model)) {
    throw new Error("model must be sora-2 or sora-2-pro");
  }
  // remix：路径参数携带原任务 ID；普通提交要求 prompt
  const isRemix = Boolean((ctx.params || {}).task_id);
  if (!isRemix && !trimmed(req.prompt)) throw new Error("field prompt is required");
  const body = Object.assign({}, req, { model: model });
  if (isRemix) body.remix_of = String(ctx.params.task_id);
  return { kind: "submit", model: model, requestBody: body };
}

export function estimateUsage(ctx) {
  const req = ctx.body || {};
  let seconds = Number(req.seconds || req.duration || 4);
  if (!Number.isFinite(seconds) || seconds <= 0) seconds = 4;
  return { seconds: Math.min(seconds, 3600), size: req.size || "720x1280" };
}

export function buildSubmitRequest(ctx) {
  const req = ctx.body || {};
  const model = ctx.upstreamModel || ctx.model;
  const values = Object.assign({}, req, { model: model });
  delete values.remix_of;
  // remix：原任务 ID 从请求体 remix_of（decode 注入）恢复路径
  if (req.remix_of) {
    return { url: "/v1/videos/" + encodeURIComponent(String(req.remix_of)) + "/remix", method: "POST", body: values };
  }
  return { url: "/v1/videos", method: "POST", body: values };
}

export function parseSubmitResponse(ctx) {
  const body = ctx.responseBody || {};
  const taskId = body.id || body.task_id;
  if (!taskId) throw new Error("task_id is empty");
  return { taskId: String(taskId), taskData: body };
}

// ── 轮询链 ────────────────────────────────────────────────

export function buildQueryRequest(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  return { url: "/v1/videos/" + encodeURIComponent(t.taskId), method: "GET" };
}

const STATUS_MAP = {
  queued: "QUEUED",
  pending: "QUEUED",
  processing: "IN_PROGRESS",
  in_progress: "IN_PROGRESS",
  completed: "SUCCESS",
  failed: "FAILURE",
  cancelled: "FAILURE",
};

export function parseQueryResponse(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const body = ctx.responseBody || {};
  const mapped = STATUS_MAP[body.status];
  const item = { taskId: t.taskId, taskData: body };
  if (body.progress > 0 && body.progress < 100) {
    item.taskData = Object.assign({}, body, { _progress: body.progress + "%" });
  }
  if (!mapped) {
    item.done = false;
    return { items: [item] };
  }
  if (mapped === "SUCCESS") {
    // 终态用量（官方 seconds/size）+ 产物：内容在需鉴权的相对端点
    const usage = {};
    const seconds = Number(body.seconds || body.duration || 0);
    if (Number.isFinite(seconds) && seconds > 0) usage.seconds = Math.min(seconds, 3600);
    const size = trimmed(body.size);
    if (["720x1280", "1280x720", "1792x1024", "1024x1792"].includes(size)) usage.size = size;
    item.done = true;
    item.usage = usage;
    item.artifacts = [
      {
        key: "video",
        mimeType: "video/mp4",
        source: { type: "url", path: "_content_url", auth: "channel-bearer" },
      },
    ];
    // 相对端点写进 taskData（宿主按渠道 baseURL 解析并注入鉴权）
    item.taskData = Object.assign({}, body, {
      _content_url: "/v1/videos/" + encodeURIComponent(t.taskId) + "/content",
    });
    return { items: [item] };
  }
  if (mapped === "FAILURE") {
    item.done = true;
    item.error = {
      code: "sora_failed",
      message: (body.error && body.error.message) || "task failed",
    };
    return { items: [item] };
  }
  item.done = false;
  return { items: [item] };
}

// ── 渲染（openai_video 形态）──────────────────────────────

export function renderSubmit(ctx) {
  const task = ctx.task || {};
  const data = task.data || {};
  return {
    status: 200,
    body: {
      id: data.id || task.publicTaskId,
      object: "video",
      model: task.model,
      status: "queued",
      created_at: utils.now(),
    },
  };
}

export function renderTask(ctx) {
  const task = ctx.task || {};
  const data = task.data || {};
  const statuses = { NOT_START: "queued", SUBMITTED: "queued", QUEUED: "queued", IN_PROGRESS: "in_progress", SUCCESS: "completed", FAILURE: "failed" };
  const progress = Number(String(task.progress || data.progress || "0").replace("%", ""));
  const output = {
    id: data.id || task.publicTaskId,
    object: "video",
    model: task.model,
    status: statuses[task.status] || "unknown",
    progress: Number.isFinite(progress) ? progress : 0,
  };
  if (task.status === "FAILURE") {
    output.error = { code: "video_generation_failed", message: "The video generation task failed." };
  }
  return { status: 200, body: output };
}
