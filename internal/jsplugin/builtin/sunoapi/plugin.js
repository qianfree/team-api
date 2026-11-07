// sunoapi：SunoAPI 音乐与歌词生成插件（submit/:action + 批量查询）。
// 上游为 Suno-API 项目（https://github.com/Suno-API/Suno-API）。
//
// 上游协议：
//   POST {base}/suno/submit/{MUSIC|LYRICS} → {code:"success",data:"<task_id>"}
//   POST {base}/suno/fetch → {code,data:[{task_id,status,data}]}
//   GET  {base}/suno/fetch/{task_id} → {code,data:{task_id,status,data}}
//
// 实现要点：
// ① meta：usageSchema 为 map 形态（clips 计费段数 + action 枚举）；
//    channelTypes / description 的本地化对象由宿主垫片解析。
// ② routes：提交 `/submit/:action`、批量查询 `/fetch`（dynamic）、单任务查询
//    `/fetch/:task_id`；路径不带 /suno 前缀（宿主统一挂在 /pjs/sunoapi 下）。
// ③ buildSubmitRequest：用 ctx.body 取请求体，鉴权与渠道地址由宿主注入，
//    不自己拼 Authorization 与 action 字段。
// ④ parseSubmitResponse / parseQueryResponse / estimateUsage 均为单参形态，
//    上游响应取自 ctx.responseBody，批量查询的任务引用取自 ctx.tasks。
// ⑤ parseQueryResponse 把 parseBatchResult 的逐条结果映射为宿主 items 契约
//    （done / usage / artifacts 终态），estimateUsage 负责提交前预扣
//    （固定口径：MUSIC 2 段 / LYRICS 1 段）。
// ⑥ 渲染钩子 renderSubmit/renderTask/renderTasks 统一返回 {status, body}，
//    并以顶级导出暴露（宿主按路由别名解析）。
// ⑦ listArtifacts/buildContentRequest 宿主当前不调用——产物走 items[].artifacts
//    声明式配置，这两个导出仅保留以维持既有导出面。
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

// ── 解码（decodeSubmit：body 为原始 JSON，action 取自路径参数）──
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

// dynamic 路由解码：POST /fetch 的 ids 列表 → 存量任务批量查询
export function decodeBatch(ctx) {
  const body = ctx.body || {};
  const ids = Array.isArray(body.ids) ? body.ids : [];
  return { kind: "query", taskIds: ids.map(String) };
}

// 预扣估算（固定口径：MUSIC 2 段 / LYRICS 1 段）
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

// parseBatchResult：逐条解析上游批量查询结果，供 parseQueryResponse 映射
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

// ── 渲染（包装为 {status, body} 形态，宿主按路由别名调用）────────

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
