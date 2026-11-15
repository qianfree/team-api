package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/qianfree/team-api/relay/common"
)

// /v1/video/generations 通用任务端点的参数收敛层。
// 客户端契约收敛为固定词汇集（9 字段）：
//
//	{model*, prompt*(可空串), duration(秒，-1=智能时长), resolution(档位词汇),
//	 aspect_ratio, sound("on"/"off"), watermark(bool), references[{role,url|file_id}],
//	 extra{厂商参数唯一通道}}
//
// 厂商参数不区分 metadata/extra 两个字段：网关认识的已知键（可灵 mode/cfg_scale/
// camera_control、火山 seed/watermark、阿里 audio_url 等，见 knownVendorExtraKeys）
// 直接写键名——提交前提升进任务体 metadata，由适配器映射到上游正确位置并联动计费；
// 未列出的键按上游原生路径平铺深合并透传。历史上的多层重复词汇（seconds/size/ratio/
// generate_audio/images[]/原生直传字段/metadata 字段本身）全部废弃：命中即 400 并给出
// 替代指引，不静默忽略——静默丢弃参考素材或时长会把图生视频当文生视频、按请求计费按
// 默认档生成，代价远高于明确报错。归一产物与 /v1/videos 同构（BuildVideosTaskBody），
// 适配器与 JS 插件解码钩子零改动。

// videoGenerationsTopLevelDeprecated 顶层废弃字段 → 替代指引
var videoGenerationsTopLevelDeprecated = map[string]string{
	"images":            "pass reference material via references[] (role first_frame / reference_image)",
	"seconds":           "use duration",
	"mode":              "put vendor parameters in extra (known keys are mapped and billed by the gateway)",
	"size":              "use resolution",
	"ratio":             "use aspect_ratio",
	"generate_audio":    "use sound",
	"first_frame_image": "use references[] with role first_frame",
	"last_frame_image":  "use references[] with role last_frame",
	"subject_reference": "use references[] with role reference_image",
	"media":             "use references[]",
	"content":           "use references[]",
	"prompt_optimizer":  "use extra",
	"fast_pretreatment": "use extra",
	"aigc_watermark":    "use watermark",
	"metadata":          "vendor parameters are merged into extra (known keys are promoted automatically)",
}

// jsonVideoGenerationsRequest 收敛后的客户端请求结构。
// Prompt 用指针区分「未传」与「空串」：必填但允许空串（纯图生视频场景）。
type jsonVideoGenerationsRequest struct {
	Model       string          `json:"model"`
	Prompt      *string         `json:"prompt"`
	Duration    json.RawMessage `json:"duration"`     // number 或数字字符串（秒，-1=智能时长）
	Resolution  string          `json:"resolution"`   // 厂商原生档位词汇
	AspectRatio string          `json:"aspect_ratio"` // 如 "16:9"
	Sound       json.RawMessage `json:"sound"`        // "on"/"off" 或 true/false
	Watermark   json.RawMessage `json:"watermark"`    // bool 或 "on"/"off"
	References  json.RawMessage `json:"references"`   // [{role, url|file_id}]
	Extra       json.RawMessage `json:"extra"`        // 厂商参数唯一通道（已知键提升 + 未知键透传）
}

// ParseVideoGenerationsRequest 校验并归一 /v1/video/generations 的客户端请求体。
// 返回的 videosCreateRequest 经 file_id 解析后由 BuildVideosTaskBody 转成通用任务体
// （与 /v1/videos 入口共用归一逻辑与参考素材校验）
func ParseVideoGenerationsRequest(body []byte) (*videosCreateRequest, *common.TaskError) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body: " + err.Error()}
	}
	// 废弃字段黑名单：命中即 400（替代指引内嵌在 message）
	for field, hint := range videoGenerationsTopLevelDeprecated {
		if _, ok := raw[field]; ok {
			return nil, &common.TaskError{
				StatusCode: http.StatusBadRequest,
				Message:    "field \"" + field + "\" is not accepted on this endpoint; " + hint,
				ErrCode:    "field_deprecated",
			}
		}
	}

	var req jsonVideoGenerationsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body: " + err.Error()}
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "model is required"}
	}
	if req.Prompt == nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "prompt is required"}
	}

	out := &videosCreateRequest{
		Model:       strings.TrimSpace(req.Model),
		Prompt:      *req.Prompt,
		Resolution:  strings.TrimSpace(req.Resolution),
		AspectRatio: strings.TrimSpace(req.AspectRatio),
	}

	// duration：number 或数字字符串，统一归一为 seconds 字符串（-1 保留给智能时长）。
	// 收敛层额外强制数值合法（normalizeVideosSeconds 沿 OpenAI 协议放行任意字符串）
	seconds, terr := normalizeVideosSeconds(req.Duration)
	if terr != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid duration field: must be a number (seconds)"}
	}
	if seconds != "" {
		if _, err := strconv.Atoi(seconds); err != nil {
			return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid duration field: must be an integer number of seconds"}
		}
	}
	out.Seconds = seconds

	// sound：与 /v1/videos 同口径（"on"/"off"、bool、"true"/"1" 等宽松归一）
	sound, terr := normalizeVideosSound(req.Sound)
	if terr != nil {
		return nil, terr
	}
	out.Sound = sound

	// watermark：bool 或宽松词汇归一（volcengine/ali/minimax/kling 均有水印通道）
	watermark, terr := normalizeVideosWatermark(req.Watermark)
	if terr != nil {
		return nil, terr
	}
	out.Watermark = watermark

	// references：复用 /v1/videos 的解析与校验（role 词汇、url/file_id 二选一、数量上限）
	refs, terr := parseVideosReferences(req.References)
	if terr != nil {
		return nil, terr
	}
	out.References = refs
	if terr := validateVideosReferences(out); terr != nil {
		return nil, terr
	}

	// extra：厂商参数唯一通道。形态校验复用 /v1/videos（对象或 JSON 字符串）；
	// 已知厂商键在 BuildVideosTaskBody 内提升进 metadata 并从 extra 剔除
	if out.Extra, terr = parseVideosExtra(req.Extra); terr != nil {
		return nil, terr
	}
	// 顶层 watermark 与 extra.watermark 同传时必须归一后一致——收敛词汇与逃生舱
	// 不静默覆盖（否则客户端无法确定哪个值生效）
	if out.Watermark != nil && len(out.Extra) > 0 {
		extraWM, terr := normalizeVideosWatermark(rawObjectKey(out.Extra, "watermark"))
		if terr != nil {
			return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: `invalid watermark in extra: must be true/false or "on"/"off"`}
		}
		if extraWM != nil && *extraWM != *out.Watermark {
			return nil, &common.TaskError{
				StatusCode: http.StatusBadRequest,
				Message:    "conflicting watermark values in watermark and extra",
				ErrCode:    "field_conflict",
			}
		}
	}
	return out, nil
}

// rawObjectKey 从 JSON 对象原始字节中取出指定键的值（不存在或解析失败返回 nil）
func rawObjectKey(raw json.RawMessage, key string) json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m[key]
}
