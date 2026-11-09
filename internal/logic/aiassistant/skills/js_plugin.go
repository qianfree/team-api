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
- models：接管的模型名数组（非空；模型跨插件独占，除非双方都声明 sharedModels）
- fetchMode：batch | per_task（缺省 per_task）
- routes：[{method, path, type}]，method 大写，path 支持 :param 段，type ∈ submit | query | dynamic
  （dynamic 一型两用：解码结果定查询或提交）；可选 decode/render 字段声明钩子别名
- usageSchema：计费用量声明 [{key, type: number|enum|boolean, unit: second|count|空, description}]，
  number 型键自动注册进计费公式 u() 注册表
- config：插件配置项 [{key, label, type: string|int|bool|select|json|text, default, options, required, encrypted, group}]
- allowedHosts：utils.fetch 额外允许的 host[:port] 白名单
- icon："text" / "text:<≤4字符>" / 图标名

三、钩子（按需导出，golden 模板有完整示范）：
- decodeRequest(ctx)：把客户端请求解码为 {kind: "submit"|"query", model, requestBody | taskIds}；入参 ctx={method, path, params, body}
- estimateUsage(ctx)：预扣口径的用量估算，返回 usageSchema 声明的键值对象
- buildSubmitRequest(ctx)：构造上游请求 {url, method, headers, body}；url 相对路径会拼接到渠道 baseUrl
- parseSubmitResponse(ctx)：解析提交响应 {taskId, taskData, state}；state 跨轮存活
- buildQueryRequest(ctx)：构造轮询请求
- parseQueryResponse(ctx)：解析轮询响应 {items: [{taskId, done, taskData, usage, artifacts, error, state}]}
- renderSubmit / renderTask：面向客户端的响应渲染（可选，缺省透传）
- artifacts 产物声明：[{key, source: {type: "url"|"inline", path}}]，产物由平台下载转存对象存储

四、宿主 API（全局对象 utils）：
- utils.fetch(url, options)：受白名单约束的 HTTP 请求（host 需在渠道 host ∪ allowedHosts 内）
- utils.config.get(key)：读插件配置值（encrypted 字段自动解密）
- utils.log(...)：日志；utils.now()：当前时间；utils.uuid()：UUID；utils.crypto：常用摘要/编解码

五、生成硬规则：
- 必须基于下方 golden 模板的结构编写，meta 与钩子形状不得自创
- 上游协议细节（端点路径、请求/响应字段）以用户提供的文档/示例为准，不要臆造；缺信息时先在围栏外列出问题清单再给代码
- 不要实现与任务无关的能力；所有用户可见文案用中文

六、golden 参考模板全文（结构范本）：
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
