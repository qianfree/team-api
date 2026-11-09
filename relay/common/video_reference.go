package common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// 视频参考素材（OpenAI Videos 协议扩展字段 references[]，见
// docs/design/OpenAI-Videos协议扩展-参考素材设计方案-v1.md）。
// role 词汇直接采用 Seedance 2.0 与 MiniMax H3 共同的 content[].role 枚举，
// 下游映射近乎恒等；提示词「图片N/视频N/音频N」按同类素材在列表中的顺序编号。

// 参考素材角色
const (
	RefRoleFirstFrame     = "first_frame"
	RefRoleLastFrame      = "last_frame"
	RefRoleReferenceImage = "reference_image"
	RefRoleReferenceVideo = "reference_video"
	RefRoleReferenceAudio = "reference_audio"
)

// 参考素材类别（role → 媒体主类型）
const (
	RefKindImage = "image"
	RefKindVideo = "video"
	RefKindAudio = "audio"
)

// ErrCodeReferenceNotSupported 目标模型不支持该参考素材角色（插件/适配器抛出）
const ErrCodeReferenceNotSupported = "reference_not_supported"

// VideoReference 单个参考素材。通用任务体内（网关 → 插件/适配器）恒为 URL 形态：
// file_id 已由网关解析为对象存储签名 URL。
type VideoReference struct {
	Role   string `json:"role"`
	URL    string `json:"url,omitempty"`
	FileID string `json:"file_id,omitempty"`
}

// ReferenceKind role 对应的媒体类别；未知 role 返回空串。
func ReferenceKind(role string) string {
	switch role {
	case RefRoleFirstFrame, RefRoleLastFrame, RefRoleReferenceImage:
		return RefKindImage
	case RefRoleReferenceVideo:
		return RefKindVideo
	case RefRoleReferenceAudio:
		return RefKindAudio
	default:
		return ""
	}
}

// IsFrameRole 是否首/尾帧角色（与 reference_* 多模态参考互斥）。
func IsFrameRole(role string) bool {
	return role == RefRoleFirstFrame || role == RefRoleLastFrame
}

// ReferencesFromTaskBody 读取通用任务体的 references[]（已解析的 map 形态）。
// 非数组 / 元素非对象 / 缺 role 或 url 的项跳过——任务体由网关写入、已校验，
// 此处只做防御性读取。
func ReferencesFromTaskBody(req map[string]any) []VideoReference {
	raw, ok := req["references"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]VideoReference, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		url, _ := m["url"].(string)
		role, url = strings.TrimSpace(role), strings.TrimSpace(url)
		if role == "" || url == "" {
			continue
		}
		out = append(out, VideoReference{Role: role, URL: url})
	}
	return out
}

// ReferencesFromTaskBodyBytes 同 ReferencesFromTaskBody，入参为 JSON 字节。
func ReferencesFromTaskBodyBytes(body []byte) []VideoReference {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}
	return ReferencesFromTaskBody(req)
}

// RejectUnsupportedReferences 适配器声明自身支持的 role，任务体含其余 role 即拒绝
// （不静默丢弃）。未升级 references 的老视频适配器在 ValidateRequest 中调用，
// 通常只声明 first_frame（兼容字段 images[]/metadata.image 承载）。
func RejectUnsupportedReferences(refs []VideoReference, model string, allowed ...string) *TaskError {
	for _, ref := range refs {
		supported := false
		for _, role := range allowed {
			if ref.Role == role {
				supported = true
				break
			}
		}
		if !supported {
			return &TaskError{
				StatusCode: http.StatusBadRequest,
				Message:    fmt.Sprintf("reference role %q is not supported by model %q", ref.Role, model),
				ErrCode:    ErrCodeReferenceNotSupported,
			}
		}
	}
	return nil
}
