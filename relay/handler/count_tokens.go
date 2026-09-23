package handler

import (
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/qianfree/team-api/relay/dto"
)

// count_tokens 端点的本地估算参数。
//
// 说明：本平台不内置精确分词器（tiktoken 等），POST /v1/messages/count_tokens
// 采用「本地粗略估算」策略——不请求上游、不计费，仅供客户端做上下文窗口管理的近似参考，
// 返回值不保证与上游精确计数一致。估算按字符类别加权：CJK 表意文字 token 密度远高于拉丁字母。
const (
	latinTokenWeight = 0.25 // 拉丁字母/数字/空白/标点：约 4 字符/token
	cjkTokenWeight   = 0.6  // CJK 表意文字：约 1.67 字符/token

	claudeImageTokens     = 1600 // 单张图片/文档的粗略 token 估值（无法本地解码，按固定值计）
	claudeMessageOverhead = 3    // 每条消息的结构化开销
	claudeToolOverhead    = 8    // 每个工具定义的结构化开销
	claudeRequestOverhead = 8    // 请求整体固定开销（system 包装、消息分隔等）
)

// errCountTokensInvalid 表示 count_tokens 请求体校验失败，调用方统一映射为 400 invalid_request_error。
var errCountTokensInvalid = errors.New("invalid count_tokens request")

// ctRequest count_tokens 的宽松解析结构：content / system / input_schema 保留为 RawMessage，
// 便于区分「字符串」与「内容块数组」两种形态后分别估算。
type ctRequest struct {
	Model    string          `json:"model"`
	System   json.RawMessage `json:"system"`
	Messages []ctMessage     `json:"messages"`
	Tools    []ctTool        `json:"tools"`
}

type ctMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type ctTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// CountClaudeTokens 本地估算 Claude Messages 请求的输入 token 数。
// 返回 (input_tokens, error)；error 非 nil 时均为请求体校验错误。
func CountClaudeTokens(body []byte) (int, error) {
	var req ctRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return 0, errWithMessage("invalid request body")
	}
	if strings.TrimSpace(req.Model) == "" {
		return 0, errWithMessage("model is required")
	}
	if len(req.Messages) == 0 {
		return 0, errWithMessage("messages is required")
	}

	tokens := claudeRequestOverhead

	// system：string 或 []ClaudeContentBlock
	tokens += estimateContentTokens(req.System)

	// messages
	for _, msg := range req.Messages {
		tokens += estimateContentTokens(msg.Content)
		tokens += claudeMessageOverhead
	}

	// tools：名称 + 描述 + input_schema（JSON 全文计入）
	for _, tool := range req.Tools {
		tokens += estimateTextTokens(tool.Name)
		tokens += estimateTextTokens(tool.Description)
		tokens += estimateRawTokens(tool.InputSchema)
		tokens += claudeToolOverhead
	}

	return tokens, nil
}

// estimateContentTokens 估算 message.content / system 的 token 数，兼容字符串与内容块数组两种形态。
func estimateContentTokens(raw json.RawMessage) int {
	trimmed := bytesTrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return 0
	}

	// 形态一：纯字符串
	var s string
	if json.Unmarshal(trimmed, &s) == nil {
		return estimateTextTokens(s)
	}

	// 形态二：内容块数组
	var blocks []dto.ClaudeContentBlock
	if json.Unmarshal(trimmed, &blocks) == nil {
		total := 0
		for i := range blocks {
			total += estimateBlockTokens(&blocks[i])
		}
		return total
	}

	// 兜底：无法结构化解析时，按原始 JSON 文本估算，避免漏计
	return estimateRawTokens(trimmed)
}

// estimateBlockTokens 估算单个内容块的 token 数（text / thinking / tool_use / tool_result / image 等）。
func estimateBlockTokens(b *dto.ClaudeContentBlock) int {
	total := 0
	if b.Text != nil {
		total += estimateTextTokens(*b.Text)
	}
	if b.Thinking != nil {
		total += estimateTextTokens(*b.Thinking)
	}
	if b.Name != "" {
		total += estimateTextTokens(b.Name)
	}
	if b.Input != nil {
		total += estimateAnyTokens(b.Input) // tool_use 的入参
	}
	if b.Content != nil {
		total += estimateAnyContent(b.Content) // tool_result 的嵌套内容（string 或 []block）
	}
	if b.Source != nil {
		total += claudeImageTokens // 图片/文档：固定估值
	}
	return total
}

// estimateAnyContent 估算 tool_result.content：可能是字符串，也可能是内容块数组。
func estimateAnyContent(v any) int {
	if v == nil {
		return 0
	}
	if s, ok := v.(string); ok {
		return estimateTextTokens(s)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return estimateContentTokens(raw)
}

// estimateAnyTokens 将任意结构（tool_use 入参等）序列化为 JSON 后按文本估算。
func estimateAnyTokens(v any) int {
	if v == nil {
		return 0
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return estimateTextTokens(string(raw))
}

// estimateRawTokens 按原始 JSON 字节的文本估算（input_schema 等已是 JSON 的场景）。
func estimateRawTokens(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	return estimateTextTokens(string(raw))
}

// estimateTextTokens 按字符类别粗略估算文本 token 数：CJK 表意文字权重高，其余字符权重低。
func estimateTextTokens(text string) int {
	if text == "" {
		return 0
	}
	var weighted float64
	for _, r := range text {
		if isCJK(r) {
			weighted += cjkTokenWeight
		} else {
			weighted += latinTokenWeight
		}
	}
	return int(math.Ceil(weighted))
}

// isCJK 判断字符是否属于常见 CJK 表意文字区段（中日韩统一表意文字及扩展、假名、谚文等）。
func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF, // CJK 统一表意文字
		r >= 0x3400 && r <= 0x4DBF,   // CJK 扩展 A
		r >= 0x20000 && r <= 0x2A6DF, // CJK 扩展 B
		r >= 0xF900 && r <= 0xFAFF,   // CJK 兼容表意文字
		r >= 0x3040 && r <= 0x30FF,   // 平假名 + 片假名
		r >= 0xAC00 && r <= 0xD7AF,   // 谚文音节
		r >= 0x3000 && r <= 0x303F:   // CJK 符号和标点
		return true
	}
	return false
}

// bytesTrimSpace 去除 RawMessage 两端的 JSON 空白字符。
func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// errWithMessage 构造 count_tokens 校验错误，消息文本即返回给客户端的错误信息。
func errWithMessage(msg string) error {
	return &countTokensError{message: msg}
}

// countTokensError 携带面向客户端的错误信息，供 handler 以 Claude 原生格式回写。
type countTokensError struct {
	message string
}

func (e *countTokensError) Error() string { return e.message }
func (e *countTokensError) Is(target error) bool {
	return target == errCountTokensInvalid
}
