// golden 插件：契约 v1 的参考实现，同时是引擎单测与 E2E 的夹具。
// 作者模板：抄这个文件起步，删掉不需要的钩子。
// Phase 2 起同时示范：usageSchema 计费事实、config 配置项、artifacts 产物声明。
//
// 上游协议（mock vendor）：
//   POST {base}/v1/golden/submit          → {code:"success", data:"<task_id>"}
//   GET  {base}/v1/golden/fetch?task_id=  → {status:"processing"|"success"|"failed", data:{audio_url}, clips, error?}
export const meta = {
  apiVersion: 1,
  key: "golden",
  name: "Golden 示例插件",
  version: "1.1.0",
  description: "契约 v1 参考实现：提交/查询/渲染/计费事实/产物声明/配置项",
  icon: "text",
  author: "qianfree",
  models: ["golden_music"],
  fetchMode: "per_task",
  usageSchema: [
    { key: "clips", type: "number", unit: "count", description: "生成歌曲段数（计费按段）" },
    { key: "tier", type: "enum", description: "生成档位（展示用，不参与计费）" },
  ],
  config: [
    { key: "fallback_url", label: "备用上游地址", type: "string", group: "上游", description: "主渠道失败时兜底" },
    { key: "api_key", label: "厂商密钥", type: "string", encrypted: true, group: "凭据" },
    { key: "max_clips", label: "单次上限", type: "int", default: 4, halfWidth: true },
    { key: "tier_mode", label: "档位", type: "select", options: ["standard", "pro"], default: "standard", halfWidth: true },
  ],
  routes: [
    { method: "POST", path: "/submit/:action", type: "submit" },
    { method: "GET", path: "/fetch/:task_id", type: "query" },
  ],
};

// ── 提交链 ────────────────────────────────────────────────

export function decodeRequest(ctx) {
  const action = String(((ctx.params || {}).action || "")).toUpperCase();
  if (action !== "MUSIC" && action !== "LYRICS") {
    throw new Error("invalid_action: " + action);
  }
  const body = Object.assign({}, ctx.body || {});
  body.action = action;
  return { kind: "submit", model: "golden_music", requestBody: body };
}

export function estimateUsage(ctx) {
  // 预扣口径：按请求段数估算（缺省 1），钳制到配置的单次上限
  const cap = Number(utils.config.get("max_clips") || 4);
  const clips = Math.min(Math.max(Number((ctx.body || {}).clips || 1), 1), cap);
  return { clips, tier: String(utils.config.get("tier_mode") || "standard") };
}

export function buildSubmitRequest(ctx) {
  const body = ctx.body || {};
  return {
    url: "/v1/golden/submit",
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: {
      prompt: body.prompt || "",
      action: body.action,
      clips: body.clips || 1,
      model: ctx.upstreamModel || ctx.model,
    },
  };
}

export function parseSubmitResponse(ctx) {
  if (ctx.status !== 200) throw new Error("submit_http_" + ctx.status);
  const resp = ctx.responseBody || {};
  if (resp.code !== "success" || !resp.data) throw new Error("submit_rejected");
  // state 跨轮存活：用轮询计数演示（第 2 轮起才允许终态）
  return { taskId: String(resp.data), taskData: resp, state: { polls: 0 } };
}

// ── 轮询链 ────────────────────────────────────────────────

export function buildQueryRequest(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  return {
    url: "/v1/golden/fetch?task_id=" + encodeURIComponent(t.taskId),
    method: "GET",
  };
}

export function parseQueryResponse(ctx) {
  const t = (ctx.tasks || [])[0] || {};
  const resp = ctx.responseBody || {};
  const state = Object.assign({}, t.state || {});
  state.polls = (state.polls || 0) + 1;

  if (resp.status === "success") {
    return {
      items: [{
        taskId: t.taskId,
        done: true,
        taskData: resp,
        usage: { clips: Number(resp.clips || 1), tier: String(resp.tier || "standard") },
        artifacts: [{ key: "audio", source: { type: "url", path: "data.audio_url" } }],
        state,
      }],
    };
  }
  if (resp.status === "failed") {
    return {
      items: [{
        taskId: t.taskId,
        done: true,
        error: { code: "golden_failed", message: resp.error || "upstream failed" },
        state,
      }],
    };
  }
  return { items: [{ taskId: t.taskId, done: false, taskData: resp, state }] };
}

// ── 渲染（面向客户端）─────────────────────────────────────

export function renderSubmit(ctx) {
  const task = ctx.task || {};
  return { status: 200, body: { code: "success", task_id: task.publicTaskId, status: task.status } };
}

export function renderTask(ctx) {
  const task = ctx.task || {};
  const data = (task.data || {}).data || {};
  return {
    status: 200,
    body: {
      task_id: task.publicTaskId,
      status: task.status,
      progress: task.progress || "",
      audio_url: data.audio_url || null,
    },
  };
}
