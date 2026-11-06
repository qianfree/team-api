// sunoapi：自 new-api plugins/tasks/sunoapi/plugin.js 移植（迁移验证样本）。
// 上游为 Suno-API 项目（https://github.com/Suno-API/Suno-API）。
//
// ── 迁移差异日志（vs new-api 原版，全文其余保持一致）──────────────────
// ① meta：channelTypes/protocols/upstreams 保留原样（宿主宽容忽略）；
//    description 本地化对象与 usageSchema map 形态直接沿用（宿主兼容垫片解析）；
//    仅 routes 路径去掉 /suno 前缀（宿主统一挂 /pjs/sunoapi 下）。
// ② buildSubmitRequest：ctx 形态适配（requestBody→body、去掉 requestHeaders/
//    apiKey/baseUrl——宿主注入鉴权与渠道地址）；去掉 Authorization 头与 action 字段。
// ③ parseSubmitResponse：双参 (ctx, resp) → 单参 ctx.responseBody。
// ④ buildQueryRequest：改用宿主契约 ctx.tasks（批量任务引用）。
// ⑤ 新增 parseQueryResponse 包装：把 new-api 的 parseBatchResult 条目映射为宿主
//    items 契约（done/usage/artifacts 终态映射）。
// ⑥ 新增 estimateUsage 包装（new-api extractUsage 的宿主契约名）。
// ⑦ 渲染钩子：native.renderSubmit/renderTask/renderTasks 包一层 {status, body}，
//    并以顶级导出暴露（宿主按路由别名解析顶级导出）。
// ⑧ 删除：protocols 对象（openai_responses SSE 渲染，宿主未实现该协议认领）。
//    listArtifacts/buildContentRequest 保留原样（宿主产物走 items[].artifacts 声明，
//    这两个导出在本宿主不被调用，保留以最小化差异）。
// ──────────────────────────────────────────────────────────────────────
export const meta = {
  apiVersion: 1,
  key: "sunoapi",
  name: "SunoAPI",
  icon: "music",
  description: {
    en: "SunoAPI project music and lyrics generation",
    zh: "SunoAPI 项目 音乐与歌词生成",
  },
  version: "1.1.0",
  author: "qianfree",
  website: "https://github.com/Suno-API/Suno-API",
  channelTypes: [36],
  models: ["suno_music", "suno_lyrics"],
  fetchMode: "batch",
  usageSchema: {
    clips: {
      type: "number",
      unit: "count",
      description: { en: "Song or lyrics generation unit price", zh: "生成歌曲或歌词单价" },
    },
    action: {
      enum: ["music", "lyrics"],
      enumLabels: { music: { en: "Generate songs", zh: "生成歌曲" }, lyrics: { en: "Generate lyrics", zh: "生成歌词" } },
      description: { en: "Generate songs or lyrics", zh: "生成歌曲或歌词" },
    },
  },
  routes: [
    { method: "POST", path: "/submit/:action", type: "submit", decode: "decodeSubmit", render: "renderSubmit" },
    { method: "POST", path: "/fetch", type: "dynamic", decode: "decodeBatch", render: "renderTasks" },
    { method: "GET", path: "/fetch/:task_id", type: "query", render: "renderTask" },
  ],
};

function trimmed(value) {
  return String(value || "").trim();
}

function actionName(ctx) {
  return String((ctx.params || {}).action || ctx.action || "").toUpperCase();
}

// ── 解码（new-api decodeNativeSubmit 的宿主契约形态：body 为原始 JSON）──
export function decodeSubmit(ctx) {
  if (!ctx.body || typeof ctx.body !== "object") throw new Error("JSON body required");
  const action = actionName(ctx);
  if (action !== "MUSIC" && action !== "LYRICS") throw new Error("invalid_action: " + action);
  return {
    kind: "submit",
    model: action === "MUSIC" ? "suno_music" : "suno_lyrics",
    requestBody: Object.assign({}, ctx.body, { action: action }),
  };
}

// dynamic 路由解码：POST /fetch 的 ids 列表 → 存量任务批量查询（new-api decodeNativeBatch）
export function decodeBatch(ctx) {
  const body = ctx.body || {};
  const ids = Array.isArray(body.ids) ? body.ids : [];
  return { kind: "query", taskIds: ids.map(String) };
}

// 预扣估算（new-api extractUsage 的宿主契约名；固定口径：MUSIC 2 段 / LYRICS 1 段）
export function estimateUsage(ctx) {
  const action = (actionName(ctx) || "MUSIC").toLowerCase();
  return { clips: action === "lyrics" ? 1 : 2, action: action };
}

// ── 提交链 ────────────────────────────────────────────────

function validateAndNormalize(ctx) {
  const body = Object.assign({}, ctx.body || {});
  if (body.make_instrumental === undefined) body.make_instrumental = false;
  const action = actionName(ctx);
  if (action === "MUSIC") {
    if (!body.mv) body.mv = "chirp-v3-0";
  } else if (action === "LYRICS") {
    if (!body.prompt) throw new Error("prompt_empty");
  } else {
    throw new Error("invalid_action");
  }
  delete body.action;
  return { action: action, body: body };
}

export function buildSubmitRequest(ctx) {
  const normalized = validateAndNormalize(ctx);
  return {
    url: "/suno/submit/" + normalized.action, // 相对路径：宿主拼接渠道 baseUrl 并注入鉴权
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: normalized.body,
  };
}

export function parseSubmitResponse(ctx) {
  const body = ctx.responseBody || {};
  if (body.code !== "success") throw new Error(String(body.message || ""));
  if (!body.data) throw new Error("task_id is empty");
  return { taskId: body.data, taskData: { message: String(body.message || "") } };
}

// ── 轮询链（批量）─────────────────────────────────────────

export function buildQueryRequest(ctx) {
  return {
    url: "/suno/fetch",
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: { ids: (ctx.tasks || []).map(function (t) { return t.taskId; }) },
  };
}

// parseBatchResult：new-api 原版逐条解析（保持不变），供 parseQueryResponse 包装
function parseBatchResult(body) {
  if (body.code !== "success") throw new Error(String(body.message || ""));
  return (body.data || []).map(function (item) {
    return {
      taskId: item.task_id || "",
      status: item.status || "",
      reason: item.fail_reason || "",
      data: item.data,
    };
  });
}

// 宿主契约：parseBatchResult 条目 → items（done/usage/artifacts 终态映射）
export function parseQueryResponse(ctx) {
  const resp = ctx.responseBody || {};
  const items = [];
  for (const entry of parseBatchResult(resp)) {
    const base = { taskId: entry.taskId, taskData: entry };
    if (entry.status === "success") {
      const songs = Array.isArray(entry.data) ? entry.data : [];
      const usage = {
        clips: songs.length || 1,
        action: songs.some(function (s) { return trimmed(s && s.audio_url); }) ? "music" : "lyrics",
      };
      const artifacts = [];
      (songs || []).forEach(function (song, i) {
        if (song && trimmed(song.audio_url)) {
          artifacts.push({ key: "audio-" + i, mimeType: "audio/mpeg", source: { type: "url", path: "data[" + i + "].audio_url" } });
        }
        if (song && trimmed(song.image_url)) {
          artifacts.push({ key: "cover-" + i, source: { type: "url", path: "data[" + i + "].image_url" } });
        }
      });
      items.push(Object.assign(base, { done: true, usage: usage, artifacts: artifacts }));
    } else if (entry.status === "failed") {
      items.push(Object.assign(base, { done: true, error: { code: "suno_failed", message: entry.reason || "upstream failed" } }));
    } else {
      items.push(Object.assign(base, { done: false }));
    }
  }
  return { items: items };
}

// ── 渲染（new-api native.render* 的宿主契约包装：{status, body}）────────

function nativeTask(t) {
  return {
    task_id: t.taskId || t.publicTaskId || "",
    status: t.status || "",
    fail_reason: t.failReason || "",
    progress: t.progress || "",
    data: t.data === undefined ? null : t.data,
  };
}

export function renderSubmit(ctx) {
  const task = ctx.task || {};
  const data = task.data && typeof task.data === "object" ? task.data : {};
  return {
    status: 200,
    body: { code: "success", message: String(data.message || ""), data: String(task.taskId || task.publicTaskId || "") },
  };
}

export function renderTask(ctx) {
  return { status: 200, body: { code: "success", message: "", data: nativeTask(ctx.task || {}) } };
}

export function renderTasks(ctx) {
  return {
    status: 200,
    body: { code: "success", message: "", data: (ctx.tasks || []).map(function (t) { return nativeTask(t); }) },
  };
}
