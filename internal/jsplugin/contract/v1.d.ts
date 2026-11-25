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

/**
 * 宿主固定用量词汇（estimateUsage 预扣估算与终态 usage 上报的键集）。
 * 计价口径由宿主模型定价决定，插件只回答「用了多少」；未知键整体拒绝。
 * 上游消费/积分等成本信息不上报（与平台计费无关）。
 */
export interface HostUsage {
  /** 生成时长（秒）：per_second 秒数信号 + 公式 u() */
  output_seconds?: number;
  /** 输出分辨率（"720p"/"1280x720"/"4K" 等，宿主归一为 480P/720P/768P/1080P/2K/4K 矩阵档） */
  resolution?: string;
  /** 产出数量（图片张数/歌曲段数）：count 信号 + u() */
  output_count?: number;
  /** 输入图片数（含首尾帧） */
  input_image_count?: number;
  /** 输入参考视频时长（秒）。提交时未知按上限估算，终态按官方 usage 覆盖 */
  input_video_seconds?: number;
  /** 输入参考音频时长（秒） */
  input_audio_seconds?: number;
  /** 输入 token 数（按 token 计价的上游） */
  input_tokens?: number;
  /** 输出 token 数 */
  output_tokens?: number;
  /** 是否生成音频（公式 u() 取 1/0，可作定价维度） */
  generate_audio?: boolean;
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
  /**
   * 插件支持的上游模型名（至少一个）。平台模型名由渠道模型路由/映射决定，
   * 与插件无关；渠道映射出的上游名须在本清单（∪ modelsFromConfig 扩展）内。
   * 跨插件同名上游模型是正常形态（同一模型由多家供应商提供），归属由渠道
   * plugin_key 决定。
   */
  models: string[];
  /** 轮询模式，缺省 per_task */
  fetchMode?: FetchMode;
  /** 插件原生路由（挂在 /pjs/{key} 下），至少一条 */
  routes: Route[];
  /** utils.fetch 额外允许的 host[:port]（渠道 baseUrl host 始终允许） */
  allowedHosts?: string[];
  /** 插件配置项（管理端动态表单；utils.config.get 读取，加密字段已解密注入） */
  config?: ConfigField[];
  /**
   * 标准视频端点解码钩子名（可选）：声明后模型可经 /v1/videos 与
   * /v1/video/generations 提交——钩子把通用任务体映射为原生 requestBody，
   * 返回形态与 decodeRequest 相同（kind 恒为 submit）。缺省 = 仅原生路由入口。
   */
  standardVideos?: string;
  /**
   * 标准端点可承接的参考素材角色（references[].role），缺省仅 first_frame；
   * 宿主在调用解码钩子前拦截未声明的角色（不静默丢弃）。
   */
  referenceRoles?: string[];
  /**
   * 扩展上游模型承载配置键（可选）：该 config 键的值（换行/逗号分隔或 JSON 数组，
   * 去重后 ≤100 个）运行时并入支持清单（有效清单 = models ∪ 配置扩展）。
   * 上游新增模型在管理后台改配置即可。指向的键须在 config 中声明、类型为
   * text/string/json、不允许加密。
   */
  modelsFromConfig?: string;
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
  /**
   * responseType="json"（缺省）且响应头 Content-Type 含 json → 对象，否则 → string；
   * "text" → string；"bytes" → ArrayBuffer
   */
  body: any;
}

/** 宿主能力：utils（基座注入的全局对象） */
export interface Utils {
  /**
   * 同步阻塞调用（非 Promise）。url 支持相对路径（按渠道 baseUrl 拼接，与
   * build*Request 同语义；decode/render 等无渠道上下文的钩子内被拒）或绝对 URL
   * （host 须在白名单内：渠道 baseUrl host ∪ meta.allowedHosts）；
   * ≤4 次/钩子、15s/次、30s/批、1MiB/响应；不跟随重定向。
   * 鉴权：目标与渠道 baseUrl 同源（scheme + host 相同，相对路径天然同源）时由基座
   * 注入 "Bearer <渠道密钥>"；跨域请求不带任何鉴权；插件自设的 Authorization 一律被剔除
   * （与 build*Request 的可覆盖语义不同）。
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
  /** 客户端请求头的首值（Authorization / Cookie 已剔除） */
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
  /** 请求声明的平台模型名（可能是渠道映射别名，原样回显） */
  model: string;
}

/** decodeRequest 返回（submit 意图，必选） */
export interface DecodeResult {
  kind: "submit";
  /** 平台模型名，原样回显（不校验 meta.models——那是上游模型清单），非空即可 */
  model: string;
  /** 原生请求体，序列化后 ≤50MiB（容纳 base64 内嵌参考图） */
  requestBody?: any;
}

/** dynamic 路由的查询意图（kind=query 仅 dynamic 路由允许） */
export interface DecodeQueryResult {
  kind: "query";
  taskIds: string[];
}

/** validateRequest 入参（渠道选定后、预扣前，带上游模型名） */
export interface ValidateRequestContext {
  /** 归一后的请求体（decode 返回的 requestBody） */
  body: any;
  params?: Record<string, string>;
  /** 平台模型名 */
  model: string;
  /** 渠道映射后的上游模型名（宿主已校验 ∈ 插件有效清单 models ∪ modelsFromConfig） */
  upstreamModel: string;
  channelBaseUrl?: string;
}

/** validateRequest 返回：requestBody 非空时替换归一请求体（空对象/undefined = 原样放行） */
export interface ValidateRequestResult {
  requestBody?: any;
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
  /** 归一后的提交请求体（validateRequest 修正后的 requestBody）：响应不含 action 线索时（如标准端点入站）由此推断提交形态 */
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
    usage?: HostUsage;
    artifacts?: ArtifactDecl[];
    failReason?: string;
  };
  /** 上游快照，≤1MiB，每轮查询整体覆盖——跨轮存活数据放 state！ */
  taskData?: any;
  /** 省略 = 保持原值 */
  state?: any;
}

// ── 计费（可选钩子）──────────────────────────────────────

/** estimateUsage 入参（提交预扣的用量估算；抛错或返回非法用量会拒绝提交，不再回落默认价） */
export interface EstimateUsageContext {
  /** 归一后的请求体（validateRequest 修正后） */
  body: any;
  model: string;
  /** 渠道映射后的上游模型名（按上游型号分支计量时使用） */
  upstreamModel?: string;
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
  /** 终态官方用量（键 ∈ 宿主固定词汇 HostUsage；结算按官方计量多退少补） */
  usage?: HostUsage;
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
  /** 最新上游快照（taskData）。成功任务中 artifacts 声明的取值路径已被宿主替换为统一内容链接 */
  data?: any;
  /**
   * 主产物统一内容链接（平台签名直链，免鉴权可直接打开，有效期由平台配置）。
   * 成功且声明过产物时非空；产物是否转存 OSS 不影响链接形态。
   */
  contentUrl?: string;
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
  /**
   * 按上游型号校验/修正（可选）：渠道选定后、预扣前调用，ctx.upstreamModel 为
   * 渠道映射后的上游模型名——按型号的参数约束（档位/时长/素材数量）在此校验，
   * 可返回 {requestBody} 替换归一请求体。抛错 = 请求不可接受（400，未预扣）。
   */
  validateRequest?(ctx: ValidateRequestContext): ValidateRequestResult | void;
  buildSubmitRequest(ctx: BuildSubmitContext): BuildRequestResult;
  parseSubmitResponse(ctx: ParseSubmitContext): ParseSubmitResult;
  buildQueryRequest(ctx: BuildQueryContext): BuildRequestResult;
  parseQueryResponse(ctx: ParseQueryContext): QueryResult;
  /** 预扣用量估算（键 ∈ 宿主固定词汇 HostUsage） */
  estimateUsage?(ctx: EstimateUsageContext): HostUsage;
  renderSubmit?(ctx: RenderTaskContext): RenderResult;
  renderTask?(ctx: RenderTaskContext): RenderResult;
  /** dynamic 路由批量查询渲染（缺省钩子名） */
  renderTasks?(ctx: RenderTasksContext): RenderResult;
}
