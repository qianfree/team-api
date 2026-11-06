/**
 * team-api JS 任务插件契约 v1 —— TypeScript 类型定义（插件作者侧）。
 *
 * 用法：import type { Meta, DecodeResult, ... } from "./v1"（与本文件同目录）
 * IDE 全程补全；运行时由基座校验，未知顶层字段会被拒绝。
 * 事实源：internal/jsplugin/contract（Go 侧），修改需双侧同步。
 *
 * 实例与隔离（重要）：实例池跨租户复用，模块作用域在实例生命周期内常驻——
 * 禁止在模块作用域保存任何请求级/租户级数据（请求级状态走 state 透传容器）；
 * 顶层代码在 meta 求值与实例创建时执行且有超时，只做纯声明。
 * 详见 docs/plugin-api/v1.md §7。
 */

export type RouteType = "submit" | "query" | "dynamic";
export type FetchMode = "batch" | "per_task";

/** 本地化文案：字符串或 {zh, en, ...} 对象（zh 优先，回退 en） */
export type LocalizedString = string | { [lang: string]: string };

/** usageSchema 单位声明：插件管「用了多少」（estimateUsage / items[].usage 出事实），基座管「多少钱」（公式 u() 定价） */
export interface UsageField {
  /** 事实键，公式经 u("key") 引用；小写字母开头的字母/数字/下划线 */
  key: string;
  type: "number" | "enum" | "boolean";
  /** 仅 number 型可用：second=时长信号 / count=张数信号 / 空=仅 u() */
  unit?: "second" | "count";
  /** 定价页展示语义 */
  description?: string;
}

/** 插件配置项声明（管理端动态表单渲染，加密字段掩码存储） */
export interface ConfigField {
  key: string;
  label: string;
  type: "string" | "int" | "bool" | "select" | "json" | "text";
  default?: unknown;
  /** select 必带选项 */
  options?: string[];
  required?: boolean;
  description?: string;
  /** 敏感字段：AES-256-GCM 存储、界面掩码、空值=保持；不允许 default */
  encrypted?: boolean;
  /** 配置界面按组渲染小节 */
  group?: string;
  halfWidth?: boolean;
}

/** 插件清单：`export const meta` */
export interface Meta {
  apiVersion: 1;
  /** 小写字母开头，仅小写字母/数字/连字符，长度 2-30 */
  key: string;
  name: string;
  /** semver，如 1.0.0 */
  version: string;
  description?: LocalizedString;
  /** 兼容字符串与 {name, url?} 两种形态 */
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
  /** 计费用量单位声明（number 型键自动注册为公式 u() 变量） */
  usageSchema?: UsageField[];
  /** 插件配置项（管理端动态表单；utils.config.get 读取，加密字段已解密注入） */
  config?: ConfigField[];
  /**
   * 标准视频端点解码钩子名（可选）：声明后模型可经 /v1/videos 与
   * /v1/video/generations 提交——钩子把通用任务体映射为原生 requestBody，
   * 返回形态与 decodeRequest 相同（kind 恒为 submit）。缺省 = 仅原生路由入口。
   */
  standardVideos?: string;
  /**
   * 扩展模型承载配置键（可选）：该 config 键的值（换行/逗号分隔或 JSON 数组，
   * 去重后 ≤100 个）运行时并入服务清单（有效清单 = models ∪ 配置扩展）。
   * 上游新增模型在管理后台改配置即可；冲突在配置保存时预检。指向的键须在
   * config 中声明、类型为 text/string/json、不允许加密。
   */
  modelsFromConfig?: string;
  /**
   * 同名模型共享声明（可选）：与其他同样声明 sharedModels 的插件同名声明模型时
   * 预检放行——同名多插件经各自渠道服务（路由归属由渠道 plugin_key 决定）。
   * 任一方未声明时同名冲突仍拒绝。同名模型的定价公式须覆盖全部声明插件的
   * 计量键（u() 缺失取 0）。
   */
  sharedModels?: boolean;
  /** 展示图标："text" / "text:<label ≤4字符>" 文字头像，或图标名；不允许 data:/http 链接 */
  icon?: string;
}

export interface Route {
  method: "GET" | "POST" | "PUT" | "DELETE" | "PATCH";
  /** 原生路径，支持 :param 段，如 /submit/:action */
  path: string;
  type: RouteType;
  /** submit/dynamic 路由的 decodeRequest 钩子别名，缺省 decodeRequest */
  decode?: string;
  /** 渲染钩子名（submit→renderSubmit、query→renderTask），缺省走默认透传 */
  render?: string;
}

/** utils.fetch 返回（同步对象——阻塞至响应返回，不是 Promise） */
export interface FetchResponse {
  status: number;
  headers: Record<string, string>;
  /** responseType="json" 且响应为 JSON → 对象；"text" → string；"bytes" → ArrayBuffer */
  body: any;
}

/** 宿主能力：utils（基座注入的全局对象） */
export interface Utils {
  /**
   * 同步阻塞调用（非 Promise）。url 支持相对路径（按渠道 baseUrl 拼接，与
   * build*Request 同语义；decode/render 等无渠道上下文的钩子内被拒）或绝对 URL
   * （host 须在白名单内：渠道 baseUrl host ∪ meta.allowedHosts）；
   * ≤4 次/钩子、15s/次、30s/批、1MiB/响应；不跟随重定向。
   */
  fetch(request: {
    url: string;
    method?: "GET" | "HEAD" | "POST";
    headers?: Record<string, string>;
    body?: string | object;
    responseType?: "json" | "text" | "bytes";
  }): FetchResponse;
  log: {
    info(message: string, fields?: object): void;
    warn(message: string, fields?: object): void;
    error(message: string, fields?: object): void;
  };
  /** 读插件自己的配置（meta.config 声明、管理端配置、加密字段已解密；未声明键=undefined） */
  config: {
    get(key: string): any;
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

/**
 * standardVideos 声明的解码钩子入参（标准任务端点入口）：body 为任务框架通用任务体
 * （{model, prompt, seconds?, metadata.{duration,size,aspect_ratio,sound,image,...}}；
 * /v1/video/generations 为客户端原样 body），映射为原生 requestBody 后进入既有提交链。
 */
export interface StandardEntryContext {
  /** 入口标识：openai_videos（/v1/videos）| video_generations（/v1/video/generations） */
  surface: "openai_videos" | "video_generations";
  /** 标准端点路径 */
  path: string;
  /** 通用任务体 */
  body: any;
  /** 请求声明的模型名（已解析到本插件） */
  model: string;
}

/** decodeRequest 返回（submit 意图，必选） */
export interface DecodeResult {
  kind: "submit";
  /** 必须在 meta.models 中声明 */
  model: string;
  /** 原生请求体，序列化后 ≤50MiB（容纳 base64 内嵌参考图） */
  requestBody?: any;
}

/** dynamic 路由的查询意图（kind=query 仅 dynamic 路由允许） */
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
  /** 渠道密钥（如 Kling 的 accessKey|secretKey）：供签名类插件经 utils.crypto 计算鉴权头 */
  apiKey?: string;
  state?: any;
}

/** buildSubmitRequest / buildQueryRequest 返回（必选） */
export interface BuildRequestResult {
  /**
   * 相对路径（拼接到渠道 baseUrl）或绝对 URL。
   * 绝对 URL 的 host 必须在白名单内（渠道 baseUrl host ∪ meta.allowedHosts），
   * 否则拒绝；重定向目标同样受白名单约束（跳出即中止）。
   */
  url: string;
  method?: string;
  /**
   * 附加请求头。Authorization 允许声明：签名类厂商（Kling JWT 等）由插件经
   * utils.crypto 计算鉴权头后覆盖基座默认 Bearer；未声明时基座注入 Bearer 渠道密钥。
   */
  headers?: Record<string, string>;
  /** 请求体，序列化后 ≤50MiB（容纳 base64 内嵌参考图；可放文件占位符） */
  body?: any;
}

/** parseSubmitResponse 入参 */
export interface ParseSubmitContext {
  status: number;
  /** 提交路由原生路径与参数（识别 action 用） */
  path?: string;
  params?: Record<string, string>;
  model?: string;
  responseBody?: any;
  /** 归一后的提交请求体（decode 返回的 requestBody）：响应不含 action 线索时（如标准端点入站）由此推断提交形态 */
  body?: any;
}

/** 产物声明：只指路（taskData 内取值路径），字节不进 JS，宿主负责转存对象存储 */
export interface ArtifactDecl {
  key: string;
  /** inline 必填；url 可选覆盖存储内容类型 */
  mimeType?: string;
  source: {
    type: "url" | "inline";
    /** taskData 内的取值路径，如 "data.videos[0].url" */
    path: string;
    /** 需鉴权下载："channel-bearer"=以渠道密钥注入 Authorization */
    auth?: string;
  };
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
    artifacts?: ArtifactDecl[];
    failReason?: string;
  };
  /** 上游快照，≤1MiB，每轮查询整体覆盖——跨轮存活数据放 state！ */
  taskData?: any;
  /** 省略 = 保持原值 */
  state?: any;
}

// ── 计费事实（可选钩子）──────────────────────────────────

/** estimateUsage 入参（提交预扣的用量估算；失败回落默认价，不阻断提交） */
export interface EstimateUsageContext {
  body: any;
  model: string;
  params?: Record<string, string>;
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
  /** 渠道地址（供构造需鉴权的产物相对路径，配合 artifacts[].source.auth） */
  channelBaseUrl?: string;
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
  /** 终态用量事实（键 ∈ meta.usageSchema；结算按官方计量多退少补） */
  usage?: Record<string, number | string>;
  /** 终态产物声明（只指路，宿主转存） */
  artifacts?: ArtifactDecl[];
  /** 任务级失败（不炸整批）；done=true 且有 error → FAILURE */
  error?: TaskHookError;
}

/** parseQueryResponse 返回（必选） */
export interface QueryResult {
  items: QueryItem[];
}

// ── 渲染 ──────────────────────────────────────────────────

/** 渲染视图的任务字段（renderSubmit / renderTask / renderTasks 共用） */
export interface RenderTask {
  /** 平台任务 ID（task_xxx） */
  publicTaskId: string;
  /** 上游任务 ID */
  taskId: string;
  model: string;
  status: "SUBMITTED" | "IN_PROGRESS" | "SUCCESS" | "FAILURE";
  progress?: string;
  failReason?: string;
  /** 最新上游快照（taskData） */
  data?: any;
}

export interface RenderTaskContext {
  task: RenderTask;
}

/** dynamic 路由批量查询渲染入参（渲染钩子缺省名 renderTasks） */
export interface RenderTasksContext {
  tasks: RenderTask[];
}

/** renderSubmit / renderTask / renderTasks 返回（可选钩子） */
export interface RenderResult {
  status?: number;
  body: any;
}

// ── 钩子签名汇总（v1 固定钩子名；别名经 routes[].decode/render 指向任意导出函数）──

export interface TaskPlugin {
  meta: Meta;
  /** submit 意图；dynamic 路由可返回查询意图（DecodeQueryResult） */
  decodeRequest(ctx: DecodeContext): DecodeResult | DecodeQueryResult;
  /**
   * 标准端点解码（可选，钩子名由 meta.standardVideos 指向）：把 /v1/videos 与
   * /v1/video/generations 的通用任务体映射为原生 requestBody，返回形态与
   * decodeRequest 的 submit 意图相同。
   */
  decodeVideos?(ctx: StandardEntryContext): DecodeResult;
  buildSubmitRequest(ctx: BuildSubmitContext): BuildRequestResult;
  parseSubmitResponse(ctx: ParseSubmitContext): ParseSubmitResult;
  buildQueryRequest(ctx: BuildQueryContext): BuildRequestResult;
  parseQueryResponse(ctx: ParseQueryContext): QueryResult;
  /** 预扣用量估算（声明 usageSchema 的插件建议导出） */
  estimateUsage?(ctx: EstimateUsageContext): Record<string, number | string>;
  renderSubmit?(ctx: RenderTaskContext): RenderResult;
  renderTask?(ctx: RenderTaskContext): RenderResult;
  /** dynamic 路由批量查询渲染（缺省钩子名） */
  renderTasks?(ctx: RenderTasksContext): RenderResult;
}
