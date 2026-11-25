package skills

import (
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
- models：插件支持的【上游模型名】数组（非空）。平台模型名由渠道模型路由/映射决定，与插件无关；
  跨插件同名上游模型合法（同一模型由多家供应商提供，归属由渠道 plugin_key 决定），不做冲突预检。
  禁止声明 sharedModels / usageSchema（契约已移除，出现即编译失败）
- modelsFromConfig（可选）：扩展上游模型承载配置键名（该键须在 config 中声明，type 为 text/string/json，
  不可 encrypted）；其配置值（换行/逗号分隔或 JSON 数组）运行时并入支持清单，上游新增模型改配置即可
- fetchMode：batch | per_task（缺省 per_task）
- routes：[{method, path, type}]，method 大写，path 支持 :param 段，type ∈ submit | query | dynamic；
  可选 decode/render 字段声明钩子别名。三类路由的语义：
  submit=客户端提交入口（解码 → 选渠道 → 预扣 → 发上游）；
  query=客户端查询单个任务，【必须带路径参数】（如 /result/:task_id，值是平台公开任务 ID task_xxx），
  网关读库后调 renderTask 回包——不调上游、不调 buildQueryRequest；查询串（?id=）读不到，
  上游用查询串的厂商也声明成路径参数形式；
  dynamic=一型两用：解码返回 kind:"query"（taskIds 是平台公开任务 ID，≤100 个）时批量查询已有任务
  （渲染钩子缺省 renderTasks），返回 kind:"submit" 时走提交。
  任务进度的轮询由网关后台定时调 buildQueryRequest → parseQueryResponse 完成，与 query 路由无关
- standardVideos："decodeVideos"（视频生成插件必须声明）：标准视频端点解码钩子名。声明后插件模型
  可经 POST /v1/videos（OpenAI Videos 协议）与 POST /v1/video/generations（通用任务端点）提交，
  查询 GET 同路径 /{id} 通用；未声明的插件这两个端点对其模型直接 400（只能走 /pjs 原生路由）
- referenceRoles：标准端点可承接的参考素材角色 ["first_frame","last_frame","reference_image",
  "reference_video","reference_audio","file","link"]（可选；缺省仅 first_frame，网关拦截未声明角色；
  file=文档文件参考、link=网页链接参考，仅上游支持时声明）。声明了多种角色的插件，decodeVideos 必须读取
  references[] 并按顺序映射
- config：插件配置项 [{key, label, type: string|int|bool|select|json|text, default, options, required, encrypted, group}]
  （不可声明宿主保留键 artifact_storage——产物是否转存平台对象存储，由网关注入）
- allowedHosts：utils.fetch 额外允许的 host[:port] 白名单
- icon："text" / "text:<≤4字符>" / 图标名

三、钩子（按需导出，golden 模板有完整示范）：
- decodeRequest(ctx)：把客户端请求解码为 {kind: "submit"|"query", model, requestBody | taskIds}；入参
  ctx={method, path, params, body, headers, files}：headers 是客户端请求头（首值；Authorization / Cookie
  已剔除），files 是 multipart 上传文件的引用（见下条）；
  kind:"query" 仅 dynamic 路由可用；submit 的 model 是【平台模型名】（可能是渠道映射别名），
  原样回显、非空即可——不要校验 meta.models（那是上游模型清单）。解码阶段不知道上游型号
  （渠道尚未选定），按型号的参数约束放 validateRequest
- 文件上传（multipart/form-data）：字段 body 为 JSON 请求体原文、其余文本字段并入 body 顶层，文件内容
  不进 JS——decode 的 ctx.files 收到 [{ref, field, filename, mimeType, size}]（ref 形如
  request_file:image / request_file:images#1）。插件不承接文件时必须在 decode 里显式抛错拒绝
  （ctx.files 非空即报错并指引改传 URL / base64）——否则上传的素材被静默丢弃（如图生视频退化成
  文生视频仍按请求计费）。需要文件内容时在 buildSubmitRequest 的 body 任意深度放占位符
  {__fileRef: ref, encoding: "base64"|"dataUrl", mimeType?, maxBytes?}，宿主发送前内联为编码字符串
  （单文件与内联总量 ≤64MB）
- decodeVideos(ctx)（视频插件必须导出，名与 meta.standardVideos 一致）：标准视频端点解码。
  /v1/videos 与 /v1/video/generations 的请求体入钩子前均已被网关归一为通用任务体
  {model, prompt, seconds?, references?: [{role, url}], images?, metadata: {duration, size,
  resolution, aspect_ratio, sound, image, last_frame, ...}, extra?}——把它映射为插件原生
  requestBody，返回与 decodeRequest 同形态 {kind:"submit", model, requestBody}；references[]
  是参考素材唯一权威来源（存在时不再读 images/metadata.image 等兼容字段），时长读顶层
  seconds 或 metadata.duration。下游 validateRequest/estimateUsage/buildSubmitRequest/轮询链与原生路由入口完全复用。
  ⚠️ 模型名分工：ctx.model 是客户端请求的【平台模型名】，原样回显到返回值 model 即可，
  禁止对它做任何白名单/校验（平台名由渠道模型映射决定，可为任意别名）；也禁止用上游模型名
  做校验。上游型号在 validateRequest 阶段经 ctx.upstreamModel 出现；同一插件兼有非视频模型
  （如音乐）时，用排除法在 validateRequest 里按上游名拒绝，decode 只做与型号无关的体形归一
- validateRequest(ctx)（建议导出）：渠道选定后、预扣前调用，入参 ctx={body, params?, model,
  upstreamModel, channelBaseUrl?}——upstreamModel 是渠道映射后的上游模型名（网关已校验它 ∈
  meta.models ∪ modelsFromConfig 扩展）。按上游型号的参数约束（时长/分辨率档位/素材数量/
  模型与端点匹配）在此校验并抛错（= 请求不可接受 400，未预扣）；需要时可返回
  {requestBody} 替换归一请求体（decode 出的型号中立中间体在此按上游型号定稿，修正值对下游
  estimateUsage / buildSubmitRequest / parseSubmitResponse 可见，与回填 state / immediate 可同时使用；
  写法见下方模板的 validateRequest，真实厂商实现可参考仓库内置插件 alibaba / doubao / hailuo）。
  不导出则跳过
- estimateUsage(ctx)：预扣口径的用量估算，返回宿主固定用量词汇的键值对象（见下方 usage 条目）
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
  batch 为同渠道同插件的全部未终态任务；taskId 是上游任务 ID，taskData 是上一轮快照，state 是上次返回的状态）。
  batch 时 items 里省略上游本轮未回报的任务（或对其返回 done:false）
- parseQueryResponse(ctx)：解析轮询响应 {items: [{taskId, done, taskData?, state?, usage?, artifacts?, error?}]}；
  items[].taskId 必须在本次 ctx.tasks 内；done:true 且带 error:{code, message} = 失败。
  ctx.status 恒为 200（网关不透传轮询的 HTTP 状态码），不要按 404/5xx 写分支——终态一律按
  ctx.responseBody 的内容判定，无法识别的响应返回 done:false（网关的任务超时兜底）。
  终态 usage 优先读上游响应里的实际计量；上游不返回时才读 state 里保存的请求值
- usage（estimateUsage 返回值与终态 items[].usage/immediate.usage）：只报【宿主固定用量词汇】，
  出现词汇外的键整体被拒：output_seconds（生成秒数）/ resolution（输出分辨率，如 "720p"/"1280x720"）/
  output_count（产出数量）/ input_image_count（输入图片数含首尾帧）/ input_video_seconds（参考视频秒数）/
  input_audio_seconds（参考音频秒数）/ input_tokens / output_tokens / generate_audio（布尔）。
  这些键自动注册进计费公式 u() 通道；计价口径（per_second 矩阵/公式/素材方案）由宿主模型定价决定，
  插件对价格世界一无所知。上游消费/积分等成本信息不上报。number 型取非负数值；提交时未知的量
  （如参考视频实际秒数）按上限估算，终态按官方 usage 覆盖（多退少补）。终态只上报上游确实返回
  且可信的键（未上报的键结算沿用预扣估算值）——上游 usage 可能是占位全零（如 duration=0、
  resolution=""），以关键信号（如生成秒数 >0）判断可信度，不可信的键不要上报（报 0 会把该键
  的预扣值覆盖成 0）
- artifacts 产物声明：[{key, mimeType?, source: {type: "url"|"inline", path, auth?}}]，path 是 taskData 内的取值路径，
  对象字段用点分（如 "data.video_url"），数组元素必须用下标（如 "results[0].url"，不要写成 "results.0.url"）；
  type="inline"（base64 字段）时 mimeType 必填；需渠道密钥下载的上游内容端点声明
  auth: "channel-bearer"。插件只指路不碰字节，由平台下载转存对象存储
- renderSubmit / renderTask：面向客户端的响应渲染 {status?, body}（可选，缺省透传）；入参 ctx.task 只有
  这些字段：publicTaskId（平台任务 ID）、taskId（上游任务 ID）、model、status、progress、failReason、
  data、contentUrl——没有 error 字段，失败原因读 task.failReason。status 是平台状态枚举
  SUBMITTED / IN_PROGRESS / SUCCESS / FAILURE（不是上游状态词）；要对外呈现 OpenAI/厂商状态词
  （queued / in_progress / completed / failed 等）必须在渲染钩子里显式映射。progress 是百分比
  字符串（如 "50%"，可能为空），需要数字时 parseInt。data 是最新上游快照，成功任务里 artifacts
  声明路径的值已替换为平台内容链接；contentUrl 为主产物链接。renderSubmit 只收到
  publicTaskId / model / status / contentUrl。renderTasks（dynamic 批量查询的缺省渲染钩子）
  入参 {tasks: [同上任务的数组]}，返回同 {status?, body}
- 通用：抛 Error = 插件级失败（转 4xx/502 透出给调用方）；返回对象出现契约外的顶层字段会被拒绝；
  requestBody/body ≤ 50MiB，taskData/state ≤ 1MiB

四、宿主 API（全局对象 utils，严格按以下签名调用）：
- utils.fetch({url, method?, headers?, body?, responseType?}) → {status, headers, body}：参数是【单个对象】，
  不是 (url, options)；同步阻塞；method 仅 GET/HEAD/POST；responseType 为 "json"(缺省)/"text"/"bytes"；
  每次钩子调用 ≤4 次、单次 15s、总 30s、响应 ≤1MiB；不跟随重定向；url 可为相对路径（拼渠道 baseUrl，
  decode/render 等无渠道上下文的钩子内不可用）或绝对 URL（host 须 ∈ 渠道 host ∪ meta.allowedHosts）；
  鉴权：与渠道 baseUrl 同源（scheme + host 相同，相对路径天然同源）的请求由网关注入
  "Bearer <渠道密钥>"，可用来调渠道自己的接口（如按 file_id 换下载直链）；跨域请求
  （meta.allowedHosts 的第三方域）不带任何鉴权；插件自设的 Authorization 一律被剔除（与 build*Request
  的可覆盖语义不同）。responseType="json" 时仅当响应头 Content-Type 含 json 才把 body 解析为对象，
  否则 body 是字符串（需自行 JSON.parse）
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
- 按上游型号的参数校验（时长/分辨率档位/素材数量/型号与端点匹配）一律放 validateRequest
  （ctx.upstreamModel），decode 钩子只做与型号无关的体形归一——平台模型名可能是渠道映射别名，
  在 decode 里按模型名分支会漏掉别名
- 代码输出一律通过工具调用完成，禁止用围栏输出插件代码（围栏只用于讲解小片段）。五个工具的分工：
  write_code=全量写入（默认方式：约 400 行以内的插件都应一次写完——输出上限足够放下，
  产生新版本）；append_code=仅当单次确实放不下（收到截断失败结果）时才向末尾追加，
  续写段直接从上次结尾接着写、不要重新规划结构；read_code=读取当前完整源码；
  edit_code=修复时精确替换（old_text 须与当前源码完全一致且唯一出现，优先定点修，
  不要重写整个文件）；validate_code=认为写完后调用一次校验（不必每段都校验），
  有 fail 项就用 edit_code 定点修复
- 【升级已有插件】会话中已载入现有插件源码时（右栏有基线版本）：先调用 read_code 查看
  现状，用 edit_code 定点修改（不要重写未受影响的部分），完成后把 meta.version 的
  补丁位 +1（如 1.0.1 → 1.0.2，同 key 同版本无法上传），最后 validate_code 校验
- 【思考从简】先在思考里把结构想清楚（meta/钩子清单，几行足矣），然后直接把正式代码写入
  工具参数——严禁在思考中起草、预演或反复推敲完整代码：思考不会被执行，只会白白消耗
  数万 token 并拖慢每一轮
- 工具参数被截断（收到失败结果）时：改小本次分段重新调用同一工具即可，不要重做整体设计
- 【严禁铺垫】收到生成需求后立即开始调用工具输出代码，不要先输出分析、实现规划、提纲
  （铺垫文字不会被执行，还会耗尽输出上限）；确需澄清的关键信息用一两句话问完即止，
  得到答复后再开始写代码
- 上游协议细节（端点路径、请求/响应字段）以用户提供的文档/示例为准，不要臆造；缺信息时先提问再写代码
- 不要实现与任务无关的能力
- 文案语言：代码注释、utils.log 日志、meta.config 的 label/description 用中文；钩子抛出的错误消息会脱敏、
  按 512 字符截断后透出给 API 调用方（OpenAI 兼容客户端），沿用 golden 模板的英文短句
  （如 "field prompt is required"），一句话说清是哪个字段、合法取值是什么

六、golden 参考模板全文（结构范本：golden_music 音乐模型走原生路由，golden_video 视频模型示范 standardVideos 标准端点双入口）：
`)
	b.WriteString(jsplugin.TemplateSource())
	return b.String()
}

// Validate 复用 jsplugin.DryRun：与插件上传前自检、激活校验同一套检查。
func (JSPluginSkill) Validate(code string) *Report {
	src := strings.TrimSpace(code)
	if src == "" {
		return &Report{Skill: KeyJSPlugin, OK: false, Error: "插件源码为空"}
	}
	rep := jsplugin.DryRun(src)
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
