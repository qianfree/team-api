package contract

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

// MaxPassthroughBytes 透传容器（taskData / state / requestBody）的大小上限。
const MaxPassthroughBytes = 1 << 20 // 1 MiB

// ---------------------------------------------------------------------------
// 钩子入参（基座 → 插件）：每钩子一个 ctx 对象，JSON 边界传递。
// ---------------------------------------------------------------------------

// DecodeContext decodeRequest 入参：把客户端请求分类归一。
type DecodeContext struct {
	Path    string            `json:"path"`              // 原生路径（不含 /pjs/{key} 前缀），如 /submit/music
	Method  string            `json:"method"`            // HTTP 方法
	Params  map[string]string `json:"params,omitempty"`  // 路径参数（:action 等）
	Body    json.RawMessage   `json:"body,omitempty"`    // 客户端请求体（JSON 原文；multipart 时为字段合并对象）
	Headers map[string]string `json:"headers,omitempty"` // 客户端请求头（Authorization 已剔除）
	// Files multipart 上传的文件引用（FileReference）：ref 为请求级不透明句柄，
	// 字节永不进 JS——插件在 buildSubmitRequest 的 body 里以 {__fileRef, encoding,
	// maxBytes?} 占位符引用，宿主发送前内联编码。同名字段多文件时 ref 带 #index。
	Files []FileRef `json:"files,omitempty"`
}

// FileRef 客户端上传文件的引用描述：ref 为请求级不透明句柄，文件字节永不进 JS。
type FileRef struct {
	Ref      string `json:"ref"`      // 如 request_file:input_reference（首个）/ request_file:images#1（后续）
	Field    string `json:"field"`    // multipart 字段名
	Filename string `json:"filename"` // 原始文件名
	MimeType string `json:"mimeType"` // 探测的内容类型
	Size     int64  `json:"size"`     // 字节数
}

// 标准任务端点入口标识（StandardEntryContext.Surface）。
const (
	// SurfaceOpenAIVideos POST /v1/videos（OpenAI Videos 协议，入站已归一为通用任务体）
	SurfaceOpenAIVideos = "openai_videos"
	// SurfaceVideoGenerations POST /v1/video/generations（通用任务端点，body 客户端原样）
	SurfaceVideoGenerations = "video_generations"
)

// StandardEntryContext standardVideos 声明的解码钩子入参：标准任务端点进入的请求。
// Body 为任务框架通用任务体（{model, prompt, seconds?, metadata.{duration,size,
// aspect_ratio,sound,image,...}}；/v1/video/generations 为客户端原样 body），
// 由插件映射为原生 requestBody 后进入既有提交链（estimateUsage/buildSubmitRequest
// 等下游钩子无感复用）。返回形态与 decodeRequest 相同，kind 恒为 submit。
type StandardEntryContext struct {
	Surface string          `json:"surface"` // 入口标识：openai_videos | video_generations
	Path    string          `json:"path"`    // 标准端点路径（/v1/videos、/v1/video/generations）
	Body    json.RawMessage `json:"body"`    // 通用任务体
	Model   string          `json:"model"`   // 请求声明的模型名（body.model，已解析到本插件）
}

// BuildSubmitContext buildSubmitRequest 入参：构造上游提交请求。
type BuildSubmitContext struct {
	Body           json.RawMessage   `json:"body"` // 归一后的请求体（decode 返回的 requestBody）
	Params         map[string]string `json:"params,omitempty"`
	Model          string            `json:"model"`                   // 平台标准模型名（计费键）
	UpstreamModel  string            `json:"upstreamModel,omitempty"` // 渠道映射后的上游模型名
	ChannelBaseURL string            `json:"channelBaseUrl"`          // 渠道 base URL
	// ApiKey 渠道密钥（如 kling 的 accessKey|secretKey）：加签类插件用它经 utils.crypto
	// 计算鉴权头后再覆盖基座默认 Bearer。信任边界=上传即信任，插件可读该值。
	ApiKey string          `json:"apiKey,omitempty"`
	State  json.RawMessage `json:"state,omitempty"` // 插件状态（提交阶段通常为空）
}

// ParseSubmitContext parseSubmitResponse 入参。
type ParseSubmitContext struct {
	Status       int               `json:"status"`                 // 上游 HTTP 状态码
	Path         string            `json:"path,omitempty"`         // 提交路由原生路径（识别 action 用）
	Params       map[string]string `json:"params,omitempty"`       // 提交路由路径参数
	Model        string            `json:"model,omitempty"`        // 平台模型名
	ResponseBody json.RawMessage   `json:"responseBody,omitempty"` // 上游响应体（JSON 原文；非 JSON 为字符串）
	// Body 归一后的提交请求体（decode 返回的 requestBody）：响应不含 action 线索时
	//（如标准端点入站，path 是 /v1/videos 而非原生路由）由此推断提交形态。
	Body json.RawMessage `json:"body,omitempty"`
}

// BuildQueryContext buildQueryRequest 入参：构造上游查询请求。
type BuildQueryContext struct {
	Tasks []QueryTaskRef `json:"tasks"` // batch 模式为该插件全部未终态任务；per_task 恒为单元素
	Model string         `json:"model,omitempty"`
}

// QueryTaskRef 查询任务引用。
type QueryTaskRef struct {
	TaskID   string          `json:"taskId"` // 上游任务 ID
	TaskData json.RawMessage `json:"taskData,omitempty"`
	State    json.RawMessage `json:"state,omitempty"`
}

// ParseQueryContext parseQueryResponse 入参。
type ParseQueryContext struct {
	ResponseBody json.RawMessage `json:"responseBody,omitempty"`
	Status       int             `json:"status"`
	Tasks        []QueryTaskRef  `json:"tasks"` // 与本次查询对应的任务（echo）
	// ChannelBaseURL 渠道地址（供插件构造需鉴权的产物相对路径，配合 artifacts[].source.auth）
	ChannelBaseURL string `json:"channelBaseUrl,omitempty"`
}

// RenderTaskContext renderSubmit / renderTask 入参：把任务渲染给客户端。
type RenderTaskContext struct {
	Task RenderTask `json:"task"`
}

// RenderTasksContext dynamic 路由查询的批量渲染入参（suno 式 POST /fetch 批量回包）。
type RenderTasksContext struct {
	Tasks []RenderTask `json:"tasks"`
}

// RenderTask 渲染视图的任务字段。
type RenderTask struct {
	PublicTaskID string          `json:"publicTaskId"` // 平台任务 ID（task_xxx）
	TaskID       string          `json:"taskId"`       // 上游任务 ID
	Model        string          `json:"model"`
	Status       string          `json:"status"` // 平台状态：SUBMITTED/IN_PROGRESS/SUCCESS/FAILURE
	Progress     string          `json:"progress,omitempty"`
	FailReason   string          `json:"failReason,omitempty"`
	Data         json.RawMessage `json:"data,omitempty"` // 最新上游快照（taskData；登记产物的声明路径已替换为统一内容链接）
	// ContentURL 主产物统一内容链接（平台签名直链，免鉴权可直接打开；成功且有产物时非空）。
	// 产物是否转存 OSS 不影响链接形态——转存的从 OSS 供给，未转存的平台实时回源
	ContentURL string `json:"contentUrl,omitempty"`
}

// ---------------------------------------------------------------------------
// 钩子返回（插件 → 基座）：顶层字段白名单校验，透传容器只限大小。
// ---------------------------------------------------------------------------

// DecodeResult decodeRequest 返回。kind=submit（标准提交）；kind=query 仅 dynamic
// 路由允许——解码出「这是存量任务查询」时（如 suno 式 POST /fetch 的 ids 列表），
// 分发器改走批量查询渲染，不进提交流。
type DecodeResult struct {
	Kind        string          `json:"kind"`        // submit | query
	Model       string          `json:"model"`       // submit：声明于 meta.models 的平台模型名
	RequestBody json.RawMessage `json:"requestBody"` // submit：归一后的请求体（喂给 buildSubmitRequest）
	TaskIDs     []string        `json:"taskIds"`     // query：待查公开任务 ID 列表（dynamic 路由）
}

// ValidateDecoded 校验 decodeRequest 返回。dynamicRoute 标记当前路由类型为 dynamic
// （kind=query 仅此类型允许）。
func ValidateDecoded(raw map[string]json.RawMessage, m *Manifest, dynamicRoute bool) (*DecodeResult, error) {
	if err := rejectUnknownFields(raw, "kind", "model", "requestBody", "taskIds"); err != nil {
		return nil, err
	}
	var r DecodeResult
	if err := strictUnmarshal(raw, &r); err != nil {
		return nil, err
	}
	switch r.Kind {
	case "submit":
		if !m.HasModel(r.Model) {
			return nil, gerror.Newf("model %q 未在 meta.models 中声明", r.Model)
		}
		if len(r.RequestBody) == 0 {
			r.RequestBody = json.RawMessage("{}")
		}
		if len(r.RequestBody) > MaxPassthroughBytes {
			return nil, gerror.Newf("requestBody 超过 %d 字节上限", MaxPassthroughBytes)
		}
		return &r, nil
	case "query":
		if !dynamicRoute {
			return nil, gerror.New("kind=query 仅 dynamic 路由允许")
		}
		if len(r.TaskIDs) == 0 {
			return nil, gerror.New("kind=query 需要非空 taskIds")
		}
		if len(r.TaskIDs) > 100 {
			return nil, gerror.New("taskIds 单次最多 100 个")
		}
		return &r, nil
	default:
		return nil, gerror.Newf("kind 必须为 submit 或 query，当前 %q", r.Kind)
	}
}

// BuildRequestResult buildSubmitRequest / buildQueryRequest 返回。
type BuildRequestResult struct {
	URL     string            `json:"url"`               // 相对路径（拼接到渠道 baseURL）或绝对 URL（host 须在白名单：渠道 host ∪ allowedHosts，由 jsbridge 强校验）
	Method  string            `json:"method,omitempty"`  // 缺省 POST（submit）/POST（query）
	Headers map[string]string `json:"headers,omitempty"` // 附加请求头（Authorization 允许声明：签名类厂商由插件计算鉴权头覆盖默认 Bearer）
	Body    json.RawMessage   `json:"body,omitempty"`
}

// ValidateBuiltRequest 校验 build*Request 返回。
func ValidateBuiltRequest(raw map[string]json.RawMessage) (*BuildRequestResult, error) {
	if err := rejectUnknownFields(raw, "url", "method", "headers", "body"); err != nil {
		return nil, err
	}
	var r BuildRequestResult
	if err := strictUnmarshal(raw, &r); err != nil {
		return nil, err
	}
	if strings.TrimSpace(r.URL) == "" {
		return nil, gerror.New("url 不能为空")
	}
	if len(r.Body) > MaxPassthroughBytes {
		return nil, gerror.Newf("body 超过 %d 字节上限", MaxPassthroughBytes)
	}
	// Authorization 允许声明：加签类厂商（如 kling 的 JWT）由插件用 ctx.apiKey
	// 经 utils.crypto 算出鉴权头，覆盖基座默认 Bearer。
	if r.Method != "" {
		if _, ok := allowedMethods[strings.ToUpper(r.Method)]; !ok {
			return nil, gerror.Newf("method %q 不合法", r.Method)
		}
		r.Method = strings.ToUpper(r.Method)
	}
	return &r, nil
}

// ParseSubmitResult parseSubmitResponse 返回。
type ParseSubmitResult struct {
	TaskID   string          `json:"taskId"`             // 必填：上游任务标识
	TaskData json.RawMessage `json:"taskData,omitempty"` // 上游快照，落库 task.Data，轮询时回带
	State    json.RawMessage `json:"state,omitempty"`    // 插件状态，跨轮存活；省略=保持
	// Immediate 同步即完成（同步图片 API 等）：任务提交即终态，免轮询直达结算。
	Im *ImmediateResult `json:"immediate,omitempty"`
}

// ImmediateResult 同步终态：status 必填（SUCCESS/FAILURE）；taskData 为终态快照；
// usage/artifacts 语义与 parseQueryResponse 终态一致（结算事实与产物声明）。
type ImmediateResult struct {
	Status     string          `json:"status"`             // SUCCESS | FAILURE
	TaskData   json.RawMessage `json:"taskData,omitempty"` // 终态快照（缺省沿用 taskData）
	Usage      json.RawMessage `json:"usage,omitempty"`
	Artifacts  []ArtifactDecl  `json:"artifacts,omitempty"`
	FailReason string          `json:"failReason,omitempty"` // FAILURE 时必填
}

// ValidateSubmitted 校验 parseSubmitResponse 返回。
func ValidateSubmitted(raw map[string]json.RawMessage) (*ParseSubmitResult, error) {
	if err := rejectUnknownFields(raw, "taskId", "taskData", "state", "immediate"); err != nil {
		return nil, err
	}
	var r ParseSubmitResult
	if err := strictUnmarshal(raw, &r); err != nil {
		return nil, err
	}
	if strings.TrimSpace(r.TaskID) == "" {
		return nil, gerror.New("taskId 不能为空")
	}
	if len(r.TaskData) > MaxPassthroughBytes {
		return nil, gerror.Newf("taskData 超过 %d 字节上限", MaxPassthroughBytes)
	}
	if len(r.State) > MaxPassthroughBytes {
		return nil, gerror.Newf("state 超过 %d 字节上限", MaxPassthroughBytes)
	}
	if r.Im != nil {
		switch r.Im.Status {
		case "SUCCESS", "FAILURE":
		default:
			return nil, gerror.Newf("immediate.status 必须为 SUCCESS/FAILURE，当前 %q", r.Im.Status)
		}
		if len(r.Im.TaskData) > MaxPassthroughBytes {
			return nil, gerror.Newf("immediate.taskData 超过 %d 字节上限", MaxPassthroughBytes)
		}
		if r.Im.Status == "FAILURE" && r.Im.FailReason == "" {
			r.Im.FailReason = "immediate failure without reason"
		}
		for j := range r.Im.Artifacts {
			if err := r.Im.Artifacts[j].validate(); err != nil {
				return nil, gerror.Wrap(err, "immediate.artifacts")
			}
		}
	}
	return &r, nil
}

// TaskHookError 任务级失败（不炸整批）。
type TaskHookError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ArtifactDecl 产物声明：插件只指路（JSON path 指向 taskData 里的上游 URL 或
// base64 字段），字节永不进 JS——转存由宿主产物流水线完成。
type ArtifactDecl struct {
	Key      string `json:"key"`                // 任务内唯一，如 "video"
	MimeType string `json:"mimeType,omitempty"` // 存储内容类型（inline 必填，url 可选覆盖）
	Source   struct {
		Type string `json:"type"` // url（下载转存）| inline（base64 字段直存）
		Path string `json:"path"` // taskData 内的取值路径，如 "data.videos[0].url"
		// Auth 需鉴权下载的凭据形态："channel-bearer" = 以渠道 ApiKey 注入
		// Authorization（如 sora 的 /v1/videos/{id}/content）；相对 URL 由
		// 产物流水线按渠道 baseURL 解析。
		Auth string `json:"auth,omitempty"`
	} `json:"source"`
}

// QueryItem parseQueryResponse 返回的单任务结果。
type QueryItem struct {
	TaskID    string          `json:"taskId"`              // 对应 BuildQueryContext.Tasks 的回带
	Done      bool            `json:"done"`                // 终态判定
	TaskData  json.RawMessage `json:"taskData,omitempty"`  // 最新上游快照（每轮整体覆盖）
	State     json.RawMessage `json:"state,omitempty"`     // 省略=保持
	Usage     json.RawMessage `json:"usage,omitempty"`     // 终态用量事实（键 ∈ usageSchema）
	Artifacts []ArtifactDecl  `json:"artifacts,omitempty"` // 终态产物声明
	Error     *TaskHookError  `json:"error,omitempty"`     // 任务级失败；done=true 且有 error → FAILURE
}

// QueryResult parseQueryResponse 返回。
type QueryResult struct {
	Items []QueryItem `json:"items"`
}

// ValidateQueried 校验 parseQueryResponse 返回，并核对 items 与请求任务集对应。
func ValidateQueried(raw map[string]json.RawMessage, want []QueryTaskRef) (*QueryResult, error) {
	if err := rejectUnknownFields(raw, "items"); err != nil {
		return nil, err
	}
	var r QueryResult
	if err := strictUnmarshal(raw, &r); err != nil {
		return nil, err
	}
	if len(r.Items) == 0 {
		return nil, gerror.New("items 不能为空")
	}
	wanted := make(map[string]struct{}, len(want))
	for _, t := range want {
		wanted[t.TaskID] = struct{}{}
	}
	for i := range r.Items {
		item := &r.Items[i]
		if strings.TrimSpace(item.TaskID) == "" {
			return nil, gerror.Newf("items[%d].taskId 为空", i)
		}
		if _, ok := wanted[item.TaskID]; !ok {
			return nil, gerror.Newf("items[%d].taskId %q 不在本次查询任务集内", i, item.TaskID)
		}
		if len(item.TaskData) > MaxPassthroughBytes {
			return nil, gerror.Newf("items[%d].taskData 超过 %d 字节上限", i, MaxPassthroughBytes)
		}
		if len(item.State) > MaxPassthroughBytes {
			return nil, gerror.Newf("items[%d].state 超过 %d 字节上限", i, MaxPassthroughBytes)
		}
		if item.Error != nil && strings.TrimSpace(item.Error.Message) == "" {
			item.Error.Message = "unknown task error"
		}
		for j := range item.Artifacts {
			if err := item.Artifacts[j].validate(); err != nil {
				return nil, gerror.Wrapf(err, "items[%d].artifacts[%d]", i, j)
			}
		}
	}
	return &r, nil
}

// artifactKeyPattern 产物 key 形态：字母数字开头，允许连字符/下划线（对齐 DB 列宽 128；
// 常见取值形如 "audio-0"、"cover-1"、"video"）。
var artifactKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)

// validate 校验单条产物声明。
func (a *ArtifactDecl) validate() error {
	if !artifactKeyPattern.MatchString(a.Key) {
		return gerror.Newf("key %q 不合法", a.Key)
	}
	switch a.Source.Type {
	case "url", "inline":
	default:
		return gerror.Newf("source.type %q 不合法：url 或 inline", a.Source.Type)
	}
	if a.Source.Path == "" || len(a.Source.Path) > 256 {
		return gerror.New("source.path 需要 1-256 字符的 taskData 取值路径")
	}
	if a.Source.Type == "inline" && a.MimeType == "" {
		return gerror.New("inline 产物必须声明 mimeType")
	}
	return nil
}

// ---------------------------------------------------------------------------
// 计费事实钩子（可选）：estimateUsage 提交预扣估算；终态结算事实由 parseQueryResponse
// 的 items[].usage 内联上报，v1 不设独立的终态用量钩子
// ---------------------------------------------------------------------------

// EstimateUsageContext estimateUsage 入参。
type EstimateUsageContext struct {
	Body   json.RawMessage   `json:"body"`  // 归一后的请求体
	Model  string            `json:"model"` // 平台标准模型名
	Params map[string]string `json:"params,omitempty"`
}

// UsageFacts 用量事实：键必须 ∈ usageSchema 声明。number 型进计费（u() 通道，
// unit=second/count 额外映射为时长/张数信号），enum 型仅随快照记录。
type UsageFacts map[string]any

// ValidateUsageFacts 校验用量事实：键已声明、number 型取值可转 float、非负。
// 返回过滤后的数值事实（enum 值剔除）。
func ValidateUsageFacts(raw map[string]json.RawMessage, m *Manifest) (map[string]float64, error) {
	out := make(map[string]float64, len(raw))
	for key, valueJSON := range raw {
		field := m.UsageFieldByKey(key)
		if field == nil {
			return nil, gerror.Newf("usage.%s 未在 usageSchema 中声明", key)
		}
		if field.Type != "number" {
			continue // enum/boolean 值合法但不计费（信息性）
		}
		var v any
		if err := json.Unmarshal(valueJSON, &v); err != nil {
			return nil, gerror.Newf("usage.%s 不是合法 JSON 值", key)
		}
		num, ok := toFloat(v)
		if !ok {
			return nil, gerror.Newf("usage.%s 不是数值（number 型事实必须可转数字）", key)
		}
		if num < 0 {
			return nil, gerror.Newf("usage.%s 为负数", key)
		}
		out[key] = num
	}
	return out, nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		var f float64
		if _, err := fmt.Sscanf(n, "%g", &f); err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// RenderResult renderSubmit / renderTask 返回。
type RenderResult struct {
	Status int             `json:"status,omitempty"` // 缺省 200
	Body   json.RawMessage `json:"body"`             // 返回给客户端的 JSON
}

// ValidateRendered 校验渲染返回。
func ValidateRendered(raw map[string]json.RawMessage) (*RenderResult, error) {
	if err := rejectUnknownFields(raw, "status", "body"); err != nil {
		return nil, err
	}
	var r RenderResult
	if err := strictUnmarshal(raw, &r); err != nil {
		return nil, err
	}
	// HTTP 状态码合法区间：超出 [100,599] 的值落进 WriteStatus 会产生非法响应
	if r.Status < 0 {
		return nil, gerror.New("status 不能为负")
	}
	if r.Status > 0 && (r.Status < 100 || r.Status > 599) {
		return nil, gerror.Newf("status %d 不在合法 HTTP 状态码区间 [100,599]", r.Status)
	}
	if len(r.Body) == 0 {
		r.Body = json.RawMessage("{}")
	}
	if r.Status == 0 {
		r.Status = 200
	}
	return &r, nil
}

// ---------------------------------------------------------------------------
// 校验辅助：顶层未知字段拒绝 + raw map 精确解析。
// ---------------------------------------------------------------------------

// ParseJSONObject 把钩子返回的 JSON 原文解析为顶层 map（供白名单校验）。
func ParseJSONObject(data json.RawMessage) (map[string]json.RawMessage, error) {
	if len(data) == 0 {
		return nil, gerror.New("返回为空")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, gerror.Newf("返回不是 JSON 对象: %v", err)
	}
	return m, nil
}

func rejectUnknownFields(raw map[string]json.RawMessage, allowed ...string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = struct{}{}
	}
	for k := range raw {
		if _, ok := allowedSet[k]; !ok {
			return gerror.Newf("未知字段 %q（契约 v1 不允许，检查拼写或升级 apiVersion）", k)
		}
	}
	return nil
}

func strictUnmarshal(raw map[string]json.RawMessage, dst any) error {
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
