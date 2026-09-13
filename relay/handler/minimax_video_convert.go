package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/qianfree/team-api/relay/common"
)

// MiniMax 官方视频协议（POST /v2/video_generation）入站转换层。
// 原生请求体解析/校验后转成任务框架通用请求体，复用任务提交管线
// （selectTaskChannel 渠道调度 + HandleTaskSubmit 计费/提交/落库）。
//
// 转换产物在保留原生字段（content[]/resolution/duration/ratio/aigc_watermark）之外，
// 双写通用词汇（prompt/seconds/images[]/metadata）——参数倍率规则的匹配源是归一化
// 任务体（「规则跟语义走不跟入口协议走」，见 internal/logic/billing/param_multiplier.go），
// 双写保证按通用路径配置的倍率规则对官方协议入站同样生效。

// minimaxVideoProtocol TaskRelayContext.Protocol 的 MiniMax 官方视频协议标识
const minimaxVideoProtocol = "minimax_native"

// minimaxVideoCreateRequest MiniMax 官方创建请求的归一形态（JSON 解析后的产物）
type minimaxVideoCreateRequest struct {
	Model         string
	Prompt        string // content[] 中首个非空 text 项（派生，供计费倍率匹配与审计）
	Content       []any
	Resolution    string
	Duration      int
	Ratio         string
	AigcWatermark *bool
}

// minimaxVideoRequestEcho 提交时落 PrivateData.request_echo 的请求回显，
// retrieve 时用于回显 prompt/resolution/duration/ratio（任务未轮询到上游数据前的兜底）
type minimaxVideoRequestEcho struct {
	Prompt     string `json:"prompt"`
	Resolution string `json:"resolution,omitempty"`
	Duration   int    `json:"duration,omitempty"`
	Ratio      string `json:"ratio,omitempty"`
}

// jsonMiniMaxVideoCreateRequest JSON 编码的官方创建请求结构。
// duration 兼容 number 与 string；content 保留 RawMessage 以校验数组形态。
type jsonMiniMaxVideoCreateRequest struct {
	Model         string          `json:"model"`
	Content       json.RawMessage `json:"content"`
	Resolution    string          `json:"resolution"`
	Duration      json.RawMessage `json:"duration"`
	Ratio         string          `json:"ratio"`
	AigcWatermark *bool           `json:"aigc_watermark"`
}

// ParseMiniMaxVideoCreateRequest 解析官方创建请求。
// 返回的 TaskError 已携带面向客户端的 HTTP 状态码与错误信息。
func ParseMiniMaxVideoCreateRequest(body []byte) (*minimaxVideoCreateRequest, *common.TaskError) {
	var req jsonMiniMaxVideoCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body: " + err.Error()}
	}

	out := &minimaxVideoCreateRequest{
		Model:         strings.TrimSpace(req.Model),
		Resolution:    strings.TrimSpace(req.Resolution),
		Ratio:         strings.TrimSpace(req.Ratio),
		AigcWatermark: req.AigcWatermark,
	}

	// content 必须是数组
	if len(req.Content) > 0 && string(req.Content) != "null" {
		var content []any
		if err := json.Unmarshal(req.Content, &content); err != nil {
			return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid content: expected an array"}
		}
		out.Content = content
	}

	// duration 兼容 number（官方 schema）与 string（宽松兼容）
	duration, terr := parseMiniMaxDuration(req.Duration)
	if terr != nil {
		return nil, terr
	}
	out.Duration = duration

	// 派生 prompt：content[] 中首个非空 text 项
	for _, item := range out.Content {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); t == "text" {
			if text, ok := m["text"].(string); ok && strings.TrimSpace(text) != "" {
				out.Prompt = text
				break
			}
		}
	}

	return out, nil
}

// parseMiniMaxDuration 解析 duration 字段（integer 或数字字符串，须为正数）
func parseMiniMaxDuration(raw json.RawMessage) (int, *common.TaskError) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		if d, err := strconv.Atoi(n.String()); err == nil {
			return d, nil
		}
		// 小数秒向下取整（官方档位均为整数，宽松处理）
		if f, err := strconv.ParseFloat(n.String(), 64); err == nil && f > 0 {
			return int(f), nil
		}
	}
	return 0, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid duration field: must be a positive integer"}
}

// ValidateMiniMaxVideoCreateRequest 请求必填校验（对齐官方 schema required：
// model / content（含非空 text 项）/ resolution / duration；数值档位差异交给上游裁决）
func ValidateMiniMaxVideoCreateRequest(req *minimaxVideoCreateRequest) *common.TaskError {
	if req.Model == "" {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	if len(req.Content) == 0 {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "content is required"}
	}
	if req.Prompt == "" {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "content must contain a non-empty text item"}
	}
	if req.Resolution == "" {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "resolution is required"}
	}
	if req.Duration <= 0 {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "duration is required"}
	}
	return nil
}

// BuildMiniMaxVideoTaskBody 把官方创建请求转成任务框架通用请求体（JSON 字节）。
// callback_url 不进入通用任务体（网关轮询计费，上游回调与网关 ID 体系不兼容）。
func BuildMiniMaxVideoTaskBody(req *minimaxVideoCreateRequest) ([]byte, error) {
	body := map[string]any{
		"model":      req.Model,
		"prompt":     req.Prompt,
		"content":    req.Content,
		"resolution": req.Resolution,
		"seconds":    strconv.Itoa(req.Duration), // 通用词汇双写（kling/volcengine 消费同一键）
		"duration":   req.Duration,
		"metadata": map[string]any{
			"resolution": req.Resolution,
			"duration":   req.Duration,
		},
	}
	if req.Ratio != "" {
		body["ratio"] = req.Ratio
		body["metadata"].(map[string]any)["ratio"] = req.Ratio
	}
	if req.AigcWatermark != nil {
		body["aigc_watermark"] = *req.AigcWatermark
	}
	// images[] 双写：content 中首帧/无 role 的 image_url 项提取 URL（图生视频判定与参数倍率用）
	if images := extractMiniMaxFrameImages(req.Content); len(images) > 0 {
		body["images"] = images
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal minimax video task body: %w", err)
	}
	return data, nil
}

// extractMiniMaxFrameImages 从 content[] 提取图片 URL（role=first_frame 或未标 role 的单图）
func extractMiniMaxFrameImages(content []any) []string {
	images := make([]string, 0, 2)
	for _, item := range content {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := m["type"].(string); t != "image_url" {
			continue
		}
		role, _ := m["role"].(string)
		if role != "" && role != "first_frame" {
			continue
		}
		if media, ok := m["image_url"].(map[string]any); ok {
			if url, ok := media["url"].(string); ok && url != "" {
				images = append(images, url)
			}
		}
	}
	return images
}

// NewMiniMaxRequestEcho 从归一请求构造提交回显（胶水层经 TaskRelayContext.RequestEcho 传入，
// 提交时随 PrivateData 落库，retrieve 时在上游数据未就绪前回显）
func NewMiniMaxRequestEcho(req *minimaxVideoCreateRequest) minimaxVideoRequestEcho {
	return minimaxVideoRequestEcho{
		Prompt:     req.Prompt,
		Resolution: req.Resolution,
		Duration:   req.Duration,
		Ratio:      req.Ratio,
	}
}

// ==================== v1（Hailuo 系列）官方协议入站 ====================

// minimaxVideoV1Protocol TaskRelayContext.Protocol 的 MiniMax v1 官方视频协议标识
const minimaxVideoV1Protocol = "minimax_native_v1"

// minimaxVideoV1CreateRequest v1 官方创建请求的归一形态（扁平字段协议，无 content[]）。
// resolution/duration 官方可选（有默认值），零值表示未指定。
type minimaxVideoV1CreateRequest struct {
	Model            string
	Prompt           string
	FirstFrameImage  string
	LastFrameImage   string
	SubjectReference json.RawMessage
	Duration         int
	Resolution       string
	PromptOptimizer  *bool
	FastPretreatment *bool
	AigcWatermark    *bool
}

// minimaxVideoV1RequestEcho v1 提交回显（任务未轮询到上游数据前的兜底）
type minimaxVideoV1RequestEcho struct {
	Prompt     string `json:"prompt"`
	Resolution string `json:"resolution,omitempty"`
	Duration   int    `json:"duration,omitempty"`
}

// ParseMiniMaxVideoV1CreateRequest 解析 v1 官方创建请求（POST /v1/video_generation，扁平字段）。
// 文生/图生/首尾帧/主体参考四种形态共用端点，按字段自然区分，duration 兼容 number 与 string。
func ParseMiniMaxVideoV1CreateRequest(body []byte) (*minimaxVideoV1CreateRequest, *common.TaskError) {
	var raw struct {
		Model            string          `json:"model"`
		Prompt           string          `json:"prompt"`
		FirstFrameImage  string          `json:"first_frame_image"`
		LastFrameImage   string          `json:"last_frame_image"`
		SubjectReference json.RawMessage `json:"subject_reference"`
		Duration         json.RawMessage `json:"duration"`
		Resolution       string          `json:"resolution"`
		PromptOptimizer  *bool           `json:"prompt_optimizer"`
		FastPretreatment *bool           `json:"fast_pretreatment"`
		AigcWatermark    *bool           `json:"aigc_watermark"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body: " + err.Error()}
	}

	duration, terr := parseMiniMaxDuration(raw.Duration)
	if terr != nil {
		return nil, terr
	}

	return &minimaxVideoV1CreateRequest{
		Model:            strings.TrimSpace(raw.Model),
		Prompt:           raw.Prompt,
		FirstFrameImage:  strings.TrimSpace(raw.FirstFrameImage),
		LastFrameImage:   strings.TrimSpace(raw.LastFrameImage),
		SubjectReference: raw.SubjectReference,
		Duration:         duration,
		Resolution:       strings.TrimSpace(raw.Resolution),
		PromptOptimizer:  raw.PromptOptimizer,
		FastPretreatment: raw.FastPretreatment,
		AigcWatermark:    raw.AigcWatermark,
	}, nil
}

// ValidateMiniMaxVideoV1CreateRequest 请求必填校验（对齐官方四形态约束）：
// model 必填；prompt（文生）/ first_frame_image（图生）/ subject_reference（主体参考）至少其一。
func ValidateMiniMaxVideoV1CreateRequest(req *minimaxVideoV1CreateRequest) *common.TaskError {
	if req.Model == "" {
		return &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	if strings.TrimSpace(req.Prompt) == "" && req.FirstFrameImage == "" && len(req.SubjectReference) == 0 {
		return &common.TaskError{StatusCode: http.StatusBadRequest,
			Message: "one of prompt / first_frame_image / subject_reference is required"}
	}
	return nil
}

// BuildMiniMaxVideoV1TaskBody 把 v1 官方创建请求转成任务框架通用请求体。
// callback_url 不进入任务体（网关轮询计费，上游回调与网关 ID 体系不兼容）；
// 同时双写通用词汇（seconds/images[]/metadata），保证按通用路径配置的参数倍率规则对 v1 入站同样生效。
func BuildMiniMaxVideoV1TaskBody(req *minimaxVideoV1CreateRequest) ([]byte, error) {
	body := map[string]any{
		"model": req.Model,
	}
	if strings.TrimSpace(req.Prompt) != "" {
		body["prompt"] = req.Prompt
	}
	if req.FirstFrameImage != "" {
		body["first_frame_image"] = req.FirstFrameImage
		body["images"] = []string{req.FirstFrameImage}
	}
	if req.LastFrameImage != "" {
		body["last_frame_image"] = req.LastFrameImage
	}
	if len(req.SubjectReference) > 0 && string(req.SubjectReference) != "null" {
		body["subject_reference"] = req.SubjectReference
	}
	if req.Resolution != "" {
		body["resolution"] = req.Resolution
	}
	if req.Duration > 0 {
		body["duration"] = req.Duration
		body["seconds"] = strconv.Itoa(req.Duration)
	}
	if req.PromptOptimizer != nil {
		body["prompt_optimizer"] = *req.PromptOptimizer
	}
	if req.FastPretreatment != nil {
		body["fast_pretreatment"] = *req.FastPretreatment
	}
	if req.AigcWatermark != nil {
		body["aigc_watermark"] = *req.AigcWatermark
	}

	// 通用词汇双写（metadata 键）
	metadata := map[string]any{}
	if req.Resolution != "" {
		metadata["resolution"] = req.Resolution
	}
	if req.Duration > 0 {
		metadata["duration"] = req.Duration
	}
	if req.FirstFrameImage != "" {
		metadata["image"] = req.FirstFrameImage
	}
	if len(metadata) > 0 {
		body["metadata"] = metadata
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal minimax v1 video task body: %w", err)
	}
	return data, nil
}

// NewMiniMaxV1RequestEcho 从归一请求构造 v1 提交回显
func NewMiniMaxV1RequestEcho(req *minimaxVideoV1CreateRequest) minimaxVideoV1RequestEcho {
	return minimaxVideoV1RequestEcho{
		Prompt:     req.Prompt,
		Resolution: req.Resolution,
		Duration:   req.Duration,
	}
}
