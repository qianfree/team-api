package skills

import (
	"context"
	"strings"

	"github.com/qianfree/team-api/internal/jsplugin"
)

// JSPluginSkill JS 插件代码生成技能。
// 参考注入 = manifest/钩子契约摘要 + golden 模板全文（与 GET /js-plugins/template
// 同源，全量注入）；校验复用 jsplugin.DryRun（esbuild 编译 + goja 沙箱 probe +
// 检查项，不落库、不发真实 HTTP，本就是为不可信源码设计的检查）。
type JSPluginSkill struct{}

// Key 技能标识。
func (JSPluginSkill) Key() string { return KeyJSPlugin }

// Label 中文标签。
func (JSPluginSkill) Label() string { return "JS 插件" }

// Fence 围栏语言标记。
func (JSPluginSkill) Fence() string { return "js" }

// Reference 领域参考：契约摘要 + 完整 golden 模板。
func (JSPluginSkill) Reference() string {
	var b strings.Builder
	b.WriteString(`【技能：JS 插件代码生成】
为平台的「JS 任务插件」编写 plugin.js（ES Module，接入异步任务型上游：音乐/视频/图片生成等）。

一、文件结构：` + "`export const meta`" + ` 清单对象 + 各导出钩子函数。顶层不允许副作用与耗时计算（加载期在沙箱执行，超时即拒），
不允许 import 外部模块（esbuild bundle 编译，import 会编译失败）。

二、meta 必填/常用字段：
- apiVersion: 1（必须）
- key：小写字母开头，仅小写字母/数字/连字符，2-30 字符
- name / description / author / website（website 需 https）
- version：semver（如 1.0.0）
- models：接管的模型名数组（非空；模型跨插件独占，除非双方都声明 sharedModels）
- modelsFromConfig（可选）：扩展模型承载配置键名（该键须在 config 中声明，type 为 text/string/json，
  不可 encrypted）；其配置值（换行/逗号分隔或 JSON 数组）运行时并入模型清单，上游新增模型改配置即可
- fetchMode：batch | per_task（缺省 per_task）
- routes：[{method, path, type}]，method 大写，path 支持 :param 段，type ∈ submit | query | dynamic
  （dynamic 一型两用：解码结果定查询或提交）；可选 decode/render 字段声明钩子别名
- standardVideos："decodeVideos"（视频生成插件必须声明）：标准视频端点解码钩子名。声明后插件模型
  可经 POST /v1/videos（OpenAI Videos 协议）与 POST /v1/video/generations（通用任务端点）提交，
  查询 GET 同路径 /{id} 通用；未声明的插件这两个端点对其模型直接 400（只能走 /pjs 原生路由）
- referenceRoles：标准端点可承接的参考素材角色 ["first_frame","last_frame","reference_image",
  "reference_video","reference_audio","file","link"]（可选；缺省仅 first_frame，网关拦截未声明角色；
  file=文档文件参考、link=网页链接参考，仅上游支持时声明）。声明了多种角色的插件，decodeVideos 必须读取
  references[] 并按顺序映射
- usageSchema：计费用量声明 [{key, type: number|enum|boolean, unit: second|count|空, description}]，
  number 型键自动注册进计费公式 u() 注册表
- config：插件配置项 [{key, label, type: string|int|bool|select|json|text, default, options, required, encrypted, group}]
- allowedHosts：utils.fetch 额外允许的 host[:port] 白名单
- icon："text" / "text:<≤4字符>" / 图标名

三、钩子（按需导出，golden 模板有完整示范）：
- decodeRequest(ctx)：把客户端请求解码为 {kind: "submit"|"query", model, requestBody | taskIds}；入参 ctx={method, path, params, body, headers?, files?}；
  kind:"query" 仅 dynamic 路由可用；submit 的 model 必须 ∈ meta.models（或 modelsFromConfig 扩展清单）
- decodeVideos(ctx)（视频插件必须导出，名与 meta.standardVideos 一致）：标准视频端点解码。
  /v1/videos 与 /v1/video/generations 的请求体入钩子前均已被网关归一为通用任务体
  {model, prompt, seconds?, references?: [{role, url}], images?, metadata: {duration, size,
  resolution, aspect_ratio, sound, image, last_frame, ...}, extra?}——把它映射为插件原生
  requestBody，返回与 decodeRequest 同形态 {kind:"submit", model, requestBody}；references[]
  是参考素材唯一权威来源（存在时不再读 images/metadata.image 等兼容字段），时长读顶层
  seconds 或 metadata.duration。下游 estimateUsage/buildSubmitRequest/轮询链与原生路由入口完全复用。
  ⚠️ 模型名分工：ctx.model 是客户端请求的【平台模型名】，原样回显到返回值 model 即可，
  禁止对它做任何白名单/校验——它通常是 meta.models 里的名字，但渠道显式绑定本插件并配了
  模型映射时是平台侧别名（不在 meta.models，网关放行回显）；也禁止用上游模型名做校验
  （上游名只在 buildSubmitRequest 阶段经 ctx.upstreamModel 出现）。需要按型号分支参数时
  必须有默认分支兜住别名；同一插件兼有非视频模型（如音乐）时用排除法拒绝这些已知模型名，不要用白名单；提交形态（如视频/音乐）的判定写进 requestBody 的字段，不依赖模型名
- estimateUsage(ctx)：预扣口径的用量估算，返回 usageSchema 声明的键值对象
- buildSubmitRequest(ctx)：构造上游请求 {url, method, headers, body}；入参 ctx={body, params?, model,
  upstreamModel?, channelBaseUrl, apiKey?}；url 相对路径会拼接到渠道 baseUrl；
  上游模型名用 ctx.upstreamModel || ctx.model（渠道模型映射后的上游名，网关注入——
  平台名与上游名不同时必须用 upstreamModel 发给上游）。
  鉴权：build*Request 不声明 Authorization 头时网关自动注入 "Bearer <渠道密钥>"（提交与轮询同理），
  普通 Bearer 鉴权的上游不要声明该头、也不要为密钥另设 config 项；仅签名类厂商（JWT/HMAC）
  才用 ctx.apiKey 经 utils.crypto 计算后声明 Authorization 覆盖默认值
- parseSubmitResponse(ctx)：解析提交响应 {taskId, taskData?, state?}，taskId 必填；入参 ctx={status,
  responseBody, body, model?, path?, params?}（body 为 decode 返回的 requestBody）。
  网关只在上游返回 HTTP 200 时调用本钩子（非 200 由网关直接按上游错误处理），不要写 ctx.status 判断；
  taskData 每轮被查询结果整体覆盖，跨轮要保留的数据（如请求时长、提交形态）放 state——
  网关持久化 state 并在轮询时经 ctx.tasks[].state 回带（省略 = 保持原值）。同步即出结果的上游可额外返回
  immediate: {status: "SUCCESS"|"FAILURE", taskData?, usage?, artifacts?, failReason?}，任务免轮询直达终态
- buildQueryRequest(ctx)：构造轮询请求；入参 ctx.tasks=[{taskId, taskData?, state?}]（per_task 恒单元素，
  batch 为整批；taskId 是上游任务 ID，taskData 是上一轮快照，state 是上次返回的状态）
- parseQueryResponse(ctx)：解析轮询响应 {items: [{taskId, done, taskData?, state?, usage?, artifacts?, error?}]}；
  items[].taskId 必须在本次 ctx.tasks 内；done:true 且带 error:{code, message} = 失败。
  ctx.status 恒为 200（网关不透传轮询的 HTTP 状态码），不要按 404/5xx 写分支——终态一律按
  ctx.responseBody 的内容判定，无法识别的响应返回 done:false（网关的任务超时兜底）。
  终态 usage 优先读上游响应里的实际计量；上游不返回时才读 state 里保存的请求值
- usage（estimateUsage 返回值与终态 items[].usage）：键必须全部 ∈ meta.usageSchema，出现未声明键整体被拒；number 型取非负数值
- artifacts 产物声明：[{key, mimeType?, source: {type: "url"|"inline", path, auth?}}]，path 是 taskData 内的取值路径
  （如 "data.video_url"）；type="inline"（base64 字段）时 mimeType 必填；需渠道密钥下载的上游内容端点声明
  auth: "channel-bearer"。插件只指路不碰字节，由平台下载转存对象存储
- renderSubmit / renderTask：面向客户端的响应渲染 {status?, body}（可选，缺省透传）；入参 ctx.task 只有
  这些字段：publicTaskId（平台任务 ID）、taskId（上游任务 ID）、model、status、progress、failReason、
  data、contentUrl——没有 error 字段，失败原因读 task.failReason。status 是平台状态枚举
  SUBMITTED / IN_PROGRESS / SUCCESS / FAILURE（不是上游状态词）；要对外呈现 OpenAI/厂商状态词
  （queued / in_progress / completed / failed 等）必须在渲染钩子里显式映射。progress 是百分比
  字符串（如 "50%"，可能为空），需要数字时 parseInt。data 是最新上游快照，成功任务里 artifacts
  声明路径的值已替换为平台内容链接；contentUrl 为主产物链接。renderSubmit 只收到
  publicTaskId / model / status / contentUrl
- 通用：抛 Error = 插件级失败（转 4xx/502 透出给调用方）；返回对象出现契约外的顶层字段会被拒绝；
  requestBody/body ≤ 50MiB，taskData/state ≤ 1MiB

四、宿主 API（全局对象 utils，严格按以下签名调用）：
- utils.fetch({url, method?, headers?, body?, responseType?}) → {status, headers, body}：参数是【单个对象】，
  不是 (url, options)；同步阻塞；method 仅 GET/HEAD/POST；responseType 为 "json"(缺省)/"text"/"bytes"；
  每次钩子调用 ≤4 次、单次 15s、总 30s、响应 ≤1MiB；不跟随重定向；url 可为相对路径（拼渠道 baseUrl，
  decode/render 等无渠道上下文的钩子内不可用）或绝对 URL（host 须 ∈ 渠道 host ∪ meta.allowedHosts）；
  utils.fetch 的 Authorization 头由平台注入渠道密钥，插件设置无效（与 build*Request 的可覆盖语义不同）
- utils.log.info(message, fields?) / utils.log.warn(...) / utils.log.error(...)：utils.log 是对象，
  不能直接调用 utils.log(...)
- utils.config.get(key)：读 meta.config 声明的配置值（encrypted 字段已解密；未声明键返回 undefined）
- utils.now()：毫秒时间戳；utils.uuid()：UUID v4
- utils.crypto.hmacSHA256(message, secret) → hex；utils.crypto.jwtSignHS256(claims, secret)；
  utils.crypto.base64(s) / base64URL(s) / base64URLDecode(s)
- 钩子必须同步返回结果对象（不要 async/await、不要返回 Promise）；运行环境无 setTimeout、require、Buffer、
  全局 fetch 等 Node/浏览器 API，出网一律走 utils.fetch

五、生成硬规则：
- 必须基于下方 golden 模板的结构编写，meta 与钩子形状不得自创
- 【安全】插件实例跨租户复用：禁止在模块作用域（顶层 let/var/const、闭包缓存）保存任何请求级/租户级数据，
  否则会泄漏给其他租户的请求；顶层只放常量声明，请求级状态用 state 透传
- 视频生成类插件必须声明 meta.standardVideos 并导出对应解码钩子（golden 的 golden_video
  模型 + decodeVideos 是完整示范），保证模型可同时经 /v1/videos 与 /v1/video/generations 提交；
  仅音乐/图片等无视频语义的插件不声明
- 上游协议细节（端点路径、请求/响应字段）以用户提供的文档/示例为准，不要臆造；缺信息时先在围栏外列出问题清单再给代码
- 不要实现与任务无关的能力
- 文案语言：代码注释、utils.log 日志、meta.config 的 label/description 用中文；钩子抛出的错误消息会原样透出给
  API 调用方（OpenAI 兼容客户端），沿用 golden 模板的英文短句（如 "field prompt is required"）

六、golden 参考模板全文（结构范本：golden_music 音乐模型走原生路由，golden_video 视频模型示范 standardVideos 标准端点双入口）：
`)
	b.WriteString(jsplugin.TemplateSource())
	return b.String()
}

// Validate 复用 jsplugin.DryRun：与插件上传前自检、激活校验同一套检查。
func (JSPluginSkill) Validate(ctx context.Context, code string) *Report {
	src := strings.TrimSpace(code)
	if src == "" {
		return &Report{Skill: KeyJSPlugin, OK: false, Error: "插件源码为空"}
	}
	rep := jsplugin.DryRun(ctx, src)
	out := &Report{Skill: KeyJSPlugin, OK: !rep.HasFail()}
	for _, c := range rep.Checks {
		out.Checks = append(out.Checks, Check{Name: c.Name, Status: c.Status, Detail: c.Detail})
	}
	if m := rep.Manifest; m != nil && out.OK {
		out.Checks = append(out.Checks, Check{
			Name:   "manifest",
			Status: "pass",
			Detail: m.Key + "@" + m.Version + "（" + strings.Join(m.Models, ", ") + "）",
		})
	}
	return out
}
