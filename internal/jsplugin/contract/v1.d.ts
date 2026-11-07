/**
 * team-api JS 任务插件契约 v1 —— TypeScript 类型定义（插件作者侧）。
 *
 * 用法：import type { Meta, DecodeResult, ... } from "./v1"（与本文件同目录）
 * IDE 全程补全；运行时由基座校验，未知顶层字段会被拒绝。
 * 事实源：internal/jsplugin/contract（Go 侧），修改需双侧同步。
 */

export type RouteType = "submit" | "query" | "dynamic";
export type FetchMode = "batch" | "per_task";

/** 插件清单：`export const meta` */
export interface Meta {
  apiVersion: 1;
  /** 小写字母开头，仅小写字母/数字/连字符，长度 2-30 */
  key: string;
  name: string;
  /** semver，如 1.0.0 */
  version: string;
  description?: string;
  author?: string | { name: string; url?: string };
  /** https 绝对地址（展示用） */
  website?: string;
  /** 接管的模型（平台标准名），至少一个 */
  models: string[];
  /** 轮询模式，缺省 per_task */
  fetchMode?: FetchMode;
  /** 插件原生路由（挂在 /pjs/{key} 下），至少一条 */
  routes: Route[];
  /** utils.fetch 额外允许的 host[:port]（渠道 baseUrl host 始终允许） */
  allowedHosts?: string[];
}

export interface Route {
  method: "GET" | "POST" | "PUT" | "DELETE" | "PATCH";
  /** 原生路径，支持 :param 段，如 /submit/:action */
  path: string;
  type: RouteType;
  /** submit 路由的 decodeRequest 钩子别名，缺省 decodeRequest */
  decode?: string;
  /** 渲染钩子名（submit→renderSubmit、query→renderTask），缺省走默认透传 */
  render?: string;
}

/** 宿主能力：utils（基座注入的全局对象） */
export interface Utils {
  fetch(request: {
    url: string;
    method?: "GET" | "HEAD" | "POST";
    headers?: Record<string, string>;
    body?: string | object;
    responseType?: "json" | "text" | "bytes";
  }): Promise<never> & { status: number; headers: Record<string, string>; body: any };
  // 注意：fetch 为同步调用（阻塞至响应返回）；此处类型示意返回值结构。
  log: {
    info(message: string, fields?: object): void;
    warn(message: string, fields?: object): void;
    error(message: string, fields?: object): void;
  };
  now(): number;
  uuid(): string;
  crypto: {
    /** HMAC-SHA256(message, secret) → hex 小写 */
    hmacSHA256(message: string, secret: string): string;
    /** 签发 HS256 JWT（header 固定 {alg:HS256,typ:JWT}；claims 原样写入，exp/nbf 由调用方给定） */
    jwtSignHS256(claims: Record<string, unknown>, secret: string): string;
    base64(value: string): string;
    base64URL(value: string): string;
    base64URLDecode(value: string): string;
  };
}

declare const utils: Utils;

// ── 提交链 ────────────────────────────────────────────────

/** decodeRequest 入参 */
export interface DecodeContext {
  path: string;
  method: string;
  params?: Record<string, string>;
  body?: any;
  headers?: Record<string, string>;
  /** multipart 上传的文件引用（字节不进 JS；body 内以占位符引用） */
  files?: Array<{ref: string; field: string; filename: string; mimeType: string; size: number}>;
}

/** buildSubmitRequest body 任意深度可放的文件占位符（宿主发送前内联编码） */
export interface FilePlaceholder {
  __fileRef: string;
  encoding: "base64" | "dataUrl";
  mimeType?: string;
  maxBytes?: number;
}

/** decodeRequest 返回（必选） */
export interface DecodeResult {
  kind: "submit";
  model: string;
  requestBody?: any;
}

/** dynamic 路由的查询意图（kind 仅在 dynamic 路由允许） */
export interface DecodeQueryResult {
  kind: "query";
  taskIds: string[];
}

/** buildSubmitRequest 入参 */
export interface BuildSubmitContext {
  body: any;
  params?: Record<string, string>;
  model: string;
  upstreamModel?: string;
  channelBaseUrl: string;
  state?: any;
}

/** buildSubmitRequest / buildQueryRequest 返回（必选） */
export interface BuildRequestResult {
  /** 相对路径（拼接到渠道 baseUrl）或绝对 URL（host 须在白名单内） */
  url: string;
  method?: string;
  /** Authorization 由基座管理，此处声明会被忽略 */
  headers?: Record<string, string>;
  body?: any;
}

/** parseSubmitResponse 入参 */
export interface ParseSubmitContext {
  status: number;
  responseBody?: any;
}

/** parseSubmitResponse 返回（必选） */
export interface ParseSubmitResult {
  /** 上游任务标识，必填 */
  taskId: string;
  /** 同步即完成：提交即终态（免轮询），结算/产物与轮询终态同口径 */
  immediate?: {
    status: "SUCCESS" | "FAILURE";
    taskData?: any;
    usage?: Record<string, number | string>;
    artifacts?: Array<{key: string; mimeType?: string; source: {type: "url" | "inline"; path: string}}>;
    failReason?: string;
  };
  /** 上游快照，≤1MiB，每轮查询整体覆盖——跨轮存活数据放 state！ */
  taskData?: any;
  /** 省略 = 保持原值 */
  state?: any;
}

// ── 轮询链 ────────────────────────────────────────────────

export interface QueryTaskRef {
  taskId: string;
  taskData?: any;
  state?: any;
}

/** buildQueryRequest 入参：batch=该插件全部未终态任务；per_task=单元素 */
export interface BuildQueryContext {
  tasks: QueryTaskRef[];
  model?: string;
}

/** parseQueryResponse 入参 */
export interface ParseQueryContext {
  responseBody?: any;
  status: number;
  tasks: QueryTaskRef[];
}

export interface TaskHookError {
  code: string;
  message: string;
}

export interface QueryItem {
  taskId: string;
  /** 终态判定 */
  done: boolean;
  taskData?: any;
  state?: any;
  /** 任务级失败（不炸整批）；done=true 且有 error → FAILURE */
  error?: TaskHookError;
}

/** parseQueryResponse 返回（必选） */
export interface QueryResult {
  items: QueryItem[];
}

// ── 渲染 ──────────────────────────────────────────────────

export interface RenderTaskContext {
  task: {
    publicTaskId: string;
    taskId: string;
    model: string;
    status: "SUBMITTED" | "IN_PROGRESS" | "SUCCESS" | "FAILURE";
    progress?: string;
    failReason?: string;
    data?: any;
  };
}

/** dynamic 路由批量查询渲染入参（渲染钩子缺省名 renderTasks） */
export interface RenderTasksContext {
  tasks: RenderTask[];
}

/** renderSubmit / renderTask 返回（可选钩子） */
export interface RenderResult {
  status?: number;
  body: any;
}

// ── 钩子签名汇总（v1 固定钩子名）─────────────────────────

export interface TaskPlugin {
  meta: Meta;
  decodeRequest(ctx: DecodeContext): DecodeResult;
  buildSubmitRequest(ctx: BuildSubmitContext): BuildRequestResult;
  parseSubmitResponse(ctx: ParseSubmitContext): ParseSubmitResult;
  buildQueryRequest(ctx: BuildQueryContext): BuildRequestResult;
  parseQueryResponse(ctx: ParseQueryContext): QueryResult;
  renderSubmit?(ctx: RenderTaskContext): RenderResult;
  renderTask?(ctx: RenderTaskContext): RenderResult;
}
