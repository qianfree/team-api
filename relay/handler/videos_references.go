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
//   - 参考图（含首帧）唯一入口为 references[].role（首帧 = first_frame），
//     官方 input_reference 字段已移除（命中 400 并指引改用 references[]）。

// 协议层安全上限（防滥用，非模型约束；单类上限取 Seedance 2.0 与 MiniMax H3 的公共上限）
const (
	videosMaxReferences      = 20
	videosMaxReferenceImages = 9
	videosMaxReferenceVideos = 3
	videosMaxReferenceAudios = 3
	// file/link 上限对齐万相 wan3.0 官方规则：各最多 1 个且互斥（可与 reference_* 组合）
	videosMaxReferenceFiles = 1
	videosMaxReferenceLinks = 1
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

// validateVideosReferences 协议层校验（见设计文档 §5.2）。
// 首帧（role first_frame）与其他参考素材同在 references[] 中，无独立字段。
func validateVideosReferences(req *videosCreateRequest) *common.TaskError {
	if len(req.References) == 0 {
		return nil
	}
	if len(req.References) > videosMaxReferences {
		return referencesError(errCodeTooManyReferences, "too many references: at most %d", videosMaxReferences)
	}

	counts := map[string]int{}
	frames, references := 0, 0
	for i, ref := range req.References {
		label := fmt.Sprintf("references[%d]", i)
		kind := common.ReferenceKind(ref.Role)
		if kind == "" {
			return referencesError(errCodeInvalidReferenceRole,
				"%s.role must be one of first_frame, last_frame, reference_image, reference_video, reference_audio, file, link", label)
		}
		if (ref.URL == "") == (ref.FileID == "") {
			return referencesError(errCodeInvalidReferenceSrc, "%s must provide exactly one of url or file_id", label)
		}
		// link 是网页 URL 语义，不接受文件引用（file_id 解析侧另有兜底拦截）
		if ref.Role == common.RefRoleLink && ref.FileID != "" {
			return referencesError(errCodeInvalidReferenceSrc, "%s.role link does not accept file_id; use url", label)
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
	// 首尾帧（图生视频）与多模态参考互斥：Seedance 2.0 / MiniMax H3 / wan3.0 共同规则
	// （file/link 属参考侧，与首尾帧同样互斥）
	if frames > 0 && references > 0 {
		return referencesError(errCodeReferenceModeConflict,
			"first_frame/last_frame cannot be combined with reference_image/reference_video/reference_audio/file/link")
	}
	// file 与 link 二选一（wan3.0），均可与 reference_* 组合
	if counts[common.RefRoleFile] > 0 && counts[common.RefRoleLink] > 0 {
		return referencesError(errCodeReferenceModeConflict, "file and link cannot be combined; provide only one of them")
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
	if counts[common.RefRoleFile] > videosMaxReferenceFiles {
		return referencesError(errCodeTooManyReferences, "too many file: at most %d", videosMaxReferenceFiles)
	}
	if counts[common.RefRoleLink] > videosMaxReferenceLinks {
		return referencesError(errCodeTooManyReferences, "too many link: at most %d", videosMaxReferenceLinks)
	}
	return nil
}

// validateReferenceURL url 形态校验：http(s)；data URL 主类型须与 role 类别一致；
// 厂商素材 URI（asset:// 方舟素材库、mm_file:// MiniMax 文件、oss:// 百炼临时 URL）
// 原样透传，由插件按渠道校验。file/link 为非媒体参考（wan3.0）：仅收 http(s) URL
// （file 另收 oss:// 临时 URL），不支持 base64 与厂商素材 URI
func validateReferenceURL(label, kind, raw string) *common.TaskError {
	lower := strings.ToLower(raw)
	httpOK := strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
	switch kind {
	case common.RefKindLink:
		if httpOK {
			return nil
		}
		return referencesError(errCodeInvalidReferenceSrc, "%s.url must be an http(s) URL", label)
	case common.RefKindFile:
		if httpOK || strings.HasPrefix(lower, "oss://") {
			return nil
		}
		return referencesError(errCodeInvalidReferenceSrc, "%s.url must be an http(s) or oss:// URL", label)
	}
	switch {
	case httpOK, strings.HasPrefix(lower, "asset://"), strings.HasPrefix(lower, "mm_file://"),
		strings.HasPrefix(lower, "oss://"):
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
		switch kind {
		case common.RefKindLink:
			// link 是网页 URL 语义，文件引用无从谈起
			return "", referencesError(errCodeInvalidReferenceSrc, "%s.role link does not accept file_id; use url", label)
		case common.RefKindFile:
			// 文档类内容类型多样（application/pdf、text/markdown 等），不做主类型前缀限制
			return file.URL, nil
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(file.ContentType)), kind+"/") {
			return "", referencesError(errCodeReferenceFileMismatch,
				"%s.file_id %q has content type %q, expected %s/*", label, id, file.ContentType, kind)
		}
		return file.URL, nil
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
