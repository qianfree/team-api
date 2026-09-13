package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
// {model, prompt, seconds, metadata{duration, aspect_ratio, resolution, image}}，
// 由各 taskchannel 适配器按需消费（kling 读顶层 seconds，volcengine 读 metadata.duration 等）。

// videosInputReferenceMaxBytes 参考图上传大小上限。
// GoFrame server 默认 ClientMaxBodySize 8MB，base64 膨胀 4/3 后须留余量，取 6MB。
const videosInputReferenceMaxBytes = 6 << 20

// videosCreateRequest OpenAI Videos 创建请求的归一形态（multipart/JSON 解析后的产物）
type videosCreateRequest struct {
	Model          string
	Prompt         string
	Seconds        string // 统一归一为字符串（官方协议为 "4"/"8"/"12"）；空 = 未指定
	Size           string // 原样保留（如 "1280x720"）；空 = 未指定
	InputReference string // 参考图：data URL 或 http(s) URL；空 = 无
}

// videosRequestEcho 提交时落 PrivateData.request_echo 的请求回显，
// retrieve（GET /v1/videos/{id}）时用于回显 prompt/seconds/size（上游响应不含这些字段）。
type videosRequestEcho struct {
	Prompt  string `json:"prompt"`
	Seconds string `json:"seconds,omitempty"`
	Size    string `json:"size,omitempty"`
}

// jsonVideosCreateRequest JSON 编码的创建请求结构。
// input_reference 保留 RawMessage 以区分 {file_id} / {image_url} 两种引用形态。
type jsonVideosCreateRequest struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt"`
	Seconds        json.RawMessage `json:"seconds"` // string 或 number
	Size           string          `json:"size"`
	InputReference json.RawMessage `json:"input_reference"`
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
	var req jsonVideosCreateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid request body: " + err.Error()}
	}

	out := &videosCreateRequest{
		Model:  strings.TrimSpace(req.Model),
		Prompt: req.Prompt,
		Size:   strings.TrimSpace(req.Size),
	}

	seconds, terr := normalizeVideosSeconds(req.Seconds)
	if terr != nil {
		return nil, terr
	}
	out.Seconds = seconds

	ref, terr := parseVideosInputReferenceJSON(req.InputReference)
	if terr != nil {
		return nil, terr
	}
	out.InputReference = ref

	return out, nil
}

// parseVideosCreateMultipart 解析 multipart/form-data 编码的创建请求（官方 SDK 形态）。
// input_reference 可能是文件 part（上传），也可能是字符串 part（引用对象 JSON）。
func parseVideosCreateMultipart(body []byte, boundary string) (*videosCreateRequest, *common.TaskError) {
	if boundary == "" {
		return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "missing multipart boundary"}
	}

	form, err := multipart.NewReader(bytes.NewReader(body), boundary).ReadForm(videosInputReferenceMaxBytes)
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

	out := &videosCreateRequest{
		Model:  strings.TrimSpace(formValue("model")),
		Prompt: formValue("prompt"),
		Size:   strings.TrimSpace(formValue("size")),
	}

	seconds, terr := normalizeVideosSeconds(json.RawMessage(formValue("seconds")))
	if terr != nil {
		return nil, terr
	}
	out.Seconds = seconds

	// input_reference：优先文件 part，其次字符串 part（引用对象 JSON）
	if files := form.File["input_reference"]; len(files) > 0 {
		fh := files[0]
		f, err := fh.Open()
		if err != nil {
			return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "failed to open input_reference file"}
		}
		defer f.Close()

		raw, err := io.ReadAll(io.LimitReader(f, videosInputReferenceMaxBytes+1))
		if err != nil {
			return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "failed to read input_reference file"}
		}
		if len(raw) > videosInputReferenceMaxBytes {
			return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "input_reference file too large, max 6MB"}
		}
		if len(raw) == 0 {
			return nil, &common.TaskError{StatusCode: http.StatusBadRequest, Message: "input_reference file is empty"}
		}

		// 内容类型探测优先于 part 声明的 Content-Type（客户端常误标 application/octet-stream）
		ct := fh.Header.Get("Content-Type")
		if ct == "" || ct == "application/octet-stream" {
			ct = http.DetectContentType(raw)
		}
		out.InputReference = "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(raw)
	} else if v := formValue("input_reference"); v != "" {
		ref, terr := parseVideosInputReferenceJSON(json.RawMessage(v))
		if terr != nil {
			return nil, terr
		}
		out.InputReference = ref
	}

	return out, nil
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

// parseVideosInputReferenceJSON 解析引用对象形态的 input_reference：
//   - {"image_url": "https://... 或 data:..."} → 原样透传
//   - {"file_id": "..."} → 不支持（平台无 Files API），返回 400 引导改用 image_url/文件上传
func parseVideosInputReferenceJSON(raw json.RawMessage) (string, *common.TaskError) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}

	var ref struct {
		FileID   string `json:"file_id"`
		ImageURL string `json:"image_url"`
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		return "", &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid input_reference: expected file upload or {image_url}"}
	}
	switch {
	case ref.ImageURL != "":
		return ref.ImageURL, nil
	case ref.FileID != "":
		return "", &common.TaskError{
			StatusCode: http.StatusBadRequest,
			Message:    "input_reference.file_id is not supported; upload the file directly or use image_url instead",
			ErrCode:    "file_id_not_supported",
		}
	default:
		return "", &common.TaskError{StatusCode: http.StatusBadRequest, Message: "invalid input_reference: expected file upload or {image_url}"}
	}
}

// BuildVideosTaskBody 把归一后的创建请求转成任务框架通用请求体（JSON 字节）。
// 顶层保留 seconds + images，metadata 写入 duration/size/image。
// size 只借 OpenAI 协议的字段形态、不做任何档位换算：调用方直接传目标供应商的
// 原生分辨率词汇（如 MiniMax 的 768P/2K），原值经 metadata.size 透传给适配器。
func BuildVideosTaskBody(req *videosCreateRequest) ([]byte, error) {
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
	if req.InputReference != "" {
		metadata["image"] = req.InputReference
		// volcengine 图生视频读根级 images 数组（kling 亦有该兜底）
		body["images"] = []string{req.InputReference}
	}
	if len(metadata) > 0 {
		body["metadata"] = metadata
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
	return nil
}

// NewVideosRequestEcho 从归一请求构造提交回显（胶水层经 TaskRelayContext.RequestEcho 传入，
// 提交时随 PrivateData 落库，retrieve 时回显 prompt/seconds/size）
func NewVideosRequestEcho(req *videosCreateRequest) videosRequestEcho {
	return videosRequestEcho{
		Prompt:  req.Prompt,
		Seconds: req.Seconds,
		Size:    req.Size,
	}
}
