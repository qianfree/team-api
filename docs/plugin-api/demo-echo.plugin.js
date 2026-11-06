// demo-echo.plugin.js —— JS 任务插件示例（配合「上传 → 试运行 → 激活 → 启用」流程验证）
//
// 用途：
//   1. 管理后台上传本文件 → 点「试运行」，应看到六项检查全部通过；
//   2. 激活 + 启用后进入 generation（路由 /pjs/demo-echo/...），随时可停用/删除，不影响内置插件；
//   3. 想跑通完整提交链：建一个「JS 任务插件」类型渠道，baseUrl 填 https://httpbin.org
//      （免费回声服务），settings.plugin_key 填 demo-echo，把模型 demo_echo 绑到该渠道，
//      然后 POST /pjs/demo-echo/demo/submit/happy {"prompt":"hello world"} 即可端到端体验。
//
// 写法要点（对照 docs/plugin-api/v1.md）：
//   - 单文件、零 import，只 export；顶层不允许访问 utils；
//   - 5 个必选钩子收一个 ctx 对象、返回一个对象；
//   - 本示例演示 immediate（同步即完成）：提交响应直达终态、免轮询。
//     真实厂商插件通常不要用 immediate，让轮询链跟进上游状态。

export const meta = {
	apiVersion: 1,
	key: "demo-echo",
	name: "Demo Echo 示例插件",
	version: "1.0.0",
	description: "演示契约 v1 的最小可用插件：提交/查询/渲染/计费事实/配置项/immediate",
	author: "team-api",
	models: ["demo_echo"], // 模型名跨插件独占，激活时冲突预检
	fetchMode: "per_task",
	// utils.fetch 的 host 白名单（主提交请求走渠道 baseUrl，无需在此声明）
	allowedHosts: ["httpbin.org"],
	// 计费事实声明：number 型键进 u() 公式通道；unit=count 会映射为张数信号
	usageSchema: [
		{ key: "words", type: "number", unit: "count", description: "提示词词数（演示计费事实）" },
	],
	// 管理端会出现「插件配置」面板（sys_options 存储，改完即热生效）
	config: [
		{ key: "prefix", label: "输出前缀", type: "string", default: "[demo]", description: "渲染输出时拼接在前的前缀（演示配置项）" },
	],
	routes: [
		{ method: "POST", path: "/demo/submit/:style", type: "submit" }, // 客户端: POST /pjs/demo-echo/demo/submit/happy
		{ method: "GET", path: "/demo/status/:task_id", type: "query" }, // 客户端: GET /pjs/demo-echo/demo/status/task_xxx
	],
};

// ── 提交链 ────────────────────────────────────────────────

// decodeRequest 把原生请求归一为平台内部形态（判定模型 + 归一请求体）。
// 宽松校验（缺省补默认值），保证试运行的固定样例也能跑通。
export function decodeRequest(ctx) {
	const body = Object.assign({}, ctx.body || {});
	if (!body.prompt) body.prompt = "hello demo";
	// 路径参数在此进入契约：/demo/submit/:style 的 style 段
	body.style = String((ctx.params || {}).style || "default");
	return { kind: "submit", model: "demo_echo", requestBody: body };
}

// estimateUsage（可选）：预扣估算的计费事实，键必须 ∈ usageSchema。
export function estimateUsage(ctx) {
	const prompt = String((ctx.body || {}).prompt || "hello demo");
	const words = prompt.split(/\s+/).filter(Boolean).length || 1;
	return { words };
}

// buildSubmitRequest 声明上游请求（宿主代发）。相对 URL 提交时拼渠道 baseUrl。
export function buildSubmitRequest(ctx) {
	const body = ctx.body || {};
	return {
		url: "/anything/demo", // 指向 httpbin 的回声端点；换成真实厂商时写其原生路径
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: {
			prompt: body.prompt,
			style: body.style,
			model: ctx.upstreamModel || ctx.model,
		},
	};
}

// parseSubmitResponse 解析上游受理响应 → 平台任务句柄。
// 本示例宽松解析（任意 200 JSON 均可受理），并用 immediate 演示同步完成。
export function parseSubmitResponse(ctx) {
	if (ctx.status !== 200) throw new Error("demo upstream http " + ctx.status);
	const resp = ctx.responseBody || {};
	const echoed = resp.json || {}; // httpbin 回声会把请求体放在 json 字段
	const taskId = String(resp.id || echoed.id || "") || utils.uuid();
	return {
		taskId,
		taskData: resp, // 原始快照，查询/渲染时可取用
		state: { created: utils.now() },
		// ⚠️ 演示专用：immediate 让任务在提交响应里直达终态（免轮询）。
		// 真实厂商插件只有在上游「受理即完成」语义明确时才可用。
		immediate: {
			status: "SUCCESS",
			taskData: { demo: true, prompt_echo: echoed.prompt || null, style: echoed.style || null },
			usage: { words: String(echoed.prompt || "").split(/\s+/).filter(Boolean).length || 1 },
		},
	};
}

// ── 轮询链（immediate 任务不会走到，但契约要求必选钩子齐全）──

export function buildQueryRequest(ctx) {
	const t = (ctx.tasks || [])[0] || {};
	return { url: "/v1/demo/status?task_id=" + encodeURIComponent(t.taskId), method: "GET" };
}

export function parseQueryResponse(ctx) {
	const t = (ctx.tasks || [])[0] || {};
	return { items: [{ taskId: t.taskId, done: true, taskData: ctx.responseBody || {}, state: t.state }] };
}

// ── 渲染（面向客户端，可选；不导出则平台默认透传）─────────

export function renderSubmit(ctx) {
	const t = ctx.task || {};
	const prefix = utils.config.get("prefix") || "[demo]";
	return {
		status: 200,
		body: { code: "success", task_id: t.publicTaskId, status: t.status, message: prefix + " renderSubmit 自定义响应" },
	};
}

export function renderTask(ctx) {
	const t = ctx.task || {};
	const prefix = utils.config.get("prefix") || "[demo]";
	return {
		status: 200,
		body: { task_id: t.publicTaskId, status: t.status, progress: t.progress || "", message: prefix + " renderTask 查询响应", data: t.data || null },
	};
}
