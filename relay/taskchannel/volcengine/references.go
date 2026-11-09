package volcengine

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"
)

// 标准端点参考素材（通用任务体 references[]，role 词汇与 Ark content[].role 一致）。
// 与 doubao JS 插件 referenceContent 同口径：按原序映射为带 role 的 content 项，
// 并校验 Seedance 模型层约束（网关已做协议层校验）。

const (
	seedanceMaxRefImages = 9
	seedanceMaxRefVideos = 3
	seedanceMaxRefAudios = 3
)

// supportsMultimodalReference Seedance 2.x 才支持 reference_image/video/audio
func supportsMultimodalReference(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "seedance-2-")
}

func refError(format string, args ...any) *common.TaskError {
	return &common.TaskError{StatusCode: http.StatusBadRequest, Message: fmt.Sprintf(format, args...), ErrCode: "invalid_references"}
}

// validateSeedanceReferences Seedance 模型层约束：多模态参考仅 2.x；图 ≤9、视频 ≤3、
// 音频 ≤3；音频须搭配图或视频；参考视频不收 base64；尾帧须搭配首帧。
func validateSeedanceReferences(model string, refs []common.VideoReference) *common.TaskError {
	counts := map[string]int{}
	multimodal := false
	for i, ref := range refs {
		if common.ReferenceKind(ref.Role) == "" {
			return refError("references[%d].role %q is not supported", i, ref.Role)
		}
		if !common.IsFrameRole(ref.Role) {
			multimodal = true
		}
		if ref.Role == common.RefRoleReferenceVideo && strings.HasPrefix(strings.ToLower(ref.URL), "data:") {
			return refError("references[%d]: reference_video does not accept base64 data URLs; upload via /v1/files and use file_id, or pass a public URL", i)
		}
		counts[ref.Role]++
	}
	if multimodal && !supportsMultimodalReference(model) {
		return &common.TaskError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("model %q does not support reference_image/reference_video/reference_audio (Seedance 2.x only)", model),
			ErrCode:    common.ErrCodeReferenceNotSupported,
		}
	}
	if counts[common.RefRoleLastFrame] > 0 && counts[common.RefRoleFirstFrame] == 0 {
		return refError("last_frame requires a first_frame")
	}
	if counts[common.RefRoleReferenceImage] > seedanceMaxRefImages {
		return refError("at most %d reference images", seedanceMaxRefImages)
	}
	if counts[common.RefRoleReferenceVideo] > seedanceMaxRefVideos {
		return refError("at most %d reference videos", seedanceMaxRefVideos)
	}
	if counts[common.RefRoleReferenceAudio] > seedanceMaxRefAudios {
		return refError("at most %d reference audios", seedanceMaxRefAudios)
	}
	if counts[common.RefRoleReferenceAudio] > 0 &&
		counts[common.RefRoleReferenceImage] == 0 && counts[common.RefRoleReferenceVideo] == 0 {
		return refError("reference_audio requires at least one reference_image or reference_video")
	}
	return nil
}

// referenceContentItems references → Ark content 媒体项（保持原序）
func referenceContentItems(refs []common.VideoReference) []contentItem {
	items := make([]contentItem, 0, len(refs))
	for _, ref := range refs {
		item := contentItem{Role: ref.Role}
		switch common.ReferenceKind(ref.Role) {
		case common.RefKindImage:
			item.Type, item.ImageURL = "image_url", &mediaURL{URL: ref.URL}
		case common.RefKindVideo:
			item.Type, item.VideoURL = "video_url", &mediaURL{URL: ref.URL}
		case common.RefKindAudio:
			item.Type, item.AudioURL = "audio_url", &mediaURL{URL: ref.URL}
		default:
			continue
		}
		items = append(items, item)
	}
	return items
}
