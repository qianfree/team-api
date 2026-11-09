package minimax

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"
)

// 标准端点参考素材（通用任务体 references[]，role 词汇与 MiniMax v2 content[].role 一致）。
// 与 hailuo JS 插件同口径：v2（H3 家族）按原序映射为带 role 的 content 项；v1（Hailuo 2.x/01）
// 只承载首/尾帧，S2V-01 的单张参考图映射为 subject_reference。网关已做协议层校验。

const (
	h3MaxRefTotal  = 12 // 官方：混合参考合计 ≤12
	h3MaxRefImages = 9
	h3MaxRefVideos = 3
	h3MaxRefAudios = 3
)

func refError(code, format string, args ...any) *common.TaskError {
	return &common.TaskError{StatusCode: http.StatusBadRequest, Message: fmt.Sprintf(format, args...), ErrCode: code}
}

// h3RequiresExplicitRatio H3-Lite 参考模式不支持 adaptive，须显式比例
func h3RequiresExplicitRatio(model string) bool {
	return strings.EqualFold(strings.TrimSpace(model), "MiniMax-H3-Lite")
}

// validateReferences MiniMax 模型层约束。
func validateReferences(model string, refs []common.VideoReference, ratio string) *common.TaskError {
	counts := map[string]int{}
	for i, ref := range refs {
		if common.ReferenceKind(ref.Role) == "" {
			return refError("invalid_references", "references[%d].role %q is not supported", i, ref.Role)
		}
		counts[ref.Role]++
	}
	references := counts[common.RefRoleReferenceImage] + counts[common.RefRoleReferenceVideo] + counts[common.RefRoleReferenceAudio]

	if !IsV2Model(model) {
		// v1：首/尾帧；S2V-01 允许单张 reference_image（主体参考）
		allowedImages := 0
		if strings.EqualFold(strings.TrimSpace(model), "S2V-01") {
			allowedImages = 1
		}
		if counts[common.RefRoleReferenceVideo] > 0 || counts[common.RefRoleReferenceAudio] > 0 ||
			counts[common.RefRoleReferenceImage] > allowedImages {
			return refError(common.ErrCodeReferenceNotSupported,
				"model %q supports only first_frame/last_frame references", model)
		}
		return nil
	}

	if counts[common.RefRoleReferenceImage] > h3MaxRefImages {
		return refError("invalid_references", "at most %d reference images", h3MaxRefImages)
	}
	if counts[common.RefRoleReferenceVideo] > h3MaxRefVideos {
		return refError("invalid_references", "at most %d reference videos", h3MaxRefVideos)
	}
	if counts[common.RefRoleReferenceAudio] > h3MaxRefAudios {
		return refError("invalid_references", "at most %d reference audios", h3MaxRefAudios)
	}
	if references > h3MaxRefTotal {
		return refError("invalid_references", "at most %d references in total", h3MaxRefTotal)
	}
	if references > 0 && h3RequiresExplicitRatio(model) && (ratio == "" || strings.EqualFold(ratio, "adaptive")) {
		return refError("invalid_references", "model %q requires an explicit aspect_ratio (not adaptive) in reference mode", model)
	}
	return nil
}

// referenceContent references → v2 content 媒体项（保持原序）
func referenceContent(refs []common.VideoReference) []map[string]any {
	items := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		var typ string
		switch common.ReferenceKind(ref.Role) {
		case common.RefKindImage:
			typ = "image_url"
		case common.RefKindVideo:
			typ = "video_url"
		case common.RefKindAudio:
			typ = "audio_url"
		default:
			continue
		}
		items = append(items, map[string]any{
			"type": typ,
			typ:    map[string]any{"url": ref.URL},
			"role": ref.Role,
		})
	}
	return items
}

// referenceFrames references 中的首/尾帧与（S2V-01）主体参考图
func referenceFrames(refs []common.VideoReference) (first, last, subject string) {
	for _, ref := range refs {
		switch ref.Role {
		case common.RefRoleFirstFrame:
			first = ref.URL
		case common.RefRoleLastFrame:
			last = ref.URL
		case common.RefRoleReferenceImage:
			if subject == "" {
				subject = ref.URL
			}
		}
	}
	return first, last, subject
}
