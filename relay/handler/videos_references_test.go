package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

func parseJSONVideos(t *testing.T, body string) (*videosCreateRequest, *common.TaskError) {
	t.Helper()
	req, terr := ParseVideosCreateRequest([]byte(body), "application/json")
	if terr != nil {
		return nil, terr
	}
	return req, ValidateVideosCreateRequest(req)
}

func TestVideosReferences_ParseJSONForms(t *testing.T) {
	// 数组形态
	req, terr := parseJSONVideos(t, `{"model":"m","prompt":"p","references":[
		{"role":"reference_image","url":"https://a.png"},
		{"role":"reference_video","file_id":"file-abcdef12"}]}`)
	if terr != nil {
		t.Fatalf("数组形态应通过: %+v", terr)
	}
	if len(req.References) != 2 || req.References[1].FileID != "file-abcdef12" {
		t.Fatalf("解析异常: %+v", req.References)
	}
	// JSON 字符串形态（与 multipart / SDK extra_body json.dumps 同口径）
	req, terr = parseJSONVideos(t, `{"model":"m","prompt":"p","references":"[{\"role\":\"reference_audio\",\"url\":\"https://a.mp3\"},{\"role\":\"reference_image\",\"url\":\"https://i\"}]"}`)
	if terr != nil || len(req.References) != 2 || req.References[0].Role != common.RefRoleReferenceAudio {
		t.Fatalf("字符串形态解析异常: %+v %+v", req, terr)
	}
}

func TestVideosReferences_ParseMultipart(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", "m")
	_ = w.WriteField("prompt", "p")
	// 官方 SDK 以对象形态传 input_reference 时的 brackets 编码
	_ = w.WriteField("input_reference[image_url]", "https://first.png")
	_ = w.WriteField("references", `[{"role":"last_frame","url":"https://last.png"}]`)
	_ = w.Close()

	req, terr := ParseVideosCreateRequest(buf.Bytes(), w.FormDataContentType())
	if terr != nil {
		t.Fatalf("multipart 解析失败: %+v", terr)
	}
	if req.InputReference != "https://first.png" {
		t.Fatalf("input_reference[image_url] 未解析: %+v", req)
	}
	if terr := ValidateVideosCreateRequest(req); terr != nil {
		t.Fatalf("首尾帧应通过: %+v", terr)
	}
	refs := req.allReferences()
	if len(refs) != 2 || refs[0].Role != common.RefRoleFirstFrame || refs[1].Role != common.RefRoleLastFrame {
		t.Fatalf("合并顺序异常: %+v", refs)
	}

	// input_reference[file_id]
	buf.Reset()
	w = multipart.NewWriter(&buf)
	_ = w.WriteField("model", "m")
	_ = w.WriteField("prompt", "p")
	_ = w.WriteField("input_reference[file_id]", "file-abcdef12")
	_ = w.Close()
	req, terr = ParseVideosCreateRequest(buf.Bytes(), w.FormDataContentType())
	if terr != nil || req.InputReferenceFileID != "file-abcdef12" {
		t.Fatalf("input_reference[file_id] 未解析: %+v %+v", req, terr)
	}
}

func TestVideosReferences_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		code string
	}{
		{"非数组", `{"model":"m","prompt":"p","references":{"role":"x"}}`, errCodeInvalidReferences},
		{"元素非对象", `{"model":"m","prompt":"p","references":["https://a"]}`, errCodeInvalidReferences},
		{"未知 role", `{"model":"m","prompt":"p","references":[{"role":"reference_document","url":"https://a.pdf"}]}`, errCodeInvalidReferenceRole},
		{"缺 role", `{"model":"m","prompt":"p","references":[{"url":"https://a"}]}`, errCodeInvalidReferenceRole},
		{"来源缺失", `{"model":"m","prompt":"p","references":[{"role":"reference_image"}]}`, errCodeInvalidReferenceSrc},
		{"来源重复", `{"model":"m","prompt":"p","references":[{"role":"reference_image","url":"https://a","file_id":"file-abcdef12"}]}`, errCodeInvalidReferenceSrc},
		{"非法 scheme", `{"model":"m","prompt":"p","references":[{"role":"reference_image","url":"ftp://a"}]}`, errCodeInvalidReferenceSrc},
		{"data URL 类别不符", `{"model":"m","prompt":"p","references":[{"role":"reference_audio","url":"data:image/png;base64,AAAA"}]}`, errCodeInvalidReferenceSrc},
		{"data URL 非 base64", `{"model":"m","prompt":"p","references":[{"role":"reference_image","url":"data:image/png,abc"}]}`, errCodeInvalidReferenceSrc},
		{"非法 file_id", `{"model":"m","prompt":"p","references":[{"role":"reference_image","file_id":"abc"}]}`, errCodeInvalidReferenceSrc},
		{"首帧与参考互斥", `{"model":"m","prompt":"p","references":[{"role":"first_frame","url":"https://a"},{"role":"reference_image","url":"https://b"}]}`, errCodeReferenceModeConflict},
		{"input_reference 与参考互斥", `{"model":"m","prompt":"p","input_reference":{"image_url":"https://a"},"references":[{"role":"reference_video","url":"https://v"}]}`, errCodeReferenceModeConflict},
		{"input_reference 与 first_frame 重复", `{"model":"m","prompt":"p","input_reference":{"image_url":"https://a"},"references":[{"role":"first_frame","url":"https://b"}]}`, errCodeReferenceModeConflict},
		{"两个尾帧", `{"model":"m","prompt":"p","references":[{"role":"last_frame","url":"https://a"},{"role":"last_frame","url":"https://b"}]}`, errCodeReferenceModeConflict},
		{"参考视频超 3", `{"model":"m","prompt":"p","references":[` + strings.Repeat(`{"role":"reference_video","url":"https://v"},`, 3) + `{"role":"reference_video","url":"https://v"}]}`, errCodeTooManyReferences},
		{"参考图超 9", `{"model":"m","prompt":"p","references":[` + strings.Repeat(`{"role":"reference_image","url":"https://i"},`, 9) + `{"role":"reference_image","url":"https://i"}]}`, errCodeTooManyReferences},
	}
	for _, c := range cases {
		_, terr := parseJSONVideos(t, c.body)
		if terr == nil || terr.StatusCode != 400 || terr.ErrCode != c.code {
			t.Errorf("%s: got %+v want code %s", c.name, terr, c.code)
		}
	}
	// 厂商素材 URI 原样放行
	if _, terr := parseJSONVideos(t, `{"model":"m","prompt":"p","references":[{"role":"reference_image","url":"asset://asset-123"},{"role":"reference_video","url":"mm_file://456"}]}`); terr != nil {
		t.Fatalf("厂商素材 URI 应放行: %+v", terr)
	}
}

func TestVideosReferences_ResolveFiles(t *testing.T) {
	req, terr := parseJSONVideos(t, `{"model":"m","prompt":"p","references":[
		{"role":"reference_image","url":"https://a.png"},
		{"role":"reference_video","file_id":"file-video0001"},
		{"role":"reference_audio","file_id":"file-audio0001"}]}`)
	if terr != nil {
		t.Fatalf("parse: %+v", terr)
	}
	var asked []string
	resolver := func(ids []string) (map[string]ResolvedReferenceFile, error) {
		asked = ids
		return map[string]ResolvedReferenceFile{
			"file-video0001": {URL: "https://oss/v.mp4?sig", ContentType: "video/mp4"},
			"file-audio0001": {URL: "https://oss/a.mp3?sig", ContentType: "audio/mpeg"},
		}, nil
	}
	if terr := ResolveVideosReferenceFiles(req, resolver); terr != nil {
		t.Fatalf("resolve: %+v", terr)
	}
	if len(asked) != 2 || req.References[1].URL != "https://oss/v.mp4?sig" || req.References[1].FileID != "" {
		t.Fatalf("解析结果异常: asked=%v refs=%+v", asked, req.References)
	}

	// 不存在 / 跨租户
	req, _ = parseJSONVideos(t, `{"model":"m","prompt":"p","references":[{"role":"reference_image","file_id":"file-missing01"}]}`)
	if terr := ResolveVideosReferenceFiles(req, func([]string) (map[string]ResolvedReferenceFile, error) {
		return map[string]ResolvedReferenceFile{}, nil
	}); terr == nil || terr.ErrCode != errCodeReferenceFileNotFound {
		t.Fatalf("应报 not found: %+v", terr)
	}
	// 类别不符：音频文件当参考图
	req, _ = parseJSONVideos(t, `{"model":"m","prompt":"p","input_reference":{"file_id":"file-audio0001"}}`)
	if terr := ResolveVideosReferenceFiles(req, resolver); terr == nil || terr.ErrCode != errCodeReferenceFileMismatch {
		t.Fatalf("应报 type mismatch: %+v", terr)
	}
	// 存储故障 → 500
	req, _ = parseJSONVideos(t, `{"model":"m","prompt":"p","references":[{"role":"reference_image","file_id":"file-image0001"}]}`)
	if terr := ResolveVideosReferenceFiles(req, func([]string) (map[string]ResolvedReferenceFile, error) {
		return nil, errors.New("storage down")
	}); terr == nil || terr.StatusCode != 500 {
		t.Fatalf("存储故障应 500: %+v", terr)
	}
	// 无 file_id 不调用 resolver
	req, _ = parseJSONVideos(t, `{"model":"m","prompt":"p","references":[{"role":"reference_image","url":"https://a"}]}`)
	if terr := ResolveVideosReferenceFiles(req, nil); terr != nil {
		t.Fatalf("无 file_id 不应报错: %+v", terr)
	}
}

func TestVideosReferences_TaskBody(t *testing.T) {
	// 多模态参考：references 保持原序；reference_* 不进兼容字段 images[]/metadata.image
	req, terr := parseJSONVideos(t, `{"model":"m","prompt":"p","references":[
		{"role":"reference_image","url":"https://i1"},
		{"role":"reference_video","url":"https://v1"},
		{"role":"reference_image","url":"https://i2"}]}`)
	if terr != nil {
		t.Fatalf("parse: %+v", terr)
	}
	raw, err := BuildVideosTaskBody(req)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	refs := common.ReferencesFromTaskBody(body)
	if len(refs) != 3 || refs[0].URL != "https://i1" || refs[1].Role != common.RefRoleReferenceVideo || refs[2].URL != "https://i2" {
		t.Fatalf("references 异常: %s", raw)
	}
	if _, ok := body["images"]; ok {
		t.Fatalf("参考图不得写入兼容字段 images[]: %s", raw)
	}

	// 首尾帧：input_reference 首帧在前，兼容字段只承载首/尾帧
	req, _ = parseJSONVideos(t, `{"model":"m","prompt":"p","input_reference":{"image_url":"https://first"},"references":[{"role":"last_frame","url":"https://last"}]}`)
	raw, _ = BuildVideosTaskBody(req)
	body = nil
	_ = json.Unmarshal(raw, &body)
	refs = common.ReferencesFromTaskBody(body)
	meta, _ := body["metadata"].(map[string]any)
	images, _ := body["images"].([]any)
	if len(refs) != 2 || refs[0].Role != common.RefRoleFirstFrame || len(images) != 1 || images[0] != "https://first" ||
		meta["image"] != "https://first" || meta["last_frame"] != "https://last" {
		t.Fatalf("首尾帧任务体异常: %s", raw)
	}
}
