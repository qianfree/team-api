// Package contract 定义 JS 任务插件的 v1 契约：manifest 结构与钩子出入参。
//
// 契约即产品：本包是插件作者与基座之间唯一的接口事实源。
// 对外交付物（v1.d.ts / docs/plugin-api/v1.md / testdata/golden）必须与本包保持一致。
// 演进规则：v1 内只增可选字段，破坏性变更升 apiVersion。
package contract

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

// APIVersion 当前契约版本。
const APIVersion = 1

// RouteType 路由类型：submit=提交任务，query=查询任务。
type RouteType string

const (
	RouteTypeSubmit  RouteType = "submit"
	RouteTypeQuery   RouteType = "query"
	RouteTypeDynamic RouteType = "dynamic" // 一型两用：解码结果定查询或提交（suno 式 POST /fetch）
)

var (
	pluginKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,29}$`)
	factKeyPattern   = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	versionPattern   = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	routePathPattern = regexp.MustCompile(`^/[A-Za-z0-9_\-./:]*$`)
)

// allowedMethods 路由允许的 HTTP 方法（大写归一后校验）。
var allowedMethods = map[string]struct{}{
	"GET": {}, "POST": {}, "PUT": {}, "DELETE": {}, "PATCH": {},
}

// Manifest 插件清单：plugin.js 的 `export const meta` 解析产物。
type Manifest struct {
	APIVersion   int             `json:"apiVersion"`
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Description  LocalizedString `json:"description,omitempty"` // 兼容本地化对象 {en,zh}（取 zh 优先）
	Author       Author          `json:"author,omitempty"`      // 兼容字符串形态（视为 {name}）
	Website      string          `json:"website,omitempty"`
	Models       []string        `json:"models"`                 // 插件接管的模型（平台标准名）
	FetchMode    string          `json:"fetchMode,omitempty"`    // batch | per_task（默认 per_task）
	Routes       []Route         `json:"routes"`                 // 插件原生路由（挂在 /pjs/{key} 下）
	AllowedHosts []string        `json:"allowedHosts,omitempty"` // utils.fetch 额外允许的 host[:port]
	UsageSchema  UsageSchema     `json:"usageSchema,omitempty"`  // 计费用量单位声明（兼容数组/map 双形态）
	Config       []ConfigField   `json:"config,omitempty"`       // 插件配置项（管理端动态表单）
	Icon         string          `json:"icon,omitempty"`         // 展示图标："text"/"text:<label>" 文字头像，或图标名（前端图标集解析）
}

// UsageField usageSchema 单位声明：插件管「用了多少」（estimateUsage /
// extractUsageOnComplete 返回事实），基座管「多少钱」（模型 formula 定价经 u() 引用）。
type UsageField struct {
	Key         string `json:"key"`                   // 事实键，如 "clips"；公式经 u("clips") 引用
	Type        string `json:"type"`                  // number（可计费）| enum（仅展示）
	Unit        string `json:"unit,omitempty"`        // number 型：second→时长信号 / count→张数信号 / 空→仅 u()
	Description string `json:"description,omitempty"` // 定价页展示语义
}

// ConfigField 插件配置项（与 Go 插件 ConfigFieldDef 对齐，前端 PluginConfigModal 兼容形态）。
type ConfigField struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // string|int|bool|select|json|text
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"` // select 选项
	Required    bool     `json:"required,omitempty"`
	Description string   `json:"description,omitempty"`
	Encrypted   bool     `json:"encrypted,omitempty"` // 敏感字段：AES-256-GCM 存储、界面掩码、空值=保持
	Group       string   `json:"group,omitempty"`     // 分组：配置界面按组渲染小节
	HalfWidth   bool     `json:"halfWidth,omitempty"` // 半宽布局
}

// Author 插件作者：{name, url?}。反序列化兼容字符串形态（"xxx" 等价 {name:"xxx"}），
// 与 new-api 对齐的同时不破坏已上传的 string author 插件。
type Author struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// String 展示形态（无名字时空串）。
func (a Author) String() string { return a.Name }

// UnmarshalJSON 兼容 string / {name,url} 两种形态。
func (a *Author) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var name string
		if err := json.Unmarshal(data, &name); err != nil {
			return err
		}
		a.Name = name
		return nil
	}
	type authorAlias Author
	var alias authorAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	*a = Author(alias)
	return nil
}

// LocalizedString 本地化文案：反序列化兼容字符串与 {en, zh, ...} 对象两种形态
// （new-api 插件用对象；取 zh 优先，回退 en，再回退首个非空值）。
type LocalizedString string

// UnmarshalJSON 兼容 string / 本地化对象。
func (l *LocalizedString) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		*l = ""
		return nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*l = LocalizedString(s)
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return gerror.Wrap(err, "本地化对象需为字符串表")
	}
	// zh 优先 → en → 首个非空
	for _, key := range []string{"zh", "zh-CN", "zh_CN", "en"} {
		if v := m[key]; v != "" {
			*l = LocalizedString(v)
			return nil
		}
	}
	for _, v := range m {
		if v != "" {
			*l = LocalizedString(v)
			return nil
		}
	}
	*l = ""
	return nil
}

// String 展示形态。
func (l LocalizedString) String() string { return string(l) }

// UsageSchema 计费用量声明列表；反序列化兼容两种形态：
//   - 我们的数组形态：[{key, type, unit, description}]
//   - new-api 的 map 形态：{clips: {type:"number", unit:"count", description:{en,zh}},
//     action: {enum:[...], enumLabels:{...}}}——键→key、enum→type:"enum"、
//     description 兼容字符串/本地化对象，enumLabels 丢弃（仅 UI 展示）
type UsageSchema []UsageField

// UnmarshalJSON 兼容数组 / map 双形态。
func (u *UsageSchema) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "" {
		*u = nil
		return nil
	}
	if trimmed[0] == '[' {
		var fields []UsageField
		if err := json.Unmarshal(data, &fields); err != nil {
			return err
		}
		*u = fields
		return nil
	}
	// map 形态：逐键转换
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return gerror.Wrap(err, "usageSchema 需为数组或对象")
	}
	fields := make([]UsageField, 0, len(raw))
	for key, val := range raw {
		var f UsageField
		var entry struct {
			Type        string          `json:"type"`
			Unit        string          `json:"unit"`
			Enum        []string        `json:"enum"`
			Description json.RawMessage `json:"description"`
		}
		if err := json.Unmarshal(val, &entry); err != nil {
			return gerror.Wrapf(err, "usageSchema.%s 解析失败", key)
		}
		f.Key = key
		f.Type = entry.Type
		if len(entry.Enum) > 0 {
			f.Type = "enum" // new-api 用 enum 数组表达枚举
		}
		f.Unit = entry.Unit
		if len(entry.Description) > 0 {
			var desc LocalizedString
			if err := json.Unmarshal(entry.Description, &desc); err == nil {
				f.Description = desc.String()
			}
		}
		fields = append(fields, f)
	}
	*u = fields
	return nil
}

// Route 一条插件原生路由。钩子名在 v1 为固定约定（decodeRequest / buildSubmitRequest /
// parseSubmitResponse / buildQueryRequest / parseQueryResponse / renderSubmit / renderTask），
// 路由可选经 decode/render 声明别名（new-api 兼容：routes[].decode/render 指向插件
// 任意导出函数，缺省回落固定钩子名，见 ResolveDecode/ResolveRender）。
type Route struct {
	Method string    `json:"method"` // HTTP 方法，大写
	Path   string    `json:"path"`   // 原生路径，支持 :param 段，如 /submit/:action
	Type   RouteType `json:"type"`
	Decode string    `json:"decode,omitempty"` // submit 路由的 decodeRequest 钩子别名（camelCase 导出名）
	Render string    `json:"render,omitempty"` // 渲染钩子别名（submit→renderSubmit、query→renderTask）
}

// HookNameDecode 默认解码钩子名。
const HookNameDecode = "decodeRequest"

// HookNameRenderSubmit 默认提交渲染钩子名。
const HookNameRenderSubmit = "renderSubmit"

// HookNameRenderTask 默认查询渲染钩子名。
const HookNameRenderTask = "renderTask"

// hookNamePattern 钩子别名形态：字母开头的标识符（camelCase，如 decodeSubmit）。
var hookNamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,63}$`)

// ValidateHookName 校验钩子别名形态。
func ValidateHookName(name string) bool { return hookNamePattern.MatchString(name) }

// ResolveDecode 解码钩子名：路由别名优先，回落 decodeRequest。
func (r Route) ResolveDecode() string {
	if r.Decode != "" {
		return r.Decode
	}
	return HookNameDecode
}

// ResolveRender 渲染钩子名：路由别名优先，回落给定默认（按路由类型传 renderSubmit/renderTask）。
func (r Route) ResolveRender(defaultName string) string {
	if r.Render != "" {
		return r.Render
	}
	return defaultName
}

// Validate 校验 manifest 合法性（上传与激活两个时点都会调用）。
func (m *Manifest) Validate() error {
	if m.APIVersion != APIVersion {
		return gerror.Newf("apiVersion 必须为 %d，当前 %d", APIVersion, m.APIVersion)
	}
	if !pluginKeyPattern.MatchString(m.Key) {
		return gerror.Newf("key %q 不合法：小写字母开头，仅小写字母/数字/连字符，长度 2-30", m.Key)
	}
	if strings.TrimSpace(m.Name) == "" {
		return gerror.New("name 不能为空")
	}
	if !versionPattern.MatchString(m.Version) {
		return gerror.Newf("version %q 不合法：需 semver（如 1.0.0）", m.Version)
	}
	if len(m.Models) == 0 {
		return gerror.New("models 不能为空：至少声明一个接管的模型")
	}
	for i, model := range m.Models {
		if strings.TrimSpace(model) == "" {
			return gerror.Newf("models[%d] 为空", i)
		}
	}
	switch m.FetchMode {
	case "", "batch", "per_task":
	default:
		return gerror.Newf("fetchMode %q 不合法：batch 或 per_task", m.FetchMode)
	}
	if len(m.Routes) == 0 {
		return gerror.New("routes 不能为空：至少声明一条路由")
	}
	seen := make(map[string]struct{}, len(m.Routes))
	for i, route := range m.Routes {
		if err := route.Validate(); err != nil {
			return gerror.Wrapf(err, "routes[%d]", i)
		}
		// 插件内 method+路径形状唯一（:param 段视为同一形状）
		shape := route.Method + " " + NormalizePathShape(route.Path)
		if _, dup := seen[shape]; dup {
			return gerror.Newf("routes[%d] 重复：%s %s", i, route.Method, route.Path)
		}
		seen[shape] = struct{}{}
	}
	for i, host := range m.AllowedHosts {
		if err := ValidateHostEntry(host); err != nil {
			return gerror.Wrapf(err, "allowedHosts[%d]", i)
		}
	}
	if m.Website != "" && !strings.HasPrefix(m.Website, "https://") {
		return gerror.New("website 需为 https 绝对地址")
	}
	if err := m.validateIcon(); err != nil {
		return err
	}
	if err := m.validateUsageSchema(); err != nil {
		return err
	}
	if err := m.validateConfig(); err != nil {
		return err
	}
	return nil
}

// iconPattern 图标名形态：小写字母/数字/连字符（如 "video-camera"）。
var iconNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// validateIcon 校验展示图标声明：
//   - 空 = 由前端按名称首字母渲染
//   - "text" / "text:<label>" = 文字头像（label ≤ 4 字符）
//   - 图标名（前端图标集解析，未知名渲染回落首字母）
//   - 拒绝 data: / http(s) 内联（图片走上传通道的独立 icon 字段，经安全校验落库）
func (m *Manifest) validateIcon() error {
	if m.Icon == "" {
		return nil
	}
	if len(m.Icon) > 128 {
		return gerror.New("icon 过长（≤128 字符）")
	}
	lower := strings.ToLower(m.Icon)
	if strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return gerror.New("icon 不允许 data:/http 链接（图片请走上传的 icon 文件通道）")
	}
	if m.Icon == "text" {
		return nil
	}
	if strings.HasPrefix(m.Icon, "text:") {
		label := m.Icon[len("text:"):]
		if label == "" || len([]rune(label)) > 4 {
			return gerror.New("icon text: 标签需 1-4 字符")
		}
		return nil
	}
	if !iconNamePattern.MatchString(m.Icon) {
		return gerror.Newf("icon %q 不是合法图标名（小写字母/数字/连字符）或 text 形态", m.Icon)
	}
	return nil
}

// validateUsageSchema 校验用量单位声明：键唯一、number/enum 二型、unit 仅 number 可用。
func (m *Manifest) validateUsageSchema() error {
	seen := make(map[string]struct{}, len(m.UsageSchema))
	for i, f := range m.UsageSchema {
		if !factKeyPattern.MatchString(f.Key) {
			return gerror.Newf("usageSchema[%d].key %q 不合法：小写字母开头的字母/数字/下划线", i, f.Key)
		}
		if _, dup := seen[f.Key]; dup {
			return gerror.Newf("usageSchema[%d].key %q 重复", i, f.Key)
		}
		seen[f.Key] = struct{}{}
		switch f.Type {
		case "number":
			switch f.Unit {
			case "", "second", "count":
			default:
				return gerror.Newf("usageSchema[%d] %q 的 unit %q 不合法：second/count/空", i, f.Key, f.Unit)
			}
		case "enum", "boolean": // enum/boolean 为信息性声明（不进 u() 计费通道）
			if f.Unit != "" {
				return gerror.Newf("usageSchema[%d] %q 为 %s，不允许 unit", i, f.Key, f.Type)
			}
		default:
			return gerror.Newf("usageSchema[%d].type %q 不合法：number/enum/boolean", i, f.Type)
		}
	}
	return nil
}

// validateConfig 校验配置项声明：键唯一、类型受支持、select 必带选项、加密项无默认值。
func (m *Manifest) validateConfig() error {
	seen := make(map[string]struct{}, len(m.Config))
	validTypes := map[string]struct{}{
		"string": {}, "int": {}, "bool": {}, "select": {}, "json": {}, "text": {},
	}
	for i, f := range m.Config {
		if !factKeyPattern.MatchString(f.Key) {
			return gerror.Newf("config[%d].key %q 不合法：小写字母开头的字母/数字/下划线", i, f.Key)
		}
		if _, dup := seen[f.Key]; dup {
			return gerror.Newf("config[%d].key %q 重复", i, f.Key)
		}
		seen[f.Key] = struct{}{}
		if strings.TrimSpace(f.Label) == "" {
			return gerror.Newf("config[%d] %q 缺少 label", i, f.Key)
		}
		if _, ok := validTypes[f.Type]; !ok {
			return gerror.Newf("config[%d] %q 类型 %q 不合法", i, f.Key, f.Type)
		}
		if f.Type == "select" && len(f.Options) == 0 {
			return gerror.Newf("config[%d] %q 为 select，必须声明 options", i, f.Key)
		}
		if f.Encrypted && f.Default != nil && f.Default != "" {
			return gerror.Newf("config[%d] %q 为加密字段，不允许默认值", i, f.Key)
		}
	}
	return nil
}

// UsageFieldByKey 按 key 查声明。
func (m *Manifest) UsageFieldByKey(key string) *UsageField {
	for i := range m.UsageSchema {
		if m.UsageSchema[i].Key == key {
			return &m.UsageSchema[i]
		}
	}
	return nil
}

// ConfigFieldByKey 按 key 查声明。
func (m *Manifest) ConfigFieldByKey(key string) *ConfigField {
	for i := range m.Config {
		if m.Config[i].Key == key {
			return &m.Config[i]
		}
	}
	return nil
}

// Validate 校验单条路由。
func (r Route) Validate() error {
	method := strings.ToUpper(r.Method)
	if _, ok := allowedMethods[method]; !ok {
		return gerror.Newf("method %q 不合法", r.Method)
	}
	if !routePathPattern.MatchString(r.Path) {
		return gerror.Newf("path %q 不合法：需以 / 开头的合法路径段", r.Path)
	}
	switch r.Type {
	case RouteTypeSubmit, RouteTypeQuery, RouteTypeDynamic:
	default:
		return gerror.Newf("type %q 不合法：submit、query 或 dynamic", r.Type)
	}
	if r.Render != "" && !ValidateHookName(r.Render) {
		return gerror.Newf("render %q 不合法（需为导出函数名形态）", r.Render)
	}
	if r.Decode != "" && !ValidateHookName(r.Decode) {
		return gerror.Newf("decode %q 不合法（需为导出函数名形态）", r.Decode)
	}
	if r.Decode != "" && r.Type != RouteTypeSubmit && r.Type != RouteTypeDynamic {
		return gerror.New("decode 别名仅 submit/dynamic 路由允许")
	}
	return nil
}

// EffectiveFetchMode 归一化轮询模式。
func (m *Manifest) EffectiveFetchMode() string {
	if m.FetchMode == "batch" {
		return "batch"
	}
	return "per_task"
}

// HasModel 报告模型是否被本插件声明（精确匹配，模型名大小写敏感）。
func (m *Manifest) HasModel(model string) bool {
	for _, mm := range m.Models {
		if mm == model {
			return true
		}
	}
	return false
}

// ValidateHostEntry 校验 allowedHosts 条目：host 或 host:port（IPv6 字面量带方括号），
// 不允许 scheme/path/凭据/查询串。
func ValidateHostEntry(entry string) error {
	entry = strings.ToLower(strings.TrimSpace(entry))
	if entry == "" {
		return gerror.New("不能为空")
	}
	if strings.ContainsAny(entry, "/?#@") {
		return gerror.Newf("%q 不允许包含 scheme/path/凭据/查询", entry)
	}
	host, port, ok := splitLastColon(entry)
	if !ok {
		return validateHostName(entry)
	}
	if err := validateHostName(host); err != nil {
		return err
	}
	if port == "" || len(port) > 5 {
		return gerror.Newf("%q 端口不合法", entry)
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return gerror.Newf("%q 端口不合法", entry)
		}
	}
	return nil
}

func splitLastColon(s string) (host, port string, ok bool) {
	// IPv6 字面量 [::1]:8080 —— 只在最后一个 ] 之后找冒号
	if idx := strings.LastIndex(s, "]"); idx >= 0 {
		rest := s[idx+1:]
		if strings.HasPrefix(rest, ":") {
			return s[:idx+1], rest[1:], true
		}
		return "", "", false
	}
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return "", "", false
	}
	return s[:idx], s[idx+1:], true
}

func validateHostName(host string) error {
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return nil // IPv6 字面量
	}
	if host == "" || len(host) > 253 {
		return gerror.Newf("%q host 不合法", host)
	}
	return nil
}

// NormalizePathShape 把路径归一为匹配形状：连续斜杠压缩、尾部斜杠去除，
// 保留 :param 与 * 通配段原样（分发器按形状索引，见 router 包）。
func NormalizePathShape(path string) string {
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		path = "/"
	}
	return path
}
