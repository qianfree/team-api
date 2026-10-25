package xai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// xAI 参考图编辑（/v1/images/edits）请求归一化。
//
// 上游约束：只收 application/json，参考图放 image_url（单值 string、多值 string[]，
// 上限 4 张），取值为公网 URL 或 base64 data URI（不支持 OpenAI SDK 的 multipart 表单
// 与 Files API file_id）。网关入站是 OpenAI 形态——JSON（image 字段）或 multipart
// 表单（官方 SDK images.edit() 的 image/image[] part）——这里统一转为 xAI JSON：
//   - multipart 文件 part → 读字节编 base64 data URI（纯内存转换，不涉 Files API）
//   - 字符串参考图 → URL/data URI 原样，裸 base64 按 MIME 魔数嗅探包装
//   - multipart 标量字段数字化（n/seed），OpenAI 专有参数（size/style 等）剥离
//   - mask 显式 400（静默丢弃会让用户误以为蒙版生效）
//   - 模型映射一并在转换内完成

const (
	// xaiMaxEditImages xAI 多参考图上限
	xaiMaxEditImages = 4
	// xaiMaxImageBytes 单张参考图原始字节上限（base64 前的解码字节）
	xaiMaxImageBytes = 10 << 20
	// xaiMaxScalarFieldBytes multipart 标量字段读取上限，与 handler 层防线同量级
	xaiMaxScalarFieldBytes = 64 << 10
)

// xaiOpenAIOnlyFields OpenAI images/edits 专有、xAI 不接受的参数，转换时剥离。
// mask 不在此列——显式 400 而非静默剥离。
var xaiOpenAIOnlyFields = map[string]bool{
	"size":           true,
	"style":          true,
	"user":           true,
	"background":     true,
	"input_fidelity": true,
}

// xaiMultipartIntFields multipart 标量字段中的数值字段：表单里是字符串（"2"），
// xAI JSON 要求数字。
var xaiMultipartIntFields = map[string]bool{
	"n":    true,
	"seed": true,
}

// convertImagesEditsRequest 把入站 images/edits 请求（JSON 或 multipart）转为 xAI JSON。
func convertImagesEditsRequest(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var contentType string
	if info.RequestHeaders != nil {
		contentType = info.RequestHeaders.Get("Content-Type")
	}
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if strings.HasPrefix(mediaType, "multipart/") {
		return convertMultipartEdits(requestBody, contentType, info)
	}
	return convertJSONEdits(requestBody, info)
}

// convertJSONEdits 转换 JSON 形态：image_url 已存在（xAI 原生客户端）时只做
// 模型映射与 OpenAI 专有参数剥离，不动参考图取值；否则把 image 归一化为 image_url。
func convertJSONEdits(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &raw); err != nil {
		return nil, constant.NewRequestError("invalid JSON body for images/edits: "+err.Error(), nil)
	}

	if _, hasMask := raw["mask"]; hasMask {
		return nil, constant.NewRequestError("xAI images/edits does not support mask", nil)
	}
	for f := range xaiOpenAIOnlyFields {
		delete(raw, f)
	}

	if _, native := raw["image_url"]; !native {
		imgRaw, hasImage := raw["image"]
		if !hasImage {
			return nil, constant.NewRequestError("images/edits requires a reference image in `image` or `image_url` (url or base64)", nil)
		}
		delete(raw, "image")
		normalized, err := normalizeImageValues(imgRaw)
		if err != nil {
			return nil, err
		}
		raw["image_url"] = normalized
	}

	if info.ChannelMeta.IsModelMapped {
		raw["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	}

	out, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal xai edits request failed: %w", err)
	}
	return bytes.NewReader(out), nil
}

// convertMultipartEdits 转换 multipart 表单形态（官方 SDK images.edit()）：
// image/image[] part（文件或字符串）归一化汇入 image_url，标量字段按类型写入。
func convertMultipartEdits(body []byte, contentType string, info *common.RelayInfo) (io.Reader, error) {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") || params["boundary"] == "" {
		return nil, constant.NewRequestError("invalid multipart Content-Type for images/edits", nil)
	}

	out := make(map[string]any)
	var images []string
	hasImageURLField := false

	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, perr := mr.NextPart()
		if perr == io.EOF {
			break
		}
		if perr != nil {
			return nil, constant.NewRequestError("invalid multipart form: "+perr.Error(), nil)
		}

		name := part.FormName()
		switch {
		case name == "mask":
			return nil, constant.NewRequestError("xAI images/edits does not support mask", nil)
		case name == "image" || name == "image[]":
			dataURI, isFile, rerr := readImagePart(part)
			if rerr != nil {
				return nil, rerr
			}
			if !isFile {
				normalized, nerr := normalizeRefImage(dataURI)
				if nerr != nil {
					return nil, nerr
				}
				dataURI = normalized
			}
			images = append(images, dataURI)
		case name == "image_url":
			// xAI 原生字段以字符串表单传入：归一化后仍写回 image_url
			val, rerr := readScalarPart(part)
			if rerr != nil {
				return nil, rerr
			}
			normalized, nerr := normalizeRefImage(val)
			if nerr != nil {
				return nil, nerr
			}
			out["image_url"] = normalized
			hasImageURLField = true
		case part.FileName() != "":
			// 其他文件 part（SDK 附带的杂项载荷）：忽略
		default:
			val, rerr := readScalarPart(part)
			if rerr != nil {
				return nil, rerr
			}
			applyMultipartScalarField(out, name, val)
		}
	}

	if !hasImageURLField {
		if len(images) == 0 {
			return nil, constant.NewRequestError("images/edits requires a reference image in `image`/`image[]` (file, url or base64)", nil)
		}
		if len(images) > xaiMaxEditImages {
			return nil, constant.NewRequestError(fmt.Sprintf("xAI images/edits accepts at most %d reference images, got %d", xaiMaxEditImages, len(images)), nil)
		}
		if len(images) == 1 {
			out["image_url"] = images[0]
		} else {
			out["image_url"] = images
		}
	}

	if info.ChannelMeta.IsModelMapped {
		out["model"] = info.ChannelMeta.UpstreamModelName
	}

	payload, merr := json.Marshal(out)
	if merr != nil {
		return nil, fmt.Errorf("marshal xai edits request failed: %w", merr)
	}
	return bytes.NewReader(payload), nil
}

// normalizeImageValues 把 JSON 形态的 image 取值（string 或 string[]）归一化为
// image_url 的 JSON 值，单值出 string、数组出数组（保持入参形态）。
func normalizeImageValues(imgRaw json.RawMessage) (json.RawMessage, error) {
	var single string
	if err := json.Unmarshal(imgRaw, &single); err == nil {
		v, nerr := normalizeRefImage(single)
		if nerr != nil {
			return nil, nerr
		}
		return json.Marshal(v)
	}

	var arr []string
	if err := json.Unmarshal(imgRaw, &arr); err == nil {
		if len(arr) == 0 {
			return nil, constant.NewRequestError("`image` must not be an empty array", nil)
		}
		if len(arr) > xaiMaxEditImages {
			return nil, constant.NewRequestError(fmt.Sprintf("xAI images/edits accepts at most %d reference images, got %d", xaiMaxEditImages, len(arr)), nil)
		}
		out := make([]string, 0, len(arr))
		for _, s := range arr {
			v, nerr := normalizeRefImage(s)
			if nerr != nil {
				return nil, nerr
			}
			out = append(out, v)
		}
		return json.Marshal(out)
	}

	return nil, constant.NewRequestError("`image` must be a string or array of strings (url or base64)", nil)
}

// normalizeRefImage 把单张参考图取值归一化：http(s) URL 与 data URI 原样，
// 裸 base64 按 MIME 魔数嗅探包装成 data URI。
func normalizeRefImage(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", constant.NewRequestError("reference image value must not be empty", nil)
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "data:") {
		return v, nil
	}
	return "data:" + sniffImageMIMEFromBase64(v) + ";base64," + v, nil
}

// readImagePart 读取 image/image[] part：文件 part 读原始字节（上限 xaiMaxImageBytes）
// 编成 data URI 并返回 isFile=true；字符串 part 返回原文（URL/裸 base64，由调用方
// 归一化）。
func readImagePart(part *multipart.Part) (string, bool, error) {
	if part.FileName() == "" {
		raw, err := io.ReadAll(io.LimitReader(part, xaiMaxScalarFieldBytes+1))
		if err != nil {
			return "", false, constant.NewRequestError("read image field failed: "+err.Error(), nil)
		}
		if len(raw) > xaiMaxScalarFieldBytes {
			return "", false, constant.NewRequestError(fmt.Sprintf("image field exceeds %d bytes", xaiMaxScalarFieldBytes), nil)
		}
		return string(raw), false, nil
	}

	raw, err := io.ReadAll(io.LimitReader(part, xaiMaxImageBytes+1))
	if err != nil {
		return "", false, constant.NewRequestError("read image part failed: "+err.Error(), nil)
	}
	if len(raw) > xaiMaxImageBytes {
		return "", false, constant.NewRequestError(fmt.Sprintf("reference image exceeds %d MB limit", xaiMaxImageBytes>>20), nil)
	}
	if len(raw) == 0 {
		return "", false, constant.NewRequestError("reference image part is empty", nil)
	}
	mimeType := imagePartMIME(part.Header.Get("Content-Type"), raw)
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(raw), true, nil
}

// readScalarPart 读取标量字段 part（带读取上限防线）。
func readScalarPart(part *multipart.Part) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(part, xaiMaxScalarFieldBytes+1))
	if err != nil {
		return "", constant.NewRequestError("read multipart field failed: "+err.Error(), nil)
	}
	if len(raw) > xaiMaxScalarFieldBytes {
		return "", constant.NewRequestError(fmt.Sprintf("multipart field %q exceeds %d bytes", part.FormName(), xaiMaxScalarFieldBytes), nil)
	}
	return string(raw), nil
}

// applyMultipartScalarField 把表单标量字段按类型写入出参 map：
// OpenAI 专有字段剥离，数值字段数字化，deferred 布尔化，其余原样字符串。
func applyMultipartScalarField(out map[string]any, name, value string) {
	if xaiOpenAIOnlyFields[name] {
		return
	}
	if xaiMultipartIntFields[name] {
		if n, err := strconv.Atoi(value); err == nil {
			out[name] = n
		}
		return
	}
	if name == "deferred" {
		if b, err := strconv.ParseBool(value); err == nil {
			out[name] = b
		}
		return
	}
	out[name] = value
}

// imagePartMIME 确定文件 part 的 MIME：优先取 part 声明的 image/* 类型，
// 否则按字节魔数嗅探，兜底 png。
func imagePartMIME(declared string, raw []byte) string {
	if declared != "" {
		if mt, _, err := mime.ParseMediaType(declared); err == nil && strings.HasPrefix(mt, "image/") {
			return mt
		}
	}
	return sniffImageMIMEFromBytes(raw)
}

// sniffImageMIMEFromBase64 按裸 base64 前缀嗅探 MIME（魔数字节的 base64 编码前缀）。
func sniffImageMIMEFromBase64(s string) string {
	switch {
	case strings.HasPrefix(s, "/9j/"):
		return "image/jpeg"
	case strings.HasPrefix(s, "iVBOR"):
		return "image/png"
	case strings.HasPrefix(s, "R0lGOD"):
		return "image/gif"
	case strings.HasPrefix(s, "UklGR"):
		return "image/webp"
	default:
		return "image/png"
	}
}

// sniffImageMIMEFromBytes 按字节魔数嗅探 MIME。
func sniffImageMIMEFromBytes(b []byte) string {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg"
	case len(b) >= 4 && b[0] == 0x89 && b[1] == 0x50 && b[2] == 0x4E && b[3] == 0x47:
		return "image/png"
	case len(b) >= 4 && b[0] == 0x47 && b[1] == 0x49 && b[2] == 0x46 && b[3] == 0x38:
		return "image/gif"
	case len(b) >= 12 && b[0] == 0x52 && b[1] == 0x49 && b[2] == 0x46 && b[3] == 0x46 && b[8] == 0x57 && b[9] == 0x45 && b[10] == 0x42 && b[11] == 0x50:
		return "image/webp"
	default:
		return "image/png"
	}
}
