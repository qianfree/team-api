package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/qianfree/team-api/relay/common"
)

// OpenAI Videos 协议扩展：参考素材 references[]（协议层解析与校验）。
// 设计见 docs/design/OpenAI-Videos协议扩展-参考素材设计方案-v1.md：
//   - 网关只做协议层校验（形状 / role / 来源唯一 / 模式互斥 / 安全上限 / file 归属与类别），
//     模型特有约束（数量、音频搭配、base64 可用性）由插件/适配器校验；
//   - 官方 input_reference 语义不变：单图 = first_frame，与 references 合并为一个有序列表。

// 协议层安全上限（防滥用，非模型约束；单类上限取 Seedance 2.0 与 MiniMax H3 的公共上限）
const (
	videosMaxReferences      = 20
	videosMaxReferenceImages = 9
	videosMaxReferenceVideos = 3
	videosMaxReferenceAudios = 3
)

// 参考素材错误码
const (
	errCodeInvalidReferences     = "invalid_references"
	errCodeInvalidReferenceRole  = "invalid_reference_role"
	errCodeInvalidReferenceSrc   = "invalid_reference_source"
	errCodeReferenceModeConflict = "reference_mode_conflict"
	errCodeReferenceFileNotFound = "reference_file_not_found"
	errCodeReferenceFileMismatch = "reference_file_type_mismatch"
	errCodeTooManyReferences     = "too_many_references"
)

// videosFileIDPattern /v1/files 文件 ID 形态（与 relaylogic.relayFileRefPattern 同口径）
var videosFileIDPattern = regexp.MustCompile(`^file-[A-Za-z0-9]{8,64}$`)

// ResolvedReferenceFile file_id 解析结果：签名 URL + 上传时记录的内容类型。
type ResolvedReferenceFile struct {
	URL         string
	ContentType string
}

// VideosFileResolver 批量解析 file_id（宿主注入：按租户查 /v1/files 记录并签发 URL）。
// 不存在 / 跨租户的 ID 不出现在返回 map 中；error 表示存储或查询故障。
type VideosFileResolver func(fileIDs []string) (map[string]ResolvedReferenceFile, error)

func referencesError(code, format string, args ...any) *common.TaskError {
	return &common.TaskError{StatusCode: http.StatusBadRequest, Message: fmt.Sprintf(format, args...), ErrCode: code}
}

// parseVideosReferences 解析 references：JSON 数组，或内容为 JSON 数组的字符串
// （multipart 恒为字符串；官方 SDK 经 extra_body 传入时须 json.dumps）。
func parseVideosReferences(raw json.RawMessage) ([]common.VideoReference, *common.TaskError) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, `"`) {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, referencesError(errCodeInvalidReferences, "invalid references: %v", err)
		}
		trimmed = strings.TrimSpace(s)
		if trimmed == "" {
			return nil, nil
		}
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil, referencesError(errCodeInvalidReferences, "invalid references: expected an array of {role, url|file_id}")
	}
	out := make([]common.VideoReference, 0, len(items))
	for i, item := range items {
		var ref struct {
			Role   *string `json:"role"`
			URL    *string `json:"url"`
			FileID *string `json:"file_id"`
		}
		if err := json.Unmarshal(item, &ref); err != nil || !strings.HasPrefix(strings.TrimSpace(string(item)), "{") {
			return nil, referencesError(errCodeInvalidReferences, "invalid references[%d]: expected an object {role, url|file_id}", i)
		}
		out = append(out, common.VideoReference{
			Role:   strings.TrimSpace(deref(ref.Role)),
			URL:    strings.TrimSpace(deref(ref.URL)),
			FileID: strings.TrimSpace(deref(ref.FileID)),
		})
	}
	return out, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// allReferences 合并后的有序参考素材：input_reference 派生的首帧在前，其后按 references 原序。
func (r *videosCreateRequest) allReferences() []common.VideoReference {
	out := make([]common.VideoReference, 0, len(r.References)+1)
	if r.InputReference != "" || r.InputReferenceFileID != "" {
		out = append(out, common.VideoReference{
			Role: common.RefRoleFirstFrame, URL: r.InputReference, FileID: r.InputReferenceFileID,
		})
	}
	return append(out, r.References...)
}

// validateVideosReferences 协议层校验（见设计文档 §5.2）。
func validateVideosReferences(req *videosCreateRequest) *common.TaskError {
	if req.InputReference == "" && req.InputReferenceFileID == "" && len(req.References) == 0 {
		return nil
	}
	if len(req.References) > videosMaxReferences {
		return referencesError(errCodeTooManyReferences, "too many references: at most %d", videosMaxReferences)
	}
	// input_reference 是官方首帧语义，与 references 中显式的 first_frame 二者只能取一
	if req.InputReference != "" || req.InputReferenceFileID != "" {
		for _, ref := range req.References {
			if ref.Role == common.RefRoleFirstFrame {
				return referencesError(errCodeReferenceModeConflict,
					"input_reference is the first frame; do not also pass a first_frame in references")
			}
		}
	}

	counts := map[string]int{}
	frames, references := 0, 0
	for i, ref := range req.allReferences() {
		label := referenceLabel(req, i)
		kind := common.ReferenceKind(ref.Role)
		if kind == "" {
			return referencesError(errCodeInvalidReferenceRole,
				"%s.role must be one of first_frame, last_frame, reference_image, reference_video, reference_audio", label)
		}
		if (ref.URL == "") == (ref.FileID == "") {
			return referencesError(errCodeInvalidReferenceSrc, "%s must provide exactly one of url or file_id", label)
		}
		if ref.FileID != "" && !videosFileIDPattern.MatchString(ref.FileID) {
			return referencesError(errCodeInvalidReferenceSrc, "%s.file_id is not a valid file id", label)
		}
		if ref.URL != "" {
			if terr := validateReferenceURL(label, kind, ref.URL); terr != nil {
				return terr
			}
		}
		counts[ref.Role]++
		if common.IsFrameRole(ref.Role) {
			frames++
		} else {
			references++
		}
	}

	if counts[common.RefRoleFirstFrame] > 1 || counts[common.RefRoleLastFrame] > 1 {
		return referencesError(errCodeReferenceModeConflict, "at most one first_frame and one last_frame are allowed")
	}
	// 首尾帧（图生视频）与多模态参考互斥：Seedance 2.0 与 MiniMax H3 共同规则
	if frames > 0 && references > 0 {
		return referencesError(errCodeReferenceModeConflict,
			"first_frame/last_frame (incl. input_reference) cannot be combined with reference_image/reference_video/reference_audio")
	}
	if counts[common.RefRoleReferenceImage] > videosMaxReferenceImages {
		return referencesError(errCodeTooManyReferences, "too many reference_image: at most %d", videosMaxReferenceImages)
	}
	if counts[common.RefRoleReferenceVideo] > videosMaxReferenceVideos {
		return referencesError(errCodeTooManyReferences, "too many reference_video: at most %d", videosMaxReferenceVideos)
	}
	if counts[common.RefRoleReferenceAudio] > videosMaxReferenceAudios {
		return referencesError(errCodeTooManyReferences, "too many reference_audio: at most %d", videosMaxReferenceAudios)
	}
	return nil
}

// referenceLabel 错误信息中的素材定位（input_reference 或 references[i]）。
func referenceLabel(req *videosCreateRequest, mergedIndex int) string {
	if req.InputReference != "" || req.InputReferenceFileID != "" {
		if mergedIndex == 0 {
			return "input_reference"
		}
		mergedIndex--
	}
	return fmt.Sprintf("references[%d]", mergedIndex)
}

// validateReferenceURL url 形态校验：http(s)；data URL 主类型须与 role 类别一致；
// 厂商素材 URI（asset:// 方舟素材库、mm_file:// MiniMax 文件）原样透传，由插件按渠道校验。
func validateReferenceURL(label, kind, raw string) *common.TaskError {
	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"),
		strings.HasPrefix(lower, "asset://"), strings.HasPrefix(lower, "mm_file://"):
		return nil
	case strings.HasPrefix(lower, "data:"):
		header, _, ok := strings.Cut(raw[len("data:"):], ",")
		if !ok || !strings.Contains(strings.ToLower(header), ";base64") {
			return referencesError(errCodeInvalidReferenceSrc, "%s.url must be a base64 data URL", label)
		}
		mimeType := strings.ToLower(strings.TrimSpace(strings.SplitN(header, ";", 2)[0]))
		if !strings.HasPrefix(mimeType, kind+"/") {
			return referencesError(errCodeInvalidReferenceSrc,
				"%s.url has content type %q, expected %s/*", label, mimeType, kind)
		}
		return nil
	default:
		return referencesError(errCodeInvalidReferenceSrc,
			"%s.url must be an http(s) URL, a base64 data URL, or a vendor asset URI", label)
	}
}

// ResolveVideosReferenceFiles 把全部 file_id 解析为签名 URL（就地改写），并校验文件类别与
// role 一致。须在 ValidateVideosCreateRequest 之后调用；无 file_id 时不调用 resolver。
func ResolveVideosReferenceFiles(req *videosCreateRequest, resolve VideosFileResolver) *common.TaskError {
	ids := make([]string, 0, 4)
	seen := map[string]struct{}{}
	collect := func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	collect(req.InputReferenceFileID)
	for _, ref := range req.References {
		collect(ref.FileID)
	}
	if len(ids) == 0 {
		return nil
	}
	if resolve == nil {
		return referencesError(errCodeInvalidReferenceSrc, "file_id references are not available on this endpoint")
	}
	files, err := resolve(ids)
	if err != nil {
		return &common.TaskError{StatusCode: http.StatusInternalServerError, Message: "failed to resolve file references"}
	}

	apply := func(label, role, id string) (string, *common.TaskError) {
		file, ok := files[id]
		if !ok || file.URL == "" {
			return "", referencesError(errCodeReferenceFileNotFound, "%s.file_id %q was not found", label, id)
		}
		kind := common.ReferenceKind(role)
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(file.ContentType)), kind+"/") {
			return "", referencesError(errCodeReferenceFileMismatch,
				"%s.file_id %q has content type %q, expected %s/*", label, id, file.ContentType, kind)
		}
		return file.URL, nil
	}

	if req.InputReferenceFileID != "" {
		url, terr := apply("input_reference", common.RefRoleFirstFrame, req.InputReferenceFileID)
		if terr != nil {
			return terr
		}
		req.InputReference, req.InputReferenceFileID = url, ""
	}
	for i := range req.References {
		if req.References[i].FileID == "" {
			continue
		}
		url, terr := apply(fmt.Sprintf("references[%d]", i), req.References[i].Role, req.References[i].FileID)
		if terr != nil {
			return terr
		}
		req.References[i].URL, req.References[i].FileID = url, ""
	}
	return nil
}
