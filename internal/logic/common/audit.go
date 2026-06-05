package common

import (
	"bufio"
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/qianfree/team-api/internal/dao"
)

// Audit levels
const (
	AuditLevelFull         = "full"
	AuditLevelFullText     = "full_text"
	AuditLevelMasked       = "masked"
	AuditLevelQuestionOnly = "question_only"
	AuditLevelNone         = "none"
)

// GetAuditLevel 从 sys_options 获取审计级别，默认 full
func GetAuditLevel(ctx context.Context) string {
	val := Config().GetString(ctx, "audit_level")
	if val == "" {
		return AuditLevelFull
	}
	return val
}

// GetAuditConfig 获取全局审计级别（兼容别名）
func GetAuditConfig(ctx context.Context) (string, error) {
	return GetAuditLevel(ctx), nil
}

// GetAuditLevels 返回全局审计级别和租户审计级别，两者完全独立。
// 全局级别从 sys_options 读取，默认 full。
// 租户级别从 tnt_tenants.settings JSONB 读取，未设置时默认 masked。
func GetAuditLevels(ctx context.Context, tenantID int64) (globalLevel, tenantLevel string) {
	globalLevel = GetAuditLevel(ctx)
	tenantLevel = AuditLevelMasked // 租户默认脱敏记录
	if tenantID > 0 {
		if tl := GetTenantAuditLevel(ctx, tenantID); tl != "" {
			tenantLevel = tl
		}
	}
	return
}

// GetTenantAuditLevel 从 tnt_tenants.settings JSONB 中读取租户自身设置的审计级别。
// 未设置时返回空字符串，不回退到全局级别。
func GetTenantAuditLevel(ctx context.Context, tenantID int64) string {
	var settingsJSON string
	err := dao.TntTenants.Ctx(ctx).
		Where("id", tenantID).
		Fields("settings").
		Scan(&settingsJSON)
	if err != nil || settingsJSON == "" {
		return ""
	}
	var settings struct {
		AuditLevel string `json:"audit_level"`
	}
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return ""
	}
	return settings.AuditLevel
}

// ApplyAuditLevel 根据审计级别处理请求体和响应体，返回处理后的内容。
func ApplyAuditLevel(level string, requestBody, responseBody string, isStream bool, path string) (req, resp string) {
	req = requestBody
	resp = responseBody

	switch level {
	case AuditLevelNone:
		req = ""
		resp = ""
	case AuditLevelQuestionOnly:
		resp = ""
	case AuditLevelMasked:
		req = MaskSensitiveData(req)
		// 先提取纯文本（thinking + content），再对文本整体脱敏
		if isStream {
			resp = ExtractStreamText(resp, path)
		} else {
			resp = ExtractStreamTextNonStream(resp, path)
		}
		resp = MaskSensitiveData(resp)
	case AuditLevelFullText:
		if isStream {
			resp = ExtractStreamText(resp, path)
		} else {
			resp = ExtractStreamTextNonStream(resp, path)
		}
	case AuditLevelFull:
		// 完整记录，不做处理
	}
	return
}

// MaskSensitiveData 对字符串中的敏感信息进行脱敏处理：
//   - 身份证号（18位）：显示前3后4，中间脱敏
//   - 手机号（11位）：显示前3后4，中间脱敏
//   - 邮箱地址：脱敏本地部分
//   - 银行卡号（16-19位）：显示前4后4，中间脱敏
//   - Bearer Token：完全脱敏
//   - Cookie 值：完全脱敏
//   - IPv4 地址：脱敏最后一段
func MaskSensitiveData(data string) string {
	if data == "" {
		return data
	}

	result := data

	// 身份证号
	idCardRegex := regexp.MustCompile(`\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`)
	result = idCardRegex.ReplaceAllStringFunc(result, func(s string) string {
		if len(s) == 18 {
			return s[:3] + strings.Repeat("*", 11) + s[14:]
		}
		return s
	})

	// 手机号
	phoneRegex := regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	result = phoneRegex.ReplaceAllStringFunc(result, func(s string) string {
		if len(s) == 11 {
			return s[:3] + "****" + s[7:]
		}
		return s
	})

	// 邮箱
	emailRegex := regexp.MustCompile(`\b([a-zA-Z0-9._%+\-])([a-zA-Z0-9._%+\-]*)@([a-zA-Z0-9.\-]+\.[a-zA-Z]{2,})\b`)
	result = emailRegex.ReplaceAllString(result, "$1****@$3")

	// 银行卡号
	bankRegex := regexp.MustCompile(`\b[3-6]\d{15,18}\b`)
	result = bankRegex.ReplaceAllStringFunc(result, func(s string) string {
		if len(s) >= 16 {
			return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
		}
		return s
	})

	// Bearer Token
	tokenRegex := regexp.MustCompile(`(?i)(bearer\s+|token[=:"\s]+|access_token[=:"\s]+|refresh_token[=:"\s]+)([a-zA-Z0-9_\-.]+)`)
	result = tokenRegex.ReplaceAllString(result, "$1****")

	// Cookie
	cookieRegex := regexp.MustCompile(`(?i)(cookie[=:"\s]+)([a-zA-Z0-9_\-=]+)`)
	result = cookieRegex.ReplaceAllString(result, "$1****")

	// IPv4
	ipRegex := regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3})\.(\d{1,3})\b`)
	result = ipRegex.ReplaceAllString(result, "$1.*")

	return result
}

// ExtractStreamText 从 SSE 原始数据中提取纯文本（思考过程 + 回答内容）。
// 根据 path 判断协议格式：OpenAI 格式（/chat/completions 等）和 Claude 格式（/messages）。
// 返回 JSON 字符串：{"thinking":"...","content":"..."}，无对应内容时字段为空字符串。
func ExtractStreamText(sseData string, path string) string {
	var thinking, content strings.Builder

	scanner := bufio.NewScanner(strings.NewReader(sseData))
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

	var currentEvent string

	for scanner.Scan() {
		line := scanner.Text()

		// Claude SSE 使用 event: 行标记事件类型
		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		if strings.HasSuffix(path, "/messages") {
			extractClaudeChunk(payload, currentEvent, &thinking, &content)
		} else {
			extractOpenAIChunk(payload, &thinking, &content)
		}
	}

	result := map[string]string{
		"thinking": thinking.String(),
		"content":  content.String(),
	}
	b, _ := json.Marshal(result)
	return string(b)
}

// extractOpenAIChunk 从 OpenAI 格式 chunk 中提取文本
func extractOpenAIChunk(payload string, thinking, content *strings.Builder) {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content          *string `json:"content"`
				ReasoningContent *string `json:"reasoning_content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return
	}
	for _, choice := range chunk.Choices {
		if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
			thinking.WriteString(*choice.Delta.ReasoningContent)
		}
		if choice.Delta.Content != nil && *choice.Delta.Content != "" {
			content.WriteString(*choice.Delta.Content)
		}
	}
}

// extractClaudeChunk 从 Claude 格式 SSE 事件中提取文本
func extractClaudeChunk(payload string, _ string, thinking, content *strings.Builder) {
	var event struct {
		Type  string `json:"type"`
		Delta struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"delta"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return
	}
	// thinking 块：字段名是 "thinking" 而非 "text"
	if event.Delta.Type == "thinking_delta" && event.Delta.Thinking != "" {
		thinking.WriteString(event.Delta.Thinking)
	}
	// 文本块
	if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
		content.WriteString(event.Delta.Text)
	}
}

// ExtractStreamTextNonStream 对非流式响应提取文本（full_text 级别复用）
func ExtractStreamTextNonStream(respBody string, path string) string {
	if strings.HasSuffix(path, "/messages") {
		return extractClaudeNonStreamText(respBody)
	}
	return extractOpenAINonStreamText(respBody)
}

func extractOpenAINonStreamText(respBody string) string {
	var resp struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(respBody), &resp); err != nil {
		return respBody
	}
	result := map[string]string{}
	for _, c := range resp.Choices {
		if c.Message.ReasoningContent != "" {
			result["thinking"] = result["thinking"] + c.Message.ReasoningContent
		}
		if c.Message.Content != "" {
			result["content"] = result["content"] + c.Message.Content
		}
	}
	if len(result) == 0 {
		return respBody
	}
	b, _ := json.Marshal(result)
	return string(b)
}

func extractClaudeNonStreamText(respBody string) string {
	var resp struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(respBody), &resp); err != nil {
		return respBody
	}
	result := map[string]string{}
	for _, block := range resp.Content {
		// thinking 块：字段名是 "thinking" 而非 "text"
		if block.Type == "thinking" && block.Thinking != "" {
			result["thinking"] = result["thinking"] + block.Thinking
		}
		if block.Type == "text" && block.Text != "" {
			result["content"] = result["content"] + block.Text
		}
	}
	if len(result) == 0 {
		return respBody
	}
	b, _ := json.Marshal(result)
	return string(b)
}
