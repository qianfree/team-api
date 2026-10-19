package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

// 本文件解决官方 SDK 的 multipart 编码兼容：openai-go / openai-python 对
// /v1/images/edits（image[]/mask/prompt 表单）和 /v1/audio/transcriptions
// （file/model 表单）一律发 multipart/form-data，而校验阶段提取模型名等标量
// 字段时只按 JSON 反序列化，multipart 体以 --boundary 开头，报
// "invalid character '-' in numeric literal"。文件 part 的内容属于上游载荷，
// 网关原样透传（见 openai adaptor 对 ImagesEdits/Audio 的 Content-Type 透传），
// 这里只提取校验/计费侧消费的标量字段。

// multipartFieldMaxBytes 单个标量字段的读取上限：model/stream 等 consumed 字段
// 都远小于此值，防御性挡掉异常超长字段（不被消费的 multipart 字段直接跳过）。
const multipartFieldMaxBytes = 64 << 10

// consumedMultipartFields multipart 形态下要收集的字段白名单，与
// validateRelayRequest / extractSessionSignals 从 rawRequest 消费的键一一对应。
// 白名单外的字段（prompt、size、image[] 字符串形态等）不进 map——它们要么属于
// 上游载荷，要么校验阶段不读，收集进来只会白占内存。
var consumedMultipartFields = map[string]bool{
	"model":                 true,
	"stream":                true,
	"max_tokens":            true,
	"max_completion_tokens": true,
	"max_output_tokens":     true,
	"n":                     true, // 图片协议张数（images/edits multipart 形态），计费放大用
	"metadata":              true,
	"previous_response_id":  true,
	"conversation_id":       true,
}

// isMultipartContentType 判断 Content-Type 是否为 multipart 表单。
func isMultipartContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.HasPrefix(mediaType, "multipart/")
}

// estimateMultipartInputTokens 估算 multipart 请求体的输入 token（仅用于预扣）：
// 标量字段按文本 len/4，文件 part（STT 音频、图片编辑的 image[]/mask 等二进制
// 载荷）按固定成本计入，不按字节数折算——10MB 音频按 len/4 会被估成约 260 万
// token，预扣直接冻穿余额甚至误报余额不足。音频时长/图片分辨率在网关侧无法
// 低价获得，固定成本只是闸门，结算仍以上游 usage 为准。
func estimateMultipartInputTokens(body []byte, contentType string) int {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil || params["boundary"] == "" {
		return inlineMediaTokenCost
	}

	textBytes, files := 0, 0
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, err := mr.NextPart()
		if err != nil {
			break // io.EOF 或畸形体：按已扫描到的部分估算
		}
		if part.FileName() != "" {
			files++
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(part, multipartFieldMaxBytes+1))
		if err != nil {
			break
		}
		textBytes += len(raw)
	}

	estimated := textBytes/4 + files*inlineMediaTokenCost
	if estimated == 0 {
		estimated = 1
	}
	return estimated
}

// parseRequestBodyFields 提取请求体中标量字段的原始 JSON 值，供校验阶段读取
// 模型名、stream、max_tokens 等。JSON 请求（含未声明 Content-Type）直接反序列化；
// multipart/form-data 只提取非文件 part 的白名单标量值，包成 JSON 字符串字面量
// （下游按 string 解析的 model 等字段不受影响；数值/布尔字段在 multipart 形态下
// 保持字符串形态，读取处本就容错忽略）。
func parseRequestBodyFields(body []byte, headers http.Header) (map[string]json.RawMessage, error) {
	var contentType string
	if headers != nil {
		contentType = headers.Get("Content-Type")
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		var rawRequest map[string]json.RawMessage
		if uErr := json.Unmarshal(body, &rawRequest); uErr != nil {
			return nil, uErr
		}
		return rawRequest, nil
	}
	return parseMultipartFields(body, params["boundary"])
}

// parseMultipartFields 扫描 multipart 体收集白名单标量字段。
// 用 NextPart 逐段推进而非 ReadForm：ReadForm 会把大文件 part（MB 级 image[]/mask）
// 溢写到临时文件，而这里只需要几个小字段；NextPart 换段时自动丢弃未读的当前段，
// 文件 part 零拷贝零落盘。
func parseMultipartFields(body []byte, boundary string) (map[string]json.RawMessage, error) {
	if boundary == "" {
		return nil, fmt.Errorf("missing multipart boundary")
	}

	fields := make(map[string]json.RawMessage)
	mr := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid multipart form: %w", err)
		}

		// 文件 part（image[]/mask/file 等）与白名单外的字段：不读内容直接换段
		if part.FileName() != "" || !consumedMultipartFields[part.FormName()] {
			continue
		}

		raw, err := io.ReadAll(io.LimitReader(part, multipartFieldMaxBytes+1))
		if err != nil {
			return nil, fmt.Errorf("read multipart field %q: %w", part.FormName(), err)
		}
		if len(raw) > multipartFieldMaxBytes {
			return nil, fmt.Errorf("multipart field %q exceeds %d bytes", part.FormName(), multipartFieldMaxBytes)
		}
		// 标量值包成 JSON 字符串字面量，与 JSON 形态的 map 取值方式统一
		fields[part.FormName()] = json.RawMessage(strconv.Quote(string(raw)))
	}
	return fields, nil
}
