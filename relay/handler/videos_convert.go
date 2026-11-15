package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/qianfree/team-api/relay/common"
)

// OpenAI Videos 协议（/v1/videos）入站转换层。
// 官方 SDK（openai-python / openai-node）对 POST /v1/videos 一律发送 multipart/form-data，
// 裸 HTTP 客户端常用 JSON——两种编码都要支持；解析后统一转成任务框架的通用请求体
// {model, prompt, seconds, ratio, images, references, metadata{duration, size, ratio, aspect_ratio, sound, generate_audio, image, last_frame}}，
// 由各 taskchannel 适配器按需消费（kling 读顶层 seconds，volcengine 读 metadata.duration 等）。
// size / aspect_ratio 为协议扩展字段（官方 Videos 协议无此两项），取值直接透传目标
// 供应商的原生词汇，不做档位换算与白名单校验。参考素材统一走 references[]（首帧用
// role first_frame）；厂商音频开关等私有参数走 extra 透传（可灵顶层 sound、火山顶层
// generate_audio、阿里 parameters.audio 均按上游原生路径平铺送达）。

// videosMultipartMaxMemory multipart 表单解析的内存缓冲上限。该端点不收文件 part
// （大素材走 /v1/files 上传、以 references[].file_id 引用），表单只承载文本字段
// （references / extra 为 JSON 字符串，内联 base64 参考图时仍可达 MB 级）。
const videosMultipartMaxMemory = 6 << 20

// videosCreateRequest OpenAI Videos 创建请求的归一形态（multipart/JSON 解析后的产物）
type videosCreateRequest struct {
	Model       string
	Prompt      string
	Seconds     string                  // 统一归一为字符串（官方协议为 "4"/"8"/"12"）；空 = 未指定
	Size        string                  // 原样保留（如 "1280x720"）；空 = 未指定
	References  []common.VideoReference // 参考素材（首帧 = role first_frame；file_id 已解析为 URL）
	AspectRatio string                  // 屏幕比例（如 "16:9"/"9:16"）；空 = 未指定（由适配器取各自默认档）
	Resolution  string                  // 分辨率档位（厂商原生词汇，如 "720P"/"2K"）；空 = 未指定。通用任务端点的收敛词汇
	// Sound 音频开关（"on"/"off"）。仅 /v1/video/generations 收敛词汇写入；
	// /v1/videos 已移除 sound/generate_audio 字段，厂商音频参数走 extra 透传
	Sound string
	// Watermark 水印开关。仅 /v1/video/generations 收敛词汇写入；
	// /v1/videos 入口恒为 nil（官方协议无此字段，厂商水印继续走 extra 透传）
	Watermark *bool
	// VendorMetadata 厂商专属 metadata 键值（通用任务端点收敛后保留的白名单通道：
	// kling mode/cfg_scale/camera_control、火山 seed/watermark 等），原样并入任务体
	// metadata；/v1/videos 入口不解析客户端 metadata，恒为 nil
	VendorMetadata map[string]json.RawMessage
	// Extra 扩展字段：厂商私有参数（已校验为 JSON 对象，原始字节保真），随任务体顶层 extra 下发，
	// 由任务管线平铺深合并进上游请求体（见 common.MergeTaskExtra）；nil = 未传
	Extra json.RawMessage
}

// videosRequestEcho 提交时落 PrivateData.request_echo 的请求回显，
// retrieve（GET /v1/videos/{id}）时用于回显 prompt/seconds/size（上游响应不含这些字段）。
type videosRequestEcho struct {
	Prompt      string `json:"prompt"`
	Seconds     string `json:"seconds,omitempty"`
	Size        string `json:"size,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
}

// videosCreateTopLevelDeprecated /v1/videos 已移除的顶层字段 → 替代指引。
// 参考图（含首帧）统一走 references[]，厂商音频/水印开关等私有参数统一走 extra 透传
// （/v1/video/generations 有顶层 watermark 收敛词汇，本端点保持 OpenAI 协议纯净不设同名
// 顶层字段，命中即 400 引导走 extra）；命中即 400（对齐 /v1/video/generations 的收敛
// 闸门），不静默忽略——静默丢弃首帧图会把图生视频当文生视频、按请求计费按默认档生成。
var videosCreateTopLevelDeprecated = map[string]string{
	"input_reference": "use references[] with role first_frame",
	"sound":           "use extra (vendor passthrough, e.g. kling sound / doubao generate_audio / wan3 parameters.audio)",
	"generate_audio":  "use extra (vendor passthrough, e.g. doubao generate_audio)",
	"watermark":       "use extra (vendor passthrough, e.g. doubao/wan watermark, minimax aigc_watermark, kling watermark_info; the /v1/video/generations endpoint accepts a top-level watermark)",
}

// jsonVideosCreateRequest JSON 编码的创建请求结构。
type jsonVideosCreateRequest struct {
	Model       string          `json:"model"`
	Prompt      string          `json:"prompt"`
	Seconds     json.RawMessage `json:"seconds"` // string 或 number
	Size        string          `json:"size"`
	AspectRatio string          `json:"aspect_ratio"`
	References  json.RawMessage `json:"references"` // 扩展：参考素材数组（或其 JSON 字符串）
	Extra       json.RawMessage `json:"extra"`      // 扩展：透传上游的厂商私有参数对象（或其 JSON 字符串）
}

// ParseVideosCreateRequest 解析 OpenAI Videos 创建请求（multipart 或 JSON）。
// 返回的 TaskError 已携带面向客户端的 HTTP 状态码与错误信息。
func ParseVideosCreateRequest(body []byte, contentType string) (*videosCreateRequest, *common.TaskError) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentType // 非法 Content-Type 按 JSON 尝试解析
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		return parseVideosCreateMultipart(body, params["boundary"])
	}
	return parseVideosCreateJSON(body)
}

// parseVideosCreateJSON 解析 JSON 编码的创建请求
func parseVideosCreateJSON(body []byte) (*videosCreateRequest, *common.TaskError) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body: " + err.Error()}
	}
	// 已移除字段黑名单：命中即 400（替代指引内嵌在 message）
	for field, hint := range videosCreateTopLevelDeprecated {
		if _, ok := raw[field]; ok {
			return nil, &common.TaskError{
				StatusCode: http.StatusBadRequest,
				Message:    "field \"" + field + "\" is not accepted on this endpoint; " + hint,
				ErrCode:    "field_deprecated",
			}
		}
	}

	var req jsonVideosCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body: " + err.Error()}
	}

	out := &videosCreateRequest{
		Model:       strings.TrimSpace(req.Model),
		Prompt:      req.Prompt,
		Size:        strings.TrimSpace(req.Size),
		AspectRatio: strings.TrimSpace(req.AspectRatio),
	}

	seconds, terr := normalizeVideosSeconds(req.Seconds)
	if terr != nil {
		return nil, terr
	}
	out.Seconds = seconds

	refs, terr := parseVideosReferences(req.References)
	if terr != nil {
		return nil, terr
	}
	out.References = refs

	if out.Extra, terr = parseVideosExtra(req.Extra); terr != nil {
		return nil, terr
	}

	return out, nil
}

// parseVideosCreateMultipart 解析 multipart/form-data 编码的创建请求（官方 SDK 形态）。
func parseVideosCreateMultipart(body []byte, boundary string) (*videosCreateRequest, *common.TaskError) {
	if boundary == "" {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "missing multipart boundary"}
	}

	form, err := multipart.NewReader(bytes.NewReader(body), boundary).ReadForm(videosMultipartMaxMemory)
	if err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid multipart form: " + err.Error()}
	}
	defer form.RemoveAll()

	formValue := func(name string) string {
		if vals := form.Value[name]; len(vals) > 0 {
			return vals[0]
		}
		return ""
	}

	// 已移除字段黑名单：对象形态经 brackets 编码的 part / 文件 part 一并拦截
	deprecatedFormFields := map[string]string{
		"input_reference":            videosCreateTopLevelDeprecated["input_reference"],
		"input_reference[image_url]": videosCreateTopLevelDeprecated["input_reference"],
		"input_reference[file_id]":   videosCreateTopLevelDeprecated["input_reference"],
		"sound":                      videosCreateTopLevelDeprecated["sound"],
		"generate_audio":             videosCreateTopLevelDeprecated["generate_audio"],
		"watermark":                  videosCreateTopLevelDeprecated["watermark"],
	}
	for name, hint := range deprecatedFormFields {
		if formValue(name) != "" {
			return nil, &common.TaskError{
				StatusCode: http.StatusBadRequest,
				Message:    "field \"" + name + "\" is not accepted on this endpoint; " + hint,
				ErrCode:    "field_deprecated",
			}
		}
	}
	if _, ok := form.File["input_reference"]; ok {
		return nil, &common.TaskError{
			StatusCode: http.StatusBadRequest,
			Message:    "field \"input_reference\" is not accepted on this endpoint; " + videosCreateTopLevelDeprecated["input_reference"],
			ErrCode:    "field_deprecated",
		}
	}

	out := &videosCreateRequest{
		Model:       strings.TrimSpace(formValue("model")),
		Prompt:      formValue("prompt"),
		Size:        strings.TrimSpace(formValue("size")),
		AspectRatio: strings.TrimSpace(formValue("aspect_ratio")),
	}

	seconds, terr := normalizeVideosSeconds(json.RawMessage(formValue("seconds")))
	if terr != nil {
		return nil, terr
	}
	out.Seconds = seconds

	// references 扩展字段：multipart 下为 JSON 字符串（对象列表经 key[] 编码无法还原分组）
	refs, terr := parseVideosReferences(multipartJSONString(formValue("references")))
	if terr != nil {
		return nil, terr
	}
	out.References = refs

	// extra 扩展字段：multipart 下为 JSON 字符串（与 references 同口径）
	if out.Extra, terr = parseVideosExtra(multipartJSONString(formValue("extra"))); terr != nil {
		return nil, terr
	}

	return out, nil
}

// parseVideosExtra 解析 extra：JSON 对象，或内容为 JSON 对象的字符串
// （multipart 恒为字符串；官方 SDK 经 extra_body 传入时也可能被 json.dumps）。
// 只做形态校验，保护键校验由任务管线按目标渠道执行。返回原始字节以保真数字精度
func parseVideosExtra(raw json.RawMessage) (json.RawMessage, *common.TaskError) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid extra: " + err.Error(), ErrCode: common.ErrCodeInvalidExtra}
		}
		trimmed = bytes.TrimSpace([]byte(s))
		if len(trimmed) > 0 && !json.Valid(trimmed) {
			return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid extra: expected a JSON object", ErrCode: common.ErrCodeInvalidExtra}
		}
	}
	extra, terr := common.ParseTaskExtra(trimmed)
	if terr != nil || extra == nil {
		return nil, terr
	}
	return trimmed, nil
}

// normalizeVideosSeconds 归一 seconds 字段：接受字符串（官方形态）或数字（宽松兼容），统一转字符串。
// 不做 4/8/12 白名单校验——第三方视频模型时长档位各异，透传给适配器自行对齐。
func normalizeVideosSeconds(raw json.RawMessage) (string, *common.TaskError) {
	if len(raw) == 0 {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s), nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		if _, convErr := strconv.ParseFloat(n.String(), 64); convErr == nil {
			return n.String(), nil
		}
	}
	return "", &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid seconds field: must be a string or number"}
}

// multipartJSONString 把 multipart part 的裸字符串包成 JSON 字符串字面量，
// 空值返回 nil（视为未传）。这样 multipart 与 JSON 两种编码共用同一套归一逻辑。
func multipartJSONString(v string) json.RawMessage {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}

// normalizeVideosSound 归一音频开关：接受 "on"/"off"、"true"/"false"、"1"/"0"
// 以及布尔字面量，统一转成 "on"/"off"（可灵的上游词汇）。空 = 未指定。
// 不做厂商白名单校验——各上游对音频开关的词汇与默认值不同，归一后由适配器按需消费。
func normalizeVideosSound(raw json.RawMessage) (string, *common.TaskError) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}

	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		if b {
			return "on", nil
		}
		return "off", nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "":
			return "", nil
		case "on", "true", "1", "yes":
			return "on", nil
		case "off", "false", "0", "no":
			return "off", nil
		}
	}

	return "", &common.TaskError{
		StatusCode: http.StatusBadRequest,
		Message:    `invalid sound field: must be "on"/"off" or true/false`,
	}
}

// normalizeVideosWatermark 归一水印开关：接受布尔字面量以及 "on"/"off"、"true"/"false"、
// "1"/"0" 宽松词汇（口径对齐 normalizeVideosSound），统一转成 *bool（nil = 未指定）。
// 各上游水印词汇不同（火山/阿里 watermark、MiniMax aigc_watermark、可灵 watermark_info），
// 统一走 metadata.watermark 由适配器映射。
func normalizeVideosWatermark(raw json.RawMessage) (*bool, *common.TaskError) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}

	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return &b, nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "":
			return nil, nil
		case "on", "true", "1", "yes":
			v := true
			return &v, nil
		case "off", "false", "0", "no":
			v := false
			return &v, nil
		}
	}

	return nil, &common.TaskError{
		StatusCode: http.StatusBadRequest,
		Message:    `invalid watermark field: must be true/false or "on"/"off"`,
	}
}

// knownVendorExtraKeys extra 中网关「认识」的厂商参数键（metadata/extra 合并后的唯一
// 厂商通道 extra 的已知键集）：提交前提升进任务体 metadata（适配器按各自白名单消费、
// 计费联动如可灵 mode），并从 extra 剔除——防止嵌套上游（阿里 input/parameters）被
// 平铺出错误位置的重复字段。未列出的键一律按上游原生路径平铺深合并（extra 原语义）。
// sound / generate_audio 为音频开关（/v1/videos 已移除同名顶层字段，厂商音频控制唯一
// 入口即 extra）：提升进 metadata 后可灵（Go 适配器与 JS 插件均读 metadata.sound）、
// 火山/豆包（metadata.generate_audio）、阿里 wan3（metadata.sound）按现有路径消费
var knownVendorExtraKeys = []string{
	"mode",              // 可灵档位（std/pro，计费 3.5x 联动）
	"negative_prompt",   // 可灵/阿里反向提示词
	"cfg_scale",         // 可灵提示词相关性
	"camera_control",    // 可灵运镜控制
	"image_tail",        // 可灵尾帧（原生词汇；通用词汇走 references[].last_frame）
	"seed",              // 火山/阿里随机种子
	"watermark",         // 火山/阿里水印开关
	"return_last_frame", // 火山尾帧图回传开关
	"audio_url",         // 阿里参考音频
	"prompt_extend",     // 阿里提示词改写开关
	"sound",             // 音频开关（"on"/"off"，可灵/阿里词汇；已移除顶层字段的替代通道）
	"generate_audio",    // 音频开关（bool，火山/豆包词汇；已移除顶层字段的替代通道）
}

// promoteKnownVendorKeys 把 extra 中的已知厂商键提升进 VendorMetadata 并从 extra 剔除。
// 就地改写 req（Extra / VendorMetadata），无已知键时不动
func promoteKnownVendorKeys(req *videosCreateRequest) *common.TaskError {
	if len(req.Extra) == 0 {
		return nil
	}
	extra, terr := common.ParseTaskExtra(req.Extra)
	if terr != nil || len(extra) == 0 {
		return terr
	}
	moved := false
	for _, key := range knownVendorExtraKeys {
		v, ok := extra[key]
		if !ok {
			continue
		}
		raw, err := json.Marshal(v)
		if err != nil {
			return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid extra: " + key, ErrCode: common.ErrCodeInvalidExtra}
		}
		if req.VendorMetadata == nil {
			req.VendorMetadata = map[string]json.RawMessage{}
		}
		req.VendorMetadata[key] = raw
		delete(extra, key)
		moved = true
	}
	if !moved {
		return nil
	}
	if len(extra) == 0 {
		req.Extra = nil
		return nil
	}
	out, err := json.Marshal(extra)
	if err != nil {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid extra", ErrCode: common.ErrCodeInvalidExtra}
	}
	req.Extra = out
	return nil
}

// BuildVideosTaskBody 把归一后的创建请求转成任务框架通用请求体（JSON 字节）。
// 顶层保留 seconds + ratio + images，metadata 写入 duration/size/ratio/aspect_ratio/sound/generate_audio/image。
// size 只借 OpenAI 协议的字段形态、不做任何档位换算：调用方直接传目标供应商的
// 原生分辨率词汇（如 MiniMax 的 768P/2K），原值经 metadata.size 透传给适配器。
func BuildVideosTaskBody(req *videosCreateRequest) ([]byte, error) {
	// extra 单厂商通道：已知键提升进 metadata（两个入口共用），未知键留 extra 平铺
	if terr := promoteKnownVendorKeys(req); terr != nil {
		return nil, terr
	}

	body := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
	}
	metadata := map[string]any{}

	if req.Seconds != "" {
		body["seconds"] = req.Seconds
		if d, err := strconv.Atoi(req.Seconds); err == nil {
			metadata["duration"] = d
		}
	}
	if req.Size != "" {
		metadata["size"] = req.Size
	}
	if req.Resolution != "" {
		// 分辨率档位（通用任务端点收敛词汇）：火山/MiniMax/Gemini 读 metadata.resolution，
		// 值为厂商原生档位词汇，不做换算
		metadata["resolution"] = req.Resolution
	}
	for k, v := range req.VendorMetadata {
		// 厂商专属键并入（通用键已由上方归一写入，调用方已保证无冲突）
		metadata[k] = v
	}
	if req.AspectRatio != "" {
		// 比例双写：minimax/volcengine 读 metadata.ratio，kling 读 metadata.aspect_ratio，
		// minimax 的 resolveRatio 顶层 ratio 优先级最高，故三处同写
		metadata["ratio"] = req.AspectRatio
		metadata["aspect_ratio"] = req.AspectRatio
		body["ratio"] = req.AspectRatio
	}
	if req.Sound != "" {
		// 音频开关双写：kling 读 metadata.sound（"on"/"off"），volcengine 读 metadata.generate_audio（bool）
		metadata["sound"] = req.Sound
		metadata["generate_audio"] = req.Sound == "on"
	}
	if req.Watermark != nil {
		// 水印开关：volcengine/ali 读 metadata.watermark；minimax/kling 由各自适配器映射到
		// aigc_watermark / watermark_info。写在 promoteKnownVendorKeys 之后，顶层收敛词汇
		// 覆盖 extra 提升值（一致性由解析层冲突检查保证）
		metadata["watermark"] = *req.Watermark
	}
	// 参考素材：references[] 为唯一权威来源（首帧经 role first_frame；file_id 已解析为
	// URL，顺序即提示词编号依据）；images[]/metadata.image/metadata.last_frame 为兼容
	// 字段，只承载首/尾帧，供未升级 references 的老适配器沿用——reference_* 不写入
	// images[]（否则会被当作首帧）
	if refs := req.References; len(refs) > 0 {
		out := make([]map[string]string, 0, len(refs))
		for _, ref := range refs {
			out = append(out, map[string]string{"role": ref.Role, "url": ref.URL})
			switch ref.Role {
			case common.RefRoleFirstFrame:
				metadata["image"] = ref.URL
				// volcengine 图生视频读根级 images 数组（kling 亦有该兜底）
				body["images"] = []string{ref.URL}
			case common.RefRoleLastFrame:
				metadata["last_frame"] = ref.URL
			}
		}
		body["references"] = out
	}
	if len(metadata) > 0 {
		body["metadata"] = metadata
	}
	if len(req.Extra) > 0 {
		body[common.ExtraFieldName] = req.Extra
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal videos task body: %w", err)
	}
	return data, nil
}

// ValidateVideosCreateRequest 请求必填校验（model/prompt 官方均为必填）
func ValidateVideosCreateRequest(req *videosCreateRequest) *common.TaskError {
	if req.Model == "" {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "prompt is required"}
	}
	return validateVideosReferences(req)
}

// NewVideosRequestEcho 从归一请求构造提交回显（胶水层经 TaskRelayContext.RequestEcho 传入，
// 提交时随 PrivateData 落库，retrieve 时回显 prompt/seconds/size）
func NewVideosRequestEcho(req *videosCreateRequest) videosRequestEcho {
	return videosRequestEcho{
		Prompt:      req.Prompt,
		Seconds:     req.Seconds,
		Size:        req.Size,
		AspectRatio: req.AspectRatio,
	}
}
